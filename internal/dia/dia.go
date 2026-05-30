package dia

import (
	"autoGrocery/utils"
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

type Item struct {
	name   string
	amount float64
	price  float64
}

type Ticket struct {
	items []Item
	id    string
	total float64
}

func PrintDiaTicket(DiaTicket Ticket) {
	fmt.Println("DiaTicket:", DiaTicket.id, "total:", DiaTicket.total, "items:"+fmt.Sprint(DiaTicket.items))
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

	if len(creds.Dia.Password) == 0 || len(creds.Dia.Username) ==0 {
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

	return page, cleanup, nil
}

func GetTicketList(page *rod.Page, lastFound time.Time) map[string]time.Time {
	ticketLisFound, ticketListLink, err := page.Has(".global-info__orders-link-content__button")
	if err != nil {
		log.Printf("Error finding ticket list link: %v", err)
		return nil
	}
	if !ticketLisFound {
		log.Printf("ticket list link not found, cookies are outdated, PLEASE REFRESH")
		return nil
	}
	ticketListLink.MustClick()
	humanDelay()
	page.MustWaitLoad().MustWaitIdle()

	page.MustScreenshot(debugImagesPath + "ticket_list.png")

	ticketElements, err := page.Elements(".tickets__ticket-container__card")
	if err != nil {
		log.Printf("Error finding ticket list items: %v", err)
		return nil
	}
	tickets := make([]Ticket, 0)
	ticketDates := map[string]time.Time{}
	for _, ticket := range ticketElements {
		text, _ := ticket.Text()
		dateFromTicket, err := getDateFromTicket(text)
		if err != nil {
			return nil
		}
		//fmt.Println(dateFromTicket)
		if dateFromTicket.After(lastFound) {
			ticketDates[text] = dateFromTicket
			diaTicket := getTicketDetails(ticket, page, dateFromTicket)
			tickets = append(tickets, diaTicket)
			PrintDiaTicket(diaTicket)
		}
	}

	return ticketDates
}

func getTicketDetails(ticketElement *rod.Element, page *rod.Page, date time.Time) Ticket {
	ticketBtnFound, ticketBtn, err := ticketElement.Has("[data-test-id='button-action']")
	if err != nil {
		log.Println("Cannot find ticket button, skipping")
		return Ticket{}
	}
	if ticketBtnFound {
		ticketBtn.Hover()
		time.Sleep(time.Duration(300+rand.Intn(300)) * time.Millisecond)
		ticketBtn.MustClick()

		humanDelay()
		ticketDebugImagePath := debugImagesPath + "ticket-" + date.Format("2-1-2006") + ".png"
		fmt.Println(ticketDebugImagePath)
		page.MustScreenshot(ticketDebugImagePath)
		fmt.Println("✓ Ticket button clicked")
	} else {
		fmt.Println("Ticket button not found")
	}

	ticketId, err := page.MustElement(".ticket-detail-header__simplified-invoice").Text()
	if err != nil {
		log.Println("Cannot find ticket id, skipping")
		return Ticket{}
	}
	ticketId = strings.Replace(ticketId, "Factura simplificada Nº ", "", 1)

	ticketTotalStr, err := page.MustElement("[data-test-id='ticket-summary-total-final-amount-number']").Text()
	ticketTotal := utils.ParsePrice(ticketTotalStr)

	items, err := getItemList(page)

	if !ticketIsValid(ticketTotal, items){
		log.Println("Ticket price does not match up, discarding")
		return Ticket{}
	}

	return Ticket{id: ticketId, total: ticketTotal, items: items}
}

func ticketIsValid(total float64, items []Item) bool {
	itemsTotal := 0.0

	for _, item := range items {
		itemsTotal += item.amount * item.price
	}

	return utils.FloatsEqual(total, utils.ToFixed(itemsTotal, 2))
}

func getDateFromTicket(ticketString string) (content time.Time, err error) {
	textLines := strings.Split(ticketString, "\n")
	if len(textLines) <= 1 {
		return time.Unix(0, 0), fmt.Errorf("ticket string doesn't have enough lines")
	}
	layout := "02/01/2006"

	extractedDate, err := time.Parse(layout, strings.TrimSpace(textLines[1]))
	if err != nil {
		fmt.Println("Error parsing date:", err)
		return
	}
	return extractedDate, nil
}

func getItemList(page *rod.Page) ([]Item, error) {
	rows, err := page.Elements("[data-test-id='ticket-products-product']")
	if err != nil {
		log.Println("Unable to find item row element")
		return nil, err
	}
	items := make([]Item, 0)

	for i, row := range rows {
		itemName := getItemName(row)
		itemQty := getItemQty(row)
		pricePerUnit := getPricePerUnit(row, i)
		itemTotalFound := getItemTotal(row)
		totalCorrect := isTotalCorrect(itemQty, pricePerUnit, itemTotalFound)
		if !totalCorrect {
			log.Println("Item total not correct, discarding item")
			continue
		}
		itemDiscount := getDiscount(row)
		if itemDiscount < 0 {
			// Apply discount shared between units
			pricePerUnit = pricePerUnit - itemDiscount/itemQty
		}

		fmt.Println(itemName, itemQty, pricePerUnit, totalCorrect, itemDiscount)
		items = append(items, Item{name: itemName, amount: itemQty, price: pricePerUnit})
	}

	return items, nil
}

func getItemName(element *rod.Element) string {
	itemName, err := element.MustElement("[data-test-id='ticket-products-product-name']").Text()
	if err != nil {
		log.Println("Unable to find item name element")
		return ""
	}
	return itemName
}

func getItemQty(element *rod.Element) float64 {
	qtyStr := ""
	itemQtyFound, itemQty, err := element.Has("[data-test-id='ticket-products-product-quantity']")
	if err != nil {
		log.Println("Unable to find item quantity element")
	}
	if itemQtyFound {
		qtyStr, err = itemQty.Text()
		if err != nil {
			log.Println("Unable to find item quantity element")
			return 0.0
		}
	} else {
		log.Println("Item quantity element not found, searching for weight")
		weightFound, weightElement, err := element.Has("[data-test-id='ticket-products-product-weight']")
		if err != nil {
			log.Println("Unable to find item weight element")
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
		log.Println("Unable to find item price per unit element")
	}
	parsedPrice := utils.ParsePrice(itemPricePerUnit)
	return parsedPrice
}

func getItemTotal(element *rod.Element) float64 {
	itemTotal, err := element.MustElement("[data-test-id='ticket-products-product-amount']").Text()
	if err != nil {
		log.Println("Unable to find item amount element")
	}
	parsedTotal:= utils.ParsePrice(itemTotal)
	return parsedTotal
}

func getDiscount(element *rod.Element) float64 {
	hasDiscount, itemDiscount, err := element.Has("[data-test-id='ticket-products-product-promotion-amount']")
	if err != nil {
		log.Println("Unable to find item discount element")
	}
	if !hasDiscount {
		log.Println("Item discount not found")
		return 0.0
	}
	parsedDiscountStr, err := itemDiscount.Text()
	if err != nil {
		log.Println("Unable to get discount from item discount")
		return 0.0
	}
	parsedDiscount:= utils.ParsePrice(parsedDiscountStr)
	return parsedDiscount

}

func isTotalCorrect(qty float64, pricePer float64, totalFound float64) bool {
	return utils.FloatsEqual(utils.ToFixed(qty*pricePer, 2), totalFound)
}
