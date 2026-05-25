package carrefour

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	rod "github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
)

type Credentials struct {
	Carrefour struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"carrefour"`
}

func LoadCredentials(filePath string) (*Credentials, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open credentials file: %w", err)
	}
	defer file.Close()

	var creds Credentials
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&creds); err != nil {
		return nil, fmt.Errorf("failed to decode credentials: %w", err)
	}

	return &creds, nil
}

func LoginToCarrefour(creds *Credentials) {
	// Launch browser (headless by default)
	l := launcher.New().Headless(false)
	l.Set("disable-blink-features", "AutomationControlled")
	u := l.MustLaunch()
	defer l.Cleanup()
	defer l.Kill()

	// Rod sets navigator.languages to [d.AcceptLanguage] for pages.
	d := devices.LaptopWithMDPIScreen.Landscape() // the default device
	d.AcceptLanguage = "es-ES"                    // set to your locale

	browser := rod.New().ControlURL(u).MustConnect().DefaultDevice(d)
	defer browser.Close()

	page := browser.MustPage("https://www.carrefour.es/").MustWaitStable()

	log.Println("✓ Navigated to Carrefour homepage")

	found, el, err := page.Has(".account-menu")
	if err != nil {
		log.Println("Cannot find element:", "Account menu button, skipping")
		return
	}
	if found {
		el.MustClick()
		page.MustWaitStable().MustScreenshot("images/debug/carrefour/menu.png")
		fmt.Println("✓ Account menu clicked")
	} else {
		fmt.Println("Account menu not found")
	}

	//page.MustElement(".account-modal__login").MustClick().MustWaitStable()
	found, el, err = page.Has(".account-modal__login")
	if err != nil {
		log.Println("Cannot find element:", "Account menu button, skipping")
		return
	}
	if found {
		el.MustClick()
		page.MustWaitStable().MustScreenshot("images/debug/carrefour/login.png")
		fmt.Println("✓ Login menu clicked")
	} else {
		fmt.Println("Login button not found")
	}

	fmt.Println("✓ Login page open")

	page.Timeout(7 * time.Second)

	page.Mouse.MustMoveTo(1000, 600)
	page.Mouse.MustClick("left")

	fmt.Println("✓ Cookies modal dismissed")

	// Get the iframe and switch to its page context
	frame := page.MustElement("iframe[src*='/access']")
	loginModal := frame.MustFrame()
	loginModal.MustWaitLoad()

	// Now query elements inside the iframe
	loginModal.MustElement("#gigya-loginID-133272631659353340").MustInput("test")
	loginModal.MustElement("#gigya-password-66067780736329300").MustInput("bar")
}
