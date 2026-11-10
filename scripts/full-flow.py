#!/usr/bin/env python3
"""
Export Dia.es cookies using Playwright.

Reads the password from config/credentials/secrets.json,
logs in to dia.es, waits for manual completion if needed,
then exports cookies in Netscape format to config/credentials/cookies-www-dia-es.txt.
"""

import json
import sys
import time
from datetime import datetime
from pathlib import Path

from invisible_playwright import InvisiblePlaywright

SECRETS_FILE = Path(__file__).parent.parent / "config" / "credentials" / "secrets.json"
COOKIES_FILE = (
    Path(__file__).parent.parent / "config" / "credentials" / "cookies-www-dia-es.txt"
)
LOGIN_URL = "https://www.dia.es/login"
PROJECT_ROOT = Path(__file__).parent.parent
MAX_COOKIES_AGE = 55
HEADLESS = True


def load_password() -> str:
    """Load Dia password from secrets.json."""
    with open(SECRETS_FILE, "r") as f:
        secrets = json.load(f)
    password = secrets.get("dia", {}).get("password")
    if not password or password == "null":
        print("Error: Dia password not found in secrets.json", file=sys.stderr)
        sys.exit(1)
    return password


def parse_existing_cookies(filepath: Path) -> list[dict]:
    """Parse existing Netscape cookies file and return list of cookie dicts with expiry info."""
    if not filepath.exists():
        return []

    cookies = []
    with open(filepath, "r") as f:
        for line in f:
            line = line.strip()
            # Skip comments and blank lines
            if not line or line.startswith("#"):
                continue
            parts = line.split("\t")
            if len(parts) >= 7:
                try:
                    cookies.append(
                        {
                            "name": parts[5],
                            "value": parts[6],
                            "expiry": float(parts[4]),
                        }
                    )
                except (ValueError, IndexError):
                    continue
    return cookies


def cookies_are_fresh(filepath: Path, max_age_minutes: int = MAX_COOKIES_AGE) -> bool:
    """Check if existing cookies are still fresh (not older than max_age_minutes)."""
    existing = parse_existing_cookies(filepath)
    if not existing:
        return False

    now = int(time.time())
    # Check if ALL cookies are still valid and fresh
    for cookie in existing:
        expiry = cookie["expiry"]

        # Session cookies (expiry=0 || -1) are always considered fresh
        # They don't have a time-based expiry
        if expiry == 0 or expiry == -1:
            continue

        age_seconds = now - expiry
        age_minutes = age_seconds / 60

        # Cookie is expired
        if expiry < now:
            return False

        # Cookie is older than max_age_minutes
        if age_minutes >= max_age_minutes:
            return False

    return True


def export_netscape_cookies(cookies: list[dict], filepath: Path) -> None:
    """Export Playwright cookies to Netscape HTTP Cookie File format."""
    with open(filepath, "w") as f:
        f.write(f"# Netscape HTTP Cookie File at date ${datetime.today().strftime('%Y-%m-%d-%H-%M-%s')}\n")
        f.write("# https://curl.haxx.se/rfc/cookie_spec.html\n")
        f.write("# This is a generated file! Do not edit.\n\n")

        for cookie in cookies:
            # Determine domain: prepend dot for subdomain coverage
            domain = cookie.get("domain", ".dia.es")
            if not domain.startswith("."):
                domain = f".{domain}"

            # Convert expiry (seconds or milliseconds) to Unix timestamp
            expiry = cookie.get("expires", 0)
            if expiry > 1e12:  # milliseconds
                expiry = expiry / 1000

            # Map Playwright field names to Netscape format
            secure = "TRUE" if cookie.get("secure", False) else "FALSE"
            http_only = "TRUE" if cookie.get("httpOnly", False) else "FALSE"
            path = cookie.get("path", "/")
            name = cookie["name"]
            value = cookie["value"]

            f.write(
                f"{domain}\t{secure}\t{path}\t{http_only}\t{expiry}\t{name}\t{value}\n"
            )

    print(f"Cookies saved to: {filepath}")


