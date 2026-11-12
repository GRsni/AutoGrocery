package dia

import (
	"autoGrocery/internal"
	"autoGrocery/internal/google/sh"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
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

// Selector constants for Dia web scraping.
// These map to the DOM structure of dia.es and should be updated if the site changes.
const (
	SelectorCookiesReject          = "#onetrust-reject-all-handler"
	SelectorEmailInput             = "[data-test-id='email_input']"
	SelectorPasswordInput          = "[data-test-id='password input_input']"
	SelectorLoginButton            = "[data-test-id='email_continue_button_enabled']"
	SelectorTicketListLink         = ".global-info__orders-link-content__button"
	SelectorTicketCard             = ".tickets__ticket-container__card"
	SelectorTicketActionButton     = "[data-test-id='button-action']"
	SelectorTicketId               = ".ticket-detail-header__simplified-invoice"
	SelectorProductRow             = "[data-test-id='ticket-products-product']"
	SelectorProductName            = "[data-test-id='ticket-products-product-name']"
	SelectorProductQuantity        = "[data-test-id='ticket-products-product-quantity']"
	SelectorProductWeight          = "[data-test-id='ticket-products-product-weight']"
	SelectorProductCancelled       = ".ticket-products__content-data-quantity--cancel"
	SelectorProductAmount          = "[data-test-id='ticket-products-product-amount']"
	SelectorProductPromotionAmount = "[data-test-id='ticket-products-product-promotion-amount']"
	SelectorCloseModal             = "[data-test-id='ticket-detail-modal-cross']"
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

func LoginToDia(ctx context.Context, credentialsPath string, cookiesPath string) (*rod.Page, func(), error) {
	// Launch browser (headless by default)

	userDataDir := os.Getenv("CHROME_USER_DATA_DIR")
	if userDataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			homeDir = os.TempDir()
		}
		// Dedicated profile for automation, NOT Chrome's default dir
		userDataDir = filepath.Join(homeDir, ".config", "autogrocery", "chrome-profile")
	}

	if err := os.MkdirAll(userDataDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create user data dir %q: %w", userDataDir, err)
	}

	// Use system chromium if available, otherwise let Rod download its own
	browserPath := os.Getenv("CHROME_PATH")
	if browserPath == "" {
		if found, _ := launcher.LookPath(); found != "" {
			browserPath = found
		}
	}

	l := launcher.New().
		Headless(constants.HeadlessMode).
		UserDataDir(userDataDir)

	if browserPath != "" {
		l = l.Bin(browserPath)
	}

	l = l.
		Set("disable-blink-features", "AutomationControlled").
		Set("disable-dev-shm-usage").
		Set("no-sandbox")

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

	select {
	case <-ctx.Done():
		cleanup()
		return nil, cleanup, ctx.Err()
	default:
	}

	slog.Info("✓ Navigated to Dia homepage", "STORE", constants.DIA)

	//page.MustReload().MustWaitLoad().MustWaitIdle()
	utils.HumanDelay()

	// Reject all cookies
	cookiesBtn, err := page.Timeout(2 * time.Second).Element(SelectorCookiesReject)
	if err == nil {
		cookiesBtn.MustClick()
	}

	// Check if we got redirected to login — means cookies are expired
	if strings.Contains(page.MustInfo().URL, "/login") {
		return nil, cleanup, fmt.Errorf("session expired — please re-export cookies from Firefox")
	}

	emailFieldFound, emailField, err := page.Has(SelectorEmailInput)
	if err != nil {
		slog.Debug("Cannot find email input field, skipping", "STORE", constants.DIA)
		return nil, cleanup, err
	}
	if emailFieldFound {
		emailField.MustClick()
		// Type character by character with small delays
		for _, char := range credentials.Dia.Username {
			page.Keyboard.MustType(input.Key(char))
			time.Sleep(time.Duration(50 + rand.Intn(100)) * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			cleanup()
			return nil, cleanup, ctx.Err()
		default:
		}
		page.MustWaitStable()
		utils.HumanDelay()
		slog.Debug("✓ Email added", "STORE", constants.DIA)
	} else {
		slog.Debug("Email input field not found", "STORE", constants.DIA)
	}

	passwordFieldFound, passwordField, err := page.Has(SelectorPasswordInput)
	if err != nil {
		slog.Debug("Cannot find password input field, skipping", "STORE", constants.DIA)
		return nil, cleanup, err
	}
	if passwordFieldFound {
		passwordField.MustClick()
		// Type character by character with small delays
		for _, char := range credentials.Dia.Password {
			page.Keyboard.MustType(input.Key(char))
			time.Sleep(time.Duration(50 + rand.Intn(100)) * time.Millisecond)
		}
		select {
		case <-ctx.Done():
			cleanup()
			return nil, cleanup, ctx.Err()
		default:
		}
		page.MustWaitStable()
		utils.HumanDelay()
		slog.Debug("✓ Password added", "STORE", constants.DIA)
	} else {
		slog.Debug("Password input field not found", "STORE", constants.DIA)
	}

	loginBtnFound, loginBtn, err := page.Has(SelectorLoginButton)
	if err != nil {
		slog.Debug("Cannot find login button, skipping", "STORE", constants.DIA)
		return nil, cleanup, err
	}
	if loginBtnFound {
		loginBtn.Hover()
		time.Sleep(time.Duration(300+rand.Intn(300)) * time.Millisecond)
		loginBtn.MustClick()

		utils.HumanDelay()
		page.MustWaitIdle()
		slog.Debug("✓ Login button clicked", "STORE", constants.DIA)
	} else {
		slog.Debug("Login button not found", "STORE", constants.DIA)
	}

	return page, cleanup, nil
}

func GetTicketList(ctx context.Context, page *rod.Page, lastEntryToCompare sh.Entry, excludedIds []string) []internal.Ticket {
	ticketLisFound, ticketListLink, err := page.Has(SelectorTicketListLink)
	if err != nil {
		slog.Debug("Error finding ticket list link", "ERROR", err)
		return nil
	}
	if !ticketLisFound {
		slog.Warn("ticket list link not found, cookies are outdated, PLEASE REFRESH", "STORE", constants.DIA)
		return nil
	}
	ticketListLink.MustClick()
	utils.HumanDelay()
	page.MustWaitLoad().MustWaitIdle()

	cookiesBtn, err := page.Timeout(2 * time.Second).Element(SelectorCookiesReject)
	if err == nil {
		cookiesBtn.MustClick()
	}

	ticketElements, err := page.Elements(SelectorTicketCard)
	if err != nil {
		slog.Debug("Error finding ticket list items", "ERROR", err)
		return nil
	}
	tickets := make([]internal.Ticket, 0)
	for _, ticketElement := range ticketElements {
		select {
		case <-ctx.Done():
			slog.Info("Dia scraping cancelled by context", "STORE", constants.DIA)
			return tickets
		default:
		}

		text, _ := ticketElement.Text()
		ticketDate, ticketTotal, errTicketDetails := getDateAndTotal(text)
		if errTicketDetails != nil {
			slog.Debug("Skipping malformed ticket card", "ERROR", errTicketDetails)
			continue // skip this card, don't abort the whole run
		}
		ticketDateComparison := lastEntryToCompare.Date.Compare(ticketDate)
		if ticketDateComparison > 0 {
			slog.Debug("Last ticket is older than ticket found, exiting", "STORE", constants.DIA)
			break
		}
		if ticketDateComparison == 0 && utils.FloatsEqual(ticketTotal, lastEntryToCompare.Total) {
			slog.Info("New ticket found has same date and total as last stored ticket, discarding", "DATE", ticketDate, "TOTAL", ticketTotal)
			continue
		}
		ticket := getTicketDetails(ticketElement, page, ticketDate, ticketTotal)
		if ticket != nil {
			if ticket.IsExcluded(excludedIds) {
				slog.Info("Found excluded ticket, discarding", "ID", ticket.Id, "STORE", constants.DIA)
				continue
			}
			tickets = append(tickets, *ticket)
			slog.Debug(ticket.TicketToStr())
		}

	}
	return tickets
}

func getTicketDetails(ticketElement *rod.Element, page *rod.Page, date time.Time, ticketTotal float64) *internal.Ticket {
	ticketBtnFound, ticketBtn, err := ticketElement.Has(SelectorTicketActionButton)
	if err != nil {
		slog.Debug("Cannot find ticket button, skipping", "ERROR", err)
		return nil
	}
	if ticketBtnFound {
		ticketBtn.Hover()
		utils.HumanDelay()
		ticketBtn.MustClick()

		utils.HumanDelay()
		slog.Debug("✓ Ticket button clicked", "STORE", constants.DIA)
	} else {
		slog.Debug("Ticket button not found", "STORE", constants.DIA)
	}

	ticketId, err := page.MustElement(SelectorTicketId).Text()
	if err != nil {
		slog.Debug("Cannot find ticket id, skipping", "STORE", constants.DIA)
		return nil
	}
	ticketId = strings.Replace(ticketId, "Factura simplificada Nº ", "", 1)

	items, err := getItemList(page)
	if err != nil {
		slog.Debug("Error while trying to get ticket items", "TICKET", ticketId, "ERROR", err)
	}

	if !items.IsTotalValid(ticketTotal) {
		slog.Warn("Ticket price does not match up, discarding", "STORE", constants.DIA)
		return nil
	}

	closeTicketModal(page)

	utils.HumanDelay()

	return &internal.Ticket{Id: ticketId, Total: ticketTotal, Items: items, Date: date, Store: constants.DIA}
}

func getDateAndTotal(ticketString string) (extractedDate time.Time, ticketTotal float64, err error) {
	textLines := strings.Split(ticketString, "\n")
	if len(textLines) <= 1 {
		return time.Unix(0, 0), 0.0, fmt.Errorf("ticket string doesn't have enough lines")
	}
	extractedDate, err = time.Parse(constants.TicketDateFormat, strings.TrimSpace(textLines[1]))
	if err != nil {
		slog.Debug("Error parsing date", "ERROR", err)
		return
	}
	total := utils.ParsePrice(textLines[2])
	return extractedDate, total, nil
}

func getItemList(page *rod.Page) (internal.Items, error) {
	rows, err := page.Elements(SelectorProductRow)
	if err != nil {
		slog.Debug("Error while trying to find item row element", "ERROR", err)
		return nil, err
	}
	items := make([]internal.Item, 0, 1)

	for i, row := range rows {
		itemName := getItemName(row)
		itemQty, isCancelled := getItemQty(row)
		pricePerUnit := getPricePerUnit(row, i)
		itemTotalFound := getItemTotal(row)
		totalCorrect := isTotalCorrect(itemQty, pricePerUnit, itemTotalFound)
		if !totalCorrect {
			slog.Debug("Item total not correct, discarding item", "ITEM", itemName, "STORE", constants.DIA)
			continue
		}
		itemDiscount := getDiscount(row)

		//fmt.Println(itemName, itemQty, pricePerUnit, totalCorrect, itemDiscount)
		items = append(items, internal.Item{Name: itemName, Amount: itemQty, Price: pricePerUnit, Cancelled: isCancelled, Discount: itemDiscount})
	}

	items = removeCancelledItems(items)
	items = applyDiscounts(items)
	return items, nil
}

func getItemName(element *rod.Element) string {
	itemName, err := element.MustElement(SelectorProductName).Text()
	if err != nil {
		slog.Debug("Error while trying to find item name element", "ERROR", err)
		return ""
	}
	return itemName
}

func getItemQty(element *rod.Element) (float64, bool) {
	qtyStr := ""
	itemQtyFound, itemQty, err := element.Has(SelectorProductQuantity)
	if err != nil {
		slog.Debug("Error while trying to find item quantity element", "ERROR", err, "STORE", constants.DIA)
		return 0.0, false
	}
	if itemQtyFound {
		qtyStr, err = itemQty.Text()
		if err != nil {
			slog.Debug("Error while trying to find item quantity element", "ERROR", err, "STORE", constants.DIA)
			return 0.0, false
		}
	} else {
		slog.Debug("Item quantity element not found, searching for weight")
		weightFound, weightElement, err := element.Has(SelectorProductWeight)
		if err != nil {
			slog.Debug("Error while trying to find item weight element", "ERROR", err, "STORE", constants.DIA)
			return 0.0, false
		}
		if weightFound {
			qtyStr, _ = weightElement.Text()
		}
	}
	parsedQty := utils.ParseQtyWithPrecision(qtyStr, 3)

	hasCancelled, _, err := element.Has(SelectorProductCancelled)
	if err != nil {
		slog.Debug("Error while trying to read cancelled flag", "ERROR", err, "STORE", constants.DIA)
		return 0, false
	}

	if hasCancelled {
		parsedQty *= -1
	}
	return parsedQty, hasCancelled
}

func getPricePerUnit(element *rod.Element, index int) float64 {
	itemPricePerUnit, err := element.MustElement("[data-test-id='ticket-products-product-price-per-unit-" + strconv.Itoa(index) + "']").Text()
	if err != nil {
		slog.Debug("Error while trying to find item price per unit element", "ERROR", err, "STORE", constants.DIA)
	}
	parsedPrice := utils.ParsePrice(itemPricePerUnit)
	return parsedPrice
}

func getItemTotal(element *rod.Element) float64 {
	itemTotal, err := element.MustElement(SelectorProductAmount).Text()
	if err != nil {
		slog.Debug("Error while trying to find item amount element", "ERROR", err, "STORE", constants.DIA)
	}
	parsedTotal := utils.ParsePrice(itemTotal)
	return parsedTotal
}

func getDiscount(element *rod.Element) float64 {
	hasDiscount, itemDiscount, err := element.Has(SelectorProductPromotionAmount)
	if err != nil {
		slog.Error("Error while trying to find item discount element", "ERROR", err, "STORE", constants.DIA)
		return 0.0
	}
	if !hasDiscount {
		return 0.0
	}
	slog.Debug("Item discount found")
	parsedDiscountStr, err := itemDiscount.Text()
	if err != nil {
		slog.Error("Error while trying to get discount from item discount", "ERROR", err, "STORE", constants.DIA)
		return 0.0
	}
	parsedDiscount := utils.ParsePrice(parsedDiscountStr)
	return parsedDiscount

}

func isTotalCorrect(qty float64, pricePer float64, totalFound float64) bool {
	return utils.FloatsEqual(utils.ToFixed(qty*pricePer, 2), totalFound)
}

func closeTicketModal(page *rod.Page) {
	el, err := page.Element(SelectorCloseModal)
	if err != nil {
		slog.Warn("close button not found", "ERROR", err)
		return
	}

	// Timeout turns the infinite WaitInteractable loop into an error
	if err := el.Timeout(2*time.Second).Click(proto.InputMouseButtonLeft, 1); err != nil {
		slog.Debug("native click failed, using JS click", "ERROR", err)
		if _, err := el.Eval(`() => this.click()`); err != nil {
			slog.Warn("JS click failed", "ERROR", err)
		}
	}
}

func removeCancelledItems(items []internal.Item) []internal.Item {
	indexes := make([]int, 0)
	for i, item := range items {
		if item.Cancelled {
			for j := range items {
				if item.Name == items[j].Name {
					items[j].Amount += item.Amount
				}
			}
			indexes = append(indexes, i)
		}
	}
	items = slices.DeleteFunc(items, func(item internal.Item) bool {
		return item.Cancelled
	})

	return items
}

func applyDiscounts(items []internal.Item) []internal.Item {
	for i := range items {
		if items[i].Discount < 0 {
			items[i].Price = items[i].Price + items[i].Discount/items[i].Amount
		}
	}
	return items
}
