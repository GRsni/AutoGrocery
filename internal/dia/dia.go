package dia

import (
	"autoGrocery/internal"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"sort"
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

	if len(creds.Dia.Password) == 0 || len(creds.Dia.Username) == 0 {
		return nil, fmt.Errorf("failed to extract credentials from file")
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

	slog.Info("✓ Navigated to Dia homepage")

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
		slog.Debug("Cannot find email input field, skipping")
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
		slog.Debug("✓ Email added")
	} else {
		slog.Debug("Email input field not found")
	}

	passwordFieldFound, passwordField, err := page.Has("[data-test-id='password input_input']")
	if err != nil {
		slog.Debug("Cannot find password input field, skipping")
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
		slog.Debug("✓ Password added")
	} else {
		slog.Debug("Password input field not found")
	}

	loginBtnFound, loginBtn, err := page.Has("[data-test-id='email_continue_button_enabled']")
	if err != nil {
		slog.Debug("Cannot find login button, skipping")
		return nil, cleanup, err
	}
	if loginBtnFound {
		loginBtn.Hover()
		time.Sleep(time.Duration(300+rand.Intn(300)) * time.Millisecond)
		loginBtn.MustClick()

		humanDelay()
		page.MustWaitIdle()
		//page.MustScreenshot(debugImagesPath + "login.png")
		slog.Debug("✓ Login button clicked")
	} else {
		slog.Debug("Login button not found")
	}

	return page, cleanup, nil
}

func GetTicketList(page *rod.Page, lastFound time.Time) []internal.Ticket {
	ticketLisFound, ticketListLink, err := page.Has(".global-info__orders-link-content__button")
	if err != nil {
		slog.Debug("Error finding ticket list link", "ERROR", err)
		return nil
	}
	if !ticketLisFound {
		slog.Warn("ticket list link not found, cookies are outdated, PLEASE REFRESH")
		return nil
	}
	ticketListLink.MustClick()
	humanDelay()
	page.MustWaitLoad().MustWaitIdle()

	//page.MustScreenshot(debugImagesPath + "ticket_list.png")

	ticketElements, err := page.Elements(".tickets__ticket-container__card")
	if err != nil {
		slog.Debug("Error finding ticket list items", "ERROR", err)
		return nil
	}
	tickets := make([]internal.Ticket, 0)
	for _, ticket := range ticketElements {
		text, _ := ticket.Text()
		dateFromTicket, err := getDateFromTicket(text)
		if err != nil {
			return nil
		}
		if dateFromTicket.After(lastFound) {
			diaTicket := getTicketDetails(ticket, page, dateFromTicket)
			tickets = append(tickets, diaTicket)
			slog.Debug(diaTicket.TicketToStr(constants.DIA))
		}
	}

	sort.SliceStable(tickets, func(i, j int) bool {
		return tickets[i].Date.Before(tickets[j].Date)
	})

	return tickets
}

func getTicketDetails(ticketElement *rod.Element, page *rod.Page, date time.Time) internal.Ticket {
	ticketBtnFound, ticketBtn, err := ticketElement.Has("[data-test-id='button-action']")
	if err != nil {
		slog.Debug("Cannot find ticket button, skipping", "ERROR", err)
		return internal.Ticket{}
	}
	if ticketBtnFound {
		ticketBtn.Hover()
		humanDelay()
		ticketBtn.MustClick()

		humanDelay()
		ticketDebugImagePath := debugImagesPath + "ticket-" + date.Format("2-1-2006") + ".png"
		fmt.Println(ticketDebugImagePath)
		//page.MustScreenshot(ticketDebugImagePath)
		slog.Debug("✓ Ticket button clicked")
	} else {
		slog.Debug("Ticket button not found")
	}

	ticketId, err := page.MustElement(".ticket-detail-header__simplified-invoice").Text()
	if err != nil {
		slog.Debug("Cannot find ticket id, skipping")
		return internal.Ticket{}
	}
	ticketId = strings.Replace(ticketId, "Factura simplificada Nº ", "", 1)

	ticketTotalStr, err := page.MustElement("[data-test-id='ticket-summary-total-final-amount-number']").Text()
	ticketTotal := utils.ParsePrice(ticketTotalStr)

	items, err := getItemList(page)

	if !ticketIsValid(ticketTotal, items) {
		slog.Warn("Ticket price does not match up, discarding")
		return internal.Ticket{}
	}

	// Close ticket
	ticketClose, err := page.Element("[data-test-id='ticket-detail-modal-cross']")
	if err != nil {
		slog.Debug("Cannot find ticket button, skipping")
		return internal.Ticket{}
	}
	ticketClose.Hover()
	time.Sleep(time.Duration(300+rand.Intn(300)) * time.Millisecond)
	ticketClose.MustClick()

	humanDelay()

	return internal.Ticket{Id: ticketId, Total: ticketTotal, Items: items, Date: date}
}