def run_go_service(project_root: Path) -> None:
    """Build and run the Go autoGrocery service."""
    import subprocess

    print("\nRunning Go service...")
    # Try the compiled binary first (Docker), fall back to go run (local dev)
    binary_path = project_root / "bin" / "autogrocery"
    if binary_path.exists():
        cmd = [str(binary_path)]
    else:
        # Check if autogrocery binary is in PATH (Docker)
        result = subprocess.run(
            ["which", "autogrocery"], capture_output=True, text=True
        )
        if result.returncode == 0:
            cmd = ["autogrocery"]
        else:
            # Fall back to go run for local development
            main_go = project_root / "cmd" / "autogrocery" / "main.go"
            cmd = ["go", "run", str(main_go), "--stores", "DIA"]

    result = subprocess.run(
        cmd,
        cwd=str(project_root),
        capture_output=False,
    )
    if result.returncode != 0:
        print(f"Go service exited with code {result.returncode}", file=sys.stderr)
    else:
        print("Go service completed successfully.")


def main():
    password = load_password()

    # Check if existing cookies are still fresh
    if cookies_are_fresh(COOKIES_FILE, max_age_minutes=MAX_COOKIES_AGE):
        print(
            f"Cookies are fresh (less than {MAX_COOKIES_AGE} minutes old). Skipping browser login."
        )
        print("Running Go service directly...")
        run_go_service(PROJECT_ROOT)
        return

    print("Cookies are stale or missing. Refreshing...")
    print("Opening dia.es/login with invisible_playwright...")

    # Pass --no-sandbox for Docker/headless environments
    extra_args = ["--no-sandbox"]

    with InvisiblePlaywright(extra_args=extra_args, headless=HEADLESS) as browser:
        context = browser.new_context(
            locale="es-ES",
            timezone_id="Europe/Madrid",
            color_scheme="light",
            geolocation={"longitude": -3.7038, "latitude": 40.4168},  # Madrid
        )

        page = context.new_page()

        try:
            # Navigate to login page
            print(f"Navigating to {LOGIN_URL}...")
            page.goto(LOGIN_URL, wait_until="domcontentloaded", timeout=30000)

            # Wait for page to fully load
            page.wait_for_load_state("networkidle")
            time.sleep(2)

            # Load username from secrets.json
            with open(SECRETS_FILE, "r") as f:
                secrets = json.load(f)
            username = secrets["dia"]["username"]

            # Fill email field
            print("Looking for email field...")
            email_selector = "[data-test-id='email_input']"
            try:
                email_input = page.wait_for_selector(email_selector, timeout=10000)
                if email_input:
                    print("Email field found. Filling username...")
                    # Type character by character with human-like delays
                    for char in username:
                        email_input.type(char, delay=20)
                    time.sleep(1)
                    print("Waiting for page to stabilize...")
                    page.wait_for_load_state("networkidle")
                    time.sleep(3)  # Wait for Continue button to become enabled
                else:
                    print("Email field not found.")
            except Exception:
                print("Email field not found.")

            # Check if password field is already visible (some flows show it immediately)
            password_selector = "[data-test-id='password input_input']"
            password_visible = False
            try:
                password_input = page.wait_for_selector(password_selector, timeout=5000)
                if password_input:
                    print("Password field found. Filling password...")
                    for char in password:
                        password_input.type(char, delay=50)
                    time.sleep(1)
                    password_visible = True
                    print("Password filled. Waiting for page to stabilize...")
                    page.wait_for_load_state("networkidle")
                    time.sleep(1)

                    # Click the login button (same selector as Continue button)
                    print("Looking for login button...")
                    login_button_selector = (
                        "[data-test-id='email_continue_button_enabled']"
                    )
                    try:
                        login_btn = page.wait_for_selector(
                            login_button_selector, timeout=10000, state="visible"
                        )
                        if login_btn:
                            print("  Found login button. Clicking...")
                            login_btn.click(force=True)
                            print("Clicked login button. Waiting for page to load...")
                            time.sleep(2)
                        else:
                            print("Login button not found.")
                    except Exception as e:
                        print(f"Login button click failed: {e}")
                else:
                    print("Password field not found.")
            except Exception:
                print("Password field not visible yet, clicking Continue...")

            # If password wasn't filled, click Continue to proceed to password step
            if not password_visible:
                print("Looking for Continue button...")
                button_selector = "[data-test-id='email_continue_button_enabled']"
                try:
                    # Wait for Continue button to appear and be visible
                    print(f"  Waiting for button with selector: {button_selector}")
                    btn = page.wait_for_selector(
                        button_selector, timeout=10000, state="visible"
                    )
                    if btn:
                        print("  Found Continue button. Clicking...")
                        # Use force=True to bypass visibility checks and click directly
                        btn.click(force=True)
                        print("Clicked Continue button. Waiting for password field...")
                        time.sleep(2)
                        # Wait for password field to appear with generous timeout
                        try:
                            password_input = page.wait_for_selector(
                                password_selector, timeout=15000
                            )
                            if password_input:
                                print("Password field found. Filling password...")
                                for char in password:
                                    password_input.type(char, delay=50)
                                time.sleep(1)
                                print(
                                    "Password filled. Waiting for page to stabilize..."
                                )
                                page.wait_for_load_state("networkidle")
                                time.sleep(1)

                                # Click the login button (same selector as Continue button)
                                print("Looking for login button...")
                                login_button_selector = (
                                    "[data-test-id='email_continue_button_enabled']"
                                )
                                try:
                                    login_btn = page.wait_for_selector(
                                        login_button_selector,
                                        timeout=10000,
                                        state="visible",
                                    )
                                    if login_btn:
                                        print("  Found login button. Clicking...")
                                        login_btn.click(force=True)
                                        print(
                                            "Clicked login button. Waiting for page to load..."
                                        )
                                        time.sleep(2)
                                    else:
                                        print("Login button not found.")
                                except Exception as e:
                                    print(f"Login button click failed: {e}")
                            else:
                                print("Password field did not appear after Continue.")
                        except Exception:
                            print("Password field did not appear after Continue.")
                    else:
                        print("Continue button not found.")
                except Exception as e:
                    print(f"Continue button click failed: {e}")

            # Wait for page to stabilize after login
            print("Waiting for page to load after login...")
            try:
                page.wait_for_load_state("networkidle", timeout=15000)
            except Exception:
                print("Page may still be loading, waiting manually...")
                time.sleep(5)
            time.sleep(2)

            # If still on login page, prompt for manual completion
            if "login" in page.url.lower():
                print(
                    "Warning: Still on login page. You may need to complete manual steps (captcha, 2FA, etc.)."
                )
                print("Waiting for you to complete them... (press Ctrl+C to cancel)")
                print("Press Enter when login is complete...")
                input()

            # Navigate to /my-account to ensure full login
            print("Navigating to /my-account to verify login...")
            my_account_url = "https://www.dia.es/my-account"
            try:
                page.goto(my_account_url, wait_until="domcontentloaded", timeout=30000)
                page.wait_for_load_state("networkidle")
                time.sleep(2)
            except Exception as e:
                print(f"Warning: Failed to navigate to /my-account: {e}")
                print("Attempting to read cookies anyway...")

            # Only read cookies once we're on /my-account (or attempted to navigate there)
            print("Extracting cookies...")
            cookies = context.cookies()
            print(f"Found {len(cookies)} cookies")

            if not cookies:
                print(
                    "Warning: No cookies found. Login may have failed.", file=sys.stderr
                )
                sys.exit(1)

            # Export cookies
            export_netscape_cookies(cookies, COOKIES_FILE)
            print("\nDone! Cookies exported successfully.")

            page.close()

            # Run the Go service to export all tickets
            run_go_service(PROJECT_ROOT)

        except Exception as e:
            print(f"Error: {e}", file=sys.stderr)
            sys.exit(1)
        finally:
            browser.close()


if __name__ == "__main__":
    main()
