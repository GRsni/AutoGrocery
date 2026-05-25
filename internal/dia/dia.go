package dia

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
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

func LoginToDia(cookiesPath string) (*rod.Page, func(), error) {
	// Launch browser (headless by default)
	l := launcher.New().Headless(false)
	l.Set("disable-blink-features", "AutomationControlled").
		Set("excludeSwitches", "enable-automation"). // removes "Chrome is being controlled" banner
		Set("useAutomationExtension", "false")

	u := l.MustLaunch()

	// Rod sets navigator.languages to [d.AcceptLanguage] for pages.
	d := devices.LaptopWithMDPIScreen.Landscape() // the default device
	d.AcceptLanguage = "es-ES"                    // set to your locale

	browser := rod.New().ControlURL(u).MustConnect().DefaultDevice(d)

	page := browser.MustPage()

	cleanup := func() {
		browser.MustClose()
		l.Cleanup()
		l.Kill()
	}

	rodCookies, _ := LoadSessionFromCookies(cookiesPath)
	err := page.SetCookies(rodCookies)
	if err != nil {
		return nil, cleanup, err
	}
	page.MustNavigate("https://www.dia.es/my-account").MustWaitLoad().MustWaitIdle()
	time.Sleep(5 * time.Second)

	log.Println("✓ Navigated to Dia homepage")

	page.MustReload().MustWaitLoad().MustWaitIdle()
	time.Sleep(5 * time.Second)

	// Reject all cookies
	btn, err := page.Element("#onetrust-accept-btn-handler")
	if err == nil {
		btn.MustClick()
	}

	//time.Sleep(10 * time.Minute)
	return page, cleanup, nil
}

func GetTicketList(page *rod.Page) []string {
	ticketListLink, err := page.Element(".global-info__orders-link-content__button")
	if err != nil {
		log.Printf("Error finding ticket list link: %v", err)
		return []string{}
	}
	ticketListLink.MustClick()
	page.MustWaitLoad().MustWaitIdle()

	page.MustScreenshot(debugImagesPath + "ticket_list.png")

	tickets, err := page.Elements(".tickets__ticket-container__card")
	if err != nil {
		log.Printf("Error finding ticket list items: %v", err)
		return []string{}
	}

	for _, ticket := range tickets {
		text, _ := ticket.Text()
		dateFromTicket, err := getDateFromTicket(text)
		if err != nil {
			return nil
		}
		fmt.Println(dateFromTicket)
	}

	time.Sleep(10 * time.Minute)
	return []string{}
}

func getDateFromTicket(ticketString string) (content time.Time, err error) {
	textLines := strings.Split(ticketString, "\n")
	if len(textLines) < 1 {
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
