package carrefour

import (
	"autoGrocery/internal"
	"autoGrocery/internal/google/gm"
	"autoGrocery/internal/google/sh"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"context"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
)

// Selector constants for Carrefour web scraping.
// These map to the DOM structure of carrefour.es and should be updated if the site changes.
const (
	SelectorAccountMenu       = ".account-menu"
	SelectorAccountModalLogin = ".account-modal__login"
	SelectorGigyaIframe       = "iframe[src*='/access']"
	SelectorGigyaLoginID      = "#gigya-loginID-133272631659353340"
	SelectorGigyaPassword     = "#gigya-password-66067780736329300"
	SelectorGigyaSubmit       = `input.submit-ok[type="submit"]`
	SelectorAccountMenuLogged = `div.account-menu--logged`
	SelectorAreaPrivadaLink   = `a[href*="area-privada/dashboard"]`
	SelectorTicketElem        = `div.ticket-elem`
	SelectorTicketDateField   = `p.date-field`
	SelectorTicketPriceField  = `div.price-field p`
	SelectorShowMoreLink      = "a"
	TextShowAll               = "Ver todas"
	SelectorTableFooter       = "table.table-white tfoot tr"
	SelectorDiscountLabel     = "td"
	TextDiscountApplied       = "DESCUENTOS APLICADOS"
	SelectorTicketCloseSpan   = "span.link-crumbs"
	TextMisCompras            = "Mis compras"
	SelectorTableRow          = "table.table-white tbody tr"
	SelectorTableRowNotLast   = `:not(:last-child)`
	SelectorRowFirstCell      = `td:first-child`
	SelectorRowUnitsSpan      = `td.text-gray span`
	SelectorRowPriceStrong    = `td.price strong`
	Gmail2FALabelId           = "Label_9053428003199955812"
)