func ticketIsValid(total float64, items []internal.Item) bool {
	itemsTotal := 0.0

	for _, item := range items {
		itemsTotal += item.Amount * item.Price
	}

	return utils.FloatsEqual(total, utils.ToFixed(itemsTotal, 2))
}

func getDateFromTicket(ticketString string) (content time.Time, err error) {
	textLines := strings.Split(ticketString, "\n")
	if len(textLines) <= 1 {
		return time.Unix(0, 0), fmt.Errorf("ticket string doesn't have enough lines")
	}
	extractedDate, err := time.Parse("02/01/2006", strings.TrimSpace(textLines[1]))
	if err != nil {
		slog.Debug("Error parsing date", "ERROR", err)
		return
	}
	return extractedDate, nil
}

func getItemList(page *rod.Page) ([]internal.Item, error) {
	rows, err := page.Elements("[data-test-id='ticket-products-product']")
	if err != nil {
		slog.Debug("Unable to find item row element", "ERROR", err)
		return nil, err
	}
	items := make([]internal.Item, 0)

	for i, row := range rows {
		itemName := getItemName(row)
		itemQty := getItemQty(row)
		pricePerUnit := getPricePerUnit(row, i)
		itemTotalFound := getItemTotal(row)
		totalCorrect := isTotalCorrect(itemQty, pricePerUnit, itemTotalFound)
		if !totalCorrect {
			slog.Debug("Item total not correct, discarding item")
			continue
		}
		itemDiscount := getDiscount(row)
		if itemDiscount < 0 {
			// Apply discount shared between units
			pricePerUnit = pricePerUnit - itemDiscount/itemQty
		}

		//fmt.Println(itemName, itemQty, pricePerUnit, totalCorrect, itemDiscount)
		items = append(items, internal.Item{Name: itemName, Amount: itemQty, Price: pricePerUnit})
	}

	return items, nil
}

func getItemName(element *rod.Element) string {
	itemName, err := element.MustElement("[data-test-id='ticket-products-product-name']").Text()
	if err != nil {
		slog.Debug("Unable to find item name element", "ERROR", err)
		return ""
	}
	return itemName
}

func getItemQty(element *rod.Element) float64 {
	qtyStr := ""
	itemQtyFound, itemQty, err := element.Has("[data-test-id='ticket-products-product-quantity']")
	if err != nil {
		slog.Debug("Unable to find item quantity element", "ERROR", err)
		return 0.0
	}
	if itemQtyFound {
		qtyStr, err = itemQty.Text()
		if err != nil {
			slog.Debug("Unable to find item quantity element", "ERROR", err)
			return 0.0
		}
	} else {
		slog.Debug("Item quantity element not found, searching for weight")
		weightFound, weightElement, err := element.Has("[data-test-id='ticket-products-product-weight']")
		if err != nil {
			slog.Debug("Unable to find item weight element", "ERROR", err)
			return 0.0
		}
		if weightFound {
			qtyStr, _ = weightElement.Text()
		}
	}
	parsedQty := utils.ParseQty(qtyStr)
	return parsedQty
}

func getPricePerUnit(element *rod.Element, index int) float64 {
	itemPricePerUnit, err := element.MustElement("[data-test-id='ticket-products-product-price-per-unit-" + strconv.Itoa(index) + "']").Text()
	if err != nil {
		slog.Debug("Unable to find item price per unit element", "ERROR", err)
	}
	parsedPrice := utils.ParsePrice(itemPricePerUnit)
	return parsedPrice
}

func getItemTotal(element *rod.Element) float64 {
	itemTotal, err := element.MustElement("[data-test-id='ticket-products-product-amount']").Text()
	if err != nil {
		slog.Debug("Unable to find item amount element", "ERROR", err)
	}
	parsedTotal := utils.ParsePrice(itemTotal)
	return parsedTotal
}

func getDiscount(element *rod.Element) float64 {
	hasDiscount, itemDiscount, err := element.Has("[data-test-id='ticket-products-product-promotion-amount']")
	if err != nil {
		slog.Debug("Unable to find item discount element", "ERROR", err)
		return 0.0
	}
	if !hasDiscount {
		slog.Debug("Item discount not found")
		return 0.0
	}
	parsedDiscountStr, err := itemDiscount.Text()
	if err != nil {
		slog.Debug("Unable to get discount from item discount", "ERROR", err)
		return 0.0
	}
	parsedDiscount := utils.ParsePrice(parsedDiscountStr)
	return parsedDiscount

}

func isTotalCorrect(qty float64, pricePer float64, totalFound float64) bool {
	return utils.FloatsEqual(utils.ToFixed(qty*pricePer, 2), totalFound)
}
