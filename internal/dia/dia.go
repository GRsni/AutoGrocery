package dia

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/go-rod/stealth"
)

type Credentials struct {
	Dia struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"dia"`
}

type Cookie struct {
	Name   string  `json:"name"`
	Value  string  `json:"value"`
	Domain string  `json:"domain"`
	Path   string  `json:"path"`
	Secure bool    `json:"secure"`
	Expiry float64 `json:"expirationDate"`
}

const debugImagesPath string = "/images/debug/dia/"

func humanDelay() {
	time.Sleep(time.Duration(600+rand.Intn(600)) * time.Millisecond)
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

func LoadSessionFromCookies(cookiesPath string) ([]*proto.NetworkCookieParam, error) {
	data, err := os.ReadFile(cookiesPath)
	if err != nil {
		return nil, err
	}

	var rodCookies []*proto.NetworkCookieParam
	scanner := bufio.NewScanner(strings.NewReader(string(data)))

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments (lines starting with #)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Split by tab character (\t)
		fields := strings.Split(line, "\t")

		// Ensure we have enough fields (Domain, Secure, Path, HttpOnly, Expiry, Name, Value)
		if len(fields) < 7 {
			continue // Skip malformed lines
		}

		// Parse fields
		// Index: 0=Domain, 1=Secure, 2=Path, 3=HttpOnly, 4=Expiry, 5=Name, 6=Value

		domain := fields[0]
		path := fields[2]

		// Parse Secure flag (TRUE/FALSE string -> bool)
		secureStr := strings.ToUpper(strings.TrimSpace(fields[1]))
		secure := secureStr == "TRUE"

		// Note: Index 3 is HttpOnly. proto.NetworkCookieParam does not have an HttpOnly field,
		// so we skip it here unless your proto definition includes it.

		// Parse Expiry (Unix timestamp float -> float64)
		// We convert to float64 to preserve the milliseconds as in your data
		_, err := strconv.ParseFloat(fields[4], 64)
		if err != nil {
			continue // Skip lines with invalid timestamps
		}

		name := fields[5]
		value := fields[6]

		// Create and append the cookie param
		rodCookies = append(rodCookies, &proto.NetworkCookieParam{
			Name:   name,
			Value:  value,
			Domain: domain,
			Path:   path,
			Secure: secure,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return rodCookies, nil
}

func LoginToDia(credentialsPath string, cookiesPath string) (*rod.Page, func(), error) {
	// Launch browser (headless by default)
	l := launcher.New().
		Headless(false).
		UserDataDir(`C:\Users\yaste\AppData\Local\Google\Chrome\User Data`).
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-dev-shm-usage").
		Set("no-sandbox")

	launcher.NewBrowser().MustGet()

	u := l.MustLaunch()

	// Rod sets navigator.languages to [d.AcceptLanguage] for pages.
	d := devices.LaptopWithMDPIScreen.Landscape() // the default device
	d.AcceptLanguage = "es-ES"                    // set to your locale

	browser := rod.New().ControlURL(u).MustConnect().DefaultDevice(d)

	page := stealth.MustPage(browser)

	cleanup := func() {
		browser.MustClose()
		l.Cleanup()
		l.Kill()
	}
	credentials, credLoadErr := LoadCredentials(credentialsPath)
	if credLoadErr != nil {
		return nil, cleanup, credLoadErr
	}
	rodCookies, cookiesLoadErr := LoadSessionFromCookies(cookiesPath)
	if cookiesLoadErr != nil {
		return nil, cleanup, cookiesLoadErr
	}
	err := page.SetCookies(rodCookies)
	if err != nil {
		return nil, cleanup, err
	}
	page.MustNavigate("https://www.dia.es/my-account").MustWaitLoad().MustWaitIdle()

	log.Println("✓ Navigated to Dia homepage")

	//page.MustReload().MustWaitLoad().MustWaitIdle()
	humanDelay()

	// Reject all cookies
	cookiesBtn, err := page.Timeout(2 * time.Second).Element("#onetrust-reject-all-handler")
	if err == nil {
		cookiesBtn.MustClick()
	}

	// Check if we got redirected to login — means cookies are expired
	if strings.Contains(page.MustInfo().URL, "/login") {
		return nil, cleanup, fmt.Errorf("session expired — please re-export cookies from Firefox")
	}

	emailFieldFound, emailField, err := page.Has("[data-test-id='email_input']")
	if err != nil {
		log.Println("Cannot find email input field, skipping")
		return nil, cleanup, err
	}
	if emailFieldFound {
		emailField.MustClick()
		// Type character by character with small delays
		for _, char := range credentials.Dia.Username {
			page.Keyboard.MustType(input.Key(char))
			time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		}
		page.MustWaitStable()
		humanDelay()
		fmt.Println("✓ Email added")
	} else {
		fmt.Println("Email input field not found")
	}

	passwordFieldFound, passwordField, err := page.Has("[data-test-id='password input_input']")
	if err != nil {
		log.Println("Cannot find password input field, skipping")
		return nil, cleanup, err
	}
	if passwordFieldFound {
		passwordField.MustClick()
		// Type character by character with small delays
		for _, char := range credentials.Dia.Password {
			page.Keyboard.MustType(input.Key(char))
			time.Sleep(time.Duration(50+rand.Intn(100)) * time.Millisecond)
		}
		page.MustWaitStable()
		humanDelay()
		fmt.Println("✓ Password added")
	} else {
		fmt.Println("Password input field not found")
	}

	loginBtnFound, loginBtn, err := page.Has("[data-test-id='email_continue_button_enabled']")
	if err != nil {
		log.Println("Cannot find login button, skipping")
		return nil, cleanup, err
	}
	if loginBtnFound {
		loginBtn.Hover()
		time.Sleep(time.Duration(300+rand.Intn(300)) * time.Millisecond)
		loginBtn.MustClick()

		humanDelay()
		page.MustWaitIdle().MustScreenshot(debugImagesPath + "login.png")
		fmt.Println("✓ Login button clicked")
	} else {
		fmt.Println("Login button not found")
	}

	//time.Sleep(10 * time.Minute)
	return page, cleanup, nil
}

func GetTicketList(page *rod.Page, lastFound time.Time) map[string]time.Time {
	ticketListLink, err := page.Element(".global-info__orders-link-content__button")
	if err != nil {
		log.Printf("Error finding ticket list link: %v", err)
		return nil
	}
	ticketListLink.MustClick()
	humanDelay()
	page.MustWaitLoad().MustWaitIdle()

	page.MustScreenshot(debugImagesPath + "ticket_list.png")

	tickets, err := page.Elements(".tickets__ticket-container__card")
	if err != nil {
		log.Printf("Error finding ticket list items: %v", err)
		return nil
	}

	ticketDates := map[string]time.Time{}
	for _, ticket := range tickets {
		text, _ := ticket.Text()
		dateFromTicket, err := getDateFromTicket(text)
		if err != nil {
			return nil
		}
		fmt.Println(dateFromTicket)
		if dateFromTicket.After(lastFound) {
			ticketDates[text] = dateFromTicket
		}
	}

	return ticketDates
}

func getDateFromTicket(ticketString string) (content time.Time, err error) {
	textLines := strings.Split(ticketString, "\n")
	if len(textLines) < 1 || len(textLines[0]) == 0 {
		return time.Unix(0, 0), fmt.Errorf("ticket string has no lines")
	}
	layout := "2/1/2006"

	extractedDate, err := time.Parse(layout, textLines[1])
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return
	}
	return extractedDate, nil
}