type Credentials struct {
	Carrefour struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"carrefour"`
}

// LoadCredentials reads Carrefour credentials from a JSON file.
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

func LoginToCarrefour(ctx context.Context, manager gm.Manager, credsPath string) (*rod.Page, func(), error) {
	// Launch browser (headless by default)
	// Use system chromium if available, otherwise let Rod download its own
	browserPath := os.Getenv("CHROME_PATH")
	if browserPath == "" {
		if found, _ := launcher.LookPath(); found != "" {
			browserPath = found
		}
	}

	l := launcher.New().
		Headless(constants.HeadlessMode)

	if browserPath != "" {
		l = l.Bin(browserPath)
	}

	l = l.Set("disable-blink-features", "AutomationControlled")
	u := l.MustLaunch()

	// Rod sets navigator.languages to [d.AcceptLanguage] for pages.
	d := devices.LaptopWithMDPIScreen.Landscape() // the default device
	d.AcceptLanguage = "es-ES"                    // set to your locale

	browser := rod.New().ControlURL(u).MustConnect().DefaultDevice(d)
	cleanup := func() {
		browser.MustClose()
		l.Cleanup()
		l.Kill()
	}

	creds, credsLoadError := LoadCredentials(credsPath)
	if credsLoadError != nil {
		slog.Error("Error while loading credentials for carrefour", "ERROR", credsLoadError)
		return nil, cleanup, credsLoadError
	}

	page := browser.MustPage("https://www.carrefour.es/").MustWaitStable()

	slog.Info("✓ Navigated to Carrefour homepage", "STORE", constants.CARREFOUR)

	found, el, err := page.Has(SelectorAccountMenu)
	if err != nil {
		slog.Warn("Cannot find element Account menu button, skipping")
		return nil, cleanup, err
	}
	if found {
		el.MustClick()
		page.MustWaitStable().MustScreenshot("images/debug/carrefour/menu.png")
		slog.Debug("✓ Account menu clicked", "STORE", constants.CARREFOUR)
	} else {
		slog.Debug("Account menu not found", "STORE", constants.CARREFOUR)
	}

	found, el, err = page.Has(SelectorAccountModalLogin)
	if err != nil {
		slog.Info("Cannot find element Account menu button, skipping")
		return nil, cleanup, err
	}
	if found {
		el.MustClick()
		page.MustWaitStable()
		//page.MustScreenshot("images/debug/carrefour/login.png")
		slog.Debug("✓ Login menu clicked", "STORE", constants.CARREFOUR)
	} else {
		slog.Debug("Login button not found", "STORE", constants.CARREFOUR)
	}
	utils.HumanDelay()

	slog.Info("✓ Login page open", "STORE", constants.CARREFOUR)

	page.Timeout(7 * time.Second)

	page.Mouse.MustMoveTo(1000, 600)
	page.Mouse.MustClick("left")

	slog.Info("✓ Cookies modal dismissed", "STORE", constants.CARREFOUR)

	// Safely find the Gigya iframe — if absent, site layout may have changed
	frameEl, err := page.Element(SelectorGigyaIframe)
	if err != nil {
		slog.Error("Gigya iframe not found — Carrefour site layout may have changed", "ERROR", err)
		return nil, cleanup, fmt.Errorf("cannot find Gigya login iframe: %w", err)
	}
	frame := frameEl.MustFrame()
	frame.MustWaitLoad()

	frame.MustElement(SelectorGigyaLoginID).MustInput(creds.Carrefour.Username)
	frame.MustElement(SelectorGigyaPassword).MustInput(creds.Carrefour.Password)
	utils.HumanDelay()
	frame.MustElement(SelectorGigyaSubmit).MustClick()
	frame.MustWaitLoad().MustWaitIdle()
	utils.HumanDelay()
	slog.Debug("Entered user details, waiting for 2FA email", "STORE", "CARREFOUR")

	handle2FA(ctx, manager, page)

	page.MustElement(SelectorAccountMenuLogged).MustClick()
	page.MustWaitStable().MustWaitIdle()
	utils.HumanDelay()
	page.MustElement(SelectorAreaPrivadaLink).MustClick()
	page.MustWaitStable().MustWaitIdle()
	// Optionally click on cookies
	page.Mouse.MustMoveTo(700, 410)
	page.Mouse.MustClick("left")
	page.MustElementR(SelectorShowMoreLink, TextShowAll).MustClick()
	page.MustWaitStable().MustWaitIdle()
	slog.Info("Logged into Carrefour, loading tickets now", "STORE", "CARREFOUR")

	return page, cleanup, nil
}

func handle2FA(ctx context.Context, manager gm.Manager, page *rod.Page) {
	_, err2FAElement := TFAModalVisible(page)
	if err2FAElement != nil {
		slog.Error("Error while trying to find 2FA element", "ERROR", err2FAElement)
		return
	}

	code, err2FA := get2FACode(ctx, manager)
	if err2FA != nil {
		slog.Error("Error while fetching 2FA code", "ERROR", err2FA)
	}
	slog.Debug("Received 2FA message, inputing code", "STORE", "CARREFOUR")

	page.Mouse.MustMoveTo(850, 450).MustClick("left")
	keys := make([]input.Key, 0, len(code))
	for _, r := range code {
		keys = append(keys, input.Key(r))
	}
	page.Keyboard.MustType(keys...)
	page.Timeout(3 * time.Second).MustWaitIdle()
	page.Mouse.MustMoveTo(900, 730).MustClick("left")
	page.Timeout(6 * time.Second)
}

func TFAModalVisible(page *rod.Page) (bool, error) {
	frames := page.MustElements("iframe")
	for _, f := range frames {
		src, err := f.Attribute("src")
		if err != nil || src == nil || !strings.Contains(*src, "/access") {
			continue
		}

		visible, err := f.Visible()
		if err != nil || !visible {
			return false, err
		}

		framePage := f.MustFrame()

		exists, el, err := framePage.Has(`#code1, #gigya-tfa-form`)
		if err != nil {
			return false, err
		}
		if !exists {
			return false, nil
		}

		return el.Visible()
	}
	return false, nil
}

func get2FACode(ctx context.Context, manager gm.Manager) (string, error) {
	for {
		messages := gm.GetMessagesFromLabel(ctx, manager, Gmail2FALabelId)
		if len(messages) > 0 {
			latest := messages[0]
			subject := latest.Payload.Headers[19].Value
			subjectParts := strings.Split(subject, " ")
			code := subjectParts[len(subjectParts)-1]

			gm.DeleteMessage(ctx, manager, latest.Id)

			return code, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
}

func GetTicketList(page *rod.Page, lastEntryToCompare sh.Entry, excludedIds []string) []internal.Ticket {

	var totalRows, errGetRowsNumber = getNumberOfTickets(page)
	if errGetRowsNumber != nil {
		slog.Error("Unable to get list of tickets", "ERROR", errGetRowsNumber)
	}
	tickets := make([]internal.Ticket, 0, totalRows)

	for i := 0; i < totalRows; i++ {
		pageRows, errTicketList := page.Elements(SelectorTicketElem)
		if errTicketList != nil {
			slog.Debug("Error finding ticket list items", "ERROR", errTicketList)
			return []internal.Ticket{} // consistent with Dia/Mercadona: empty slice, not nil
		}
		row := pageRows[i]
		dateStr := strings.TrimSpace(row.MustElement(SelectorTicketDateField).MustText())
		ticketDate, errParseDate := time.Parse(constants.TicketDateFormat, dateStr)
		if errParseDate != nil {
			slog.Warn("Unable to get ticket date", "ERROR", errParseDate)
			continue
		}
		ticketDateComparison := lastEntryToCompare.Date.Compare(ticketDate)
		if ticketDateComparison > 0 {
			slog.Debug("Last ticket is older than ticket found, exiting", "STORE", constants.CARREFOUR)
			break
		}
		ticketTotal := utils.ParsePrice(row.MustElement(SelectorTicketPriceField).MustText())
		if ticketDateComparison == 0 && utils.FloatsEqual(ticketTotal, lastEntryToCompare.Total) {
			slog.Info("New ticket found has same date and total as last stored ticket, discarding", "DATE", ticketDate, "TOTAL", ticketTotal)
			continue
		}

		ticketId := getId(ticketDate, ticketTotal)
		if slices.Contains(excludedIds, ticketId) {
			slog.Info("Found excluded ticket, discarding", "ID", ticketId)
			continue
		}

		row.MustClick().MustWaitLoad()
		page.MustWaitIdle().MustWaitIdle()
		time.Sleep(5 * time.Second)

		ticket := getTicketDetails(page, ticketDate, ticketTotal)
		if ticket != nil {
			tickets = append(tickets, *ticket)
			slog.Debug(ticket.TicketToStr())
		}

	}

	return tickets
}

func getNumberOfTickets(page *rod.Page) (int, error) {
	pageRows, errTicketList := page.Elements(SelectorTicketElem)
	if errTicketList != nil {
		slog.Debug("Error finding ticket list items", "ERROR", errTicketList)
		return 0, errTicketList
	}

	return len(pageRows), nil
}

func getTicketDetails(page *rod.Page, date time.Time, total float64) *internal.Ticket {
	hasShowMore, showMoreBtn, errShowAll := page.HasR(SelectorShowMoreLink, TextShowAll)
	if errShowAll != nil {
		slog.Error("Error while finding show more button", "ERROR", errShowAll)
		return nil
	}
	if hasShowMore {
		showMoreBtn.Hover()
		utils.HumanDelay()
		showMoreBtn.MustClick()
	}
	utils.HumanDelay()

	items := getItemList(page, hasShowMore)

	//page.MustElement("table.table-white tfoot tr").MustElement("td.price").MustText()
	totalTicketDiscount := getTotalTicketDiscount(page)
	if math.Abs(totalTicketDiscount) > 0 {
		totalDiscountShare := totalTicketDiscount / (float64)(len(items))
		slog.Info("Found general discount applied to ticket, will be split between items", "DISCOUNT", totalTicketDiscount)
		for i := range items {
			items[i].Price = items[i].Price + totalDiscountShare/items[i].Amount
		}
	}

	if !items.IsTotalValidWithPrecision(total, 0.02) {
		slog.Warn("Ticket price does not match up, discarding")
		closeTicketPage(page)
		return nil
	}
	closeTicketPage(page)
	return &internal.Ticket{Id: getId(date, total), Total: total, Items: items, Date: date, Store: constants.CARREFOUR}
}

func getTotalTicketDiscount(page *rod.Page) float64 {
	var numRe = regexp.MustCompile(`^-?\d+(?:[.,]\d+)?`)
	hasDiscount, discountRow, errHasDiscount := page.Has(SelectorTableFooter)
	hasDiscount2, _, errHasDiscount2 := page.HasR(SelectorDiscountLabel, TextDiscountApplied)
	if errHasDiscount != nil || errHasDiscount2 != nil {
		slog.Error("Error while getting table footer", "ERROR", errHasDiscount)
	}
	if hasDiscount && hasDiscount2 {
		totalDiscountStr := discountRow.MustElement("td.price").MustText()
		if len(totalDiscountStr) > 0 {
			slog.Debug(totalDiscountStr)
			ticketDiscount := utils.ParsePrice(numRe.FindString(totalDiscountStr))
			return ticketDiscount
		}
	}
	return 0.0
}

func closeTicketPage(page *rod.Page) {
	ticketClose, err := page.ElementR(SelectorTicketCloseSpan, TextMisCompras)
	if err != nil {
		slog.Debug("Cannot find ticket button, skipping")
	}
	ticketClose.MustClick()
	page.MustWaitLoad().MustWaitStable()
	time.Sleep(5 * time.Second)
}

func getItemList(page *rod.Page, hasShowMore bool) internal.Items {
	items := make([]internal.Item, 0, 1)
	rowsSelector := SelectorTableRow
	if hasShowMore {
		rowsSelector += SelectorTableRowNotLast
	}
	rows := page.MustElements(rowsSelector)

	for _, row := range rows {
		name := row.MustElement(SelectorRowFirstCell).MustText()
		units := utils.ParseQtyWithPrecision(row.MustElement(SelectorRowUnitsSpan).MustText(), 3)
		total := utils.ParsePrice(row.MustElement(SelectorRowPriceStrong).MustText())
		pricePerUnit := total / units

		items = append(items, internal.Item{Name: name, Amount: units, Price: pricePerUnit})
	}
	return items
}

func getId(date time.Time, total float64) string {
	return date.Format("2006-01-02") + "-" + strconv.FormatFloat(total, 'f', -1, 64)
}
