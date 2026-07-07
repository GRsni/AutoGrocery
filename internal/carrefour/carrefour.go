package carrefour

import (
	"autoGrocery/internal"
	"autoGrocery/internal/google/gm"
	"autoGrocery/pkg/constants"
	"autoGrocery/utils"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/devices"
	"github.com/go-rod/rod/lib/input"
	"github.com/go-rod/rod/lib/launcher"
	"google.golang.org/api/gmail/v1"
)

const Gmail2FALabelId = "Label_9053428003199955812"

type Credentials struct {
	Carrefour struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"carrefour"`
}

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

func LoginToCarrefour(manager gm.Manager, credsPath string) (*rod.Page, func(), error) {
	// Launch browser (headless by default)
	l := launcher.New().Headless(false)
	l.Set("disable-blink-features", "AutomationControlled")
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

	slog.Info("✓ Navigated to Carrefour homepage")

	found, el, err := page.Has(".account-menu")
	if err != nil {
		slog.Warn("Cannot find element Account menu button, skipping")
		return nil, cleanup, err
	}
	if found {
		el.MustClick()
		page.MustWaitStable().MustScreenshot("images/debug/carrefour/menu.png")
		slog.Debug("✓ Account menu clicked")
	} else {
		slog.Debug("Account menu not found")
	}

	found, el, err = page.Has(".account-modal__login")
	if err != nil {
		slog.Info("Cannot find element Account menu button, skipping")
		return nil, cleanup, err
	}
	if found {
		el.MustClick()
		page.MustWaitStable()
		//page.MustScreenshot("images/debug/carrefour/login.png")
		slog.Debug("✓ Login menu clicked")
	} else {
		slog.Debug("Login button not found")
	}
	humanDelay()

	slog.Info("✓ Login page open")

	page.Timeout(7 * time.Second)

	page.Mouse.MustMoveTo(1000, 600)
	page.Mouse.MustClick("left")

	slog.Info("✓ Cookies modal dismissed")

	frame := page.MustElement("iframe[src*='/access']")
	loginModal := frame.MustFrame()
	loginModal.MustWaitLoad()

	loginModal.MustElement("#gigya-loginID-133272631659353340").MustInput(creds.Carrefour.Username)
	loginModal.MustElement("#gigya-password-66067780736329300").MustInput(creds.Carrefour.Password)
	humanDelay()
	loginModal.MustElement(`input.submit-ok[type="submit"]`).MustClick()
	loginModal.MustWaitLoad().MustWaitIdle()
	humanDelay()

	handle2FA(manager, page, err)

	page.MustElement(`div.account-menu--logged`).MustClick()
	page.MustWaitStable().MustWaitIdle()
	humanDelay()
	page.MustElement(`a[href*="area-privada/dashboard"]`).MustClick()
	page.MustWaitStable().MustWaitIdle()
	// Optionally click on cookies
	page.Mouse.MustMoveTo(700, 410)
	page.Mouse.MustClick("left")
	page.MustElementR("a", "Ver todas").MustClick()
	page.MustWaitStable().MustWaitIdle()
	time.Sleep(5 * time.Second)

	return page, cleanup, nil
}

func handle2FA(manager gm.Manager, page *rod.Page, err error) {
	_, err2FAElement := TFAModalVisible(page)
	if err2FAElement != nil {
		slog.Error("Error while trying to find 2FA element", "ERROR", err)
		return
	}

	code, err2FA := get2FACode(manager)
	if err2FA != nil {
		slog.Error("Error while fetching 2FA code", "ERROR", err2FA)
	}

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

func get2FACode(manager gm.Manager) (string, error) {
	var messages []*gmail.Message
	for len(messages) == 0 {
		messages = gm.GetMessagesFromLabel(manager, Gmail2FALabelId)
		time.Sleep(1 * time.Second)
	}

	latest := messages[0]
	subject := latest.Payload.Headers[19].Value
	subjectParts := strings.Split(subject, " ")
	code := subjectParts[len(subjectParts)-1]

	gm.DeleteMessage(manager, latest.Id)

	return code, nil
}

func GetTicketList(page *rod.Page, lastFound time.Time) []internal.Ticket {

	var totalRows, errGetRowsNumber = getNumberOfTickets(page)
	if errGetRowsNumber != nil {
		slog.Error("Unable to get list of tickets", "ERROR", errGetRowsNumber)
	}
	tickets := make([]internal.Ticket, 0, totalRows)

	for i := 0; i < totalRows; i++ {
		pageRows, errTicketList := page.Elements(`div.ticket-elem`)
		if errTicketList != nil {
			slog.Debug("Error finding ticket list items", "ERROR", errTicketList)
			return nil
		}
		row := pageRows[i]
		dateStr := strings.TrimSpace(row.MustElement(`p.date-field`).MustText())
		ticketDate, errParseDate := time.Parse(constants.TicketDateFormat, dateStr)
		if errParseDate != nil {
			slog.Warn("Unable to get ticket date", "ERROR", errParseDate)
			continue
		}
		if lastFound.After(ticketDate) {
			slog.Debug("Last ticket is older than ticket found, exiting", "STORE", constants.CARREFOUR)
			break
		}
		price := row.MustElement(`div.price-field p`).MustText()
		ticket := getTicketDetails(row, page, ticketDate, utils.ParsePrice(price))
		if ticket != nil {
			tickets = append(tickets, *ticket)
			slog.Debug(ticket.TicketToStr())
		}

	}

	return tickets
}

func getNumberOfTickets(page *rod.Page) (int, error) {
	pageRows, errTicketList := page.Elements(`div.ticket-elem`)
	if errTicketList != nil {
		slog.Debug("Error finding ticket list items", "ERROR", errTicketList)
		return 0, errTicketList
	}

	return len(pageRows), nil
}

func getTicketDetails(row *rod.Element, page *rod.Page, date time.Time, total float64) *internal.Ticket {
	row.MustClick().MustWaitLoad()
	page.MustWaitIdle().MustWaitIdle()
	time.Sleep(5 * time.Second)

	hasShowMore, showMoreBtn, err := page.HasR("a", "Mostrar todos")
	if err != nil {
		slog.Debug("Error while finding show more button", "ERROR", err)
		return nil
	}
	if hasShowMore {
		showMoreBtn.Hover()
		humanDelay()
		showMoreBtn.MustClick()
	}
	humanDelay()

	items := getItemList(page, hasShowMore)

	if !items.IsTotalValid(total) {
		slog.Warn("Ticket price does not match up, discarding")
		closeTicketPage(page)
		return nil
	}
	closeTicketPage(page)
	return &internal.Ticket{Id: date.String(), Total: total, Items: items, Date: date, Store: constants.CARREFOUR}
}

func closeTicketPage(page *rod.Page) {
	ticketClose, err := page.ElementR("span.link-crumbs", "Mis compras")
	if err != nil {
		slog.Debug("Cannot find ticket button, skipping")
	}
	ticketClose.MustClick()
	page.MustWaitLoad().MustWaitStable()
	time.Sleep(5 * time.Second)
}

func getItemList(page *rod.Page, hasShowMore bool) internal.Items {
	items := make([]internal.Item, 0, 1)
	rowsSelector := "table.table-white tbody tr"
	if hasShowMore {
		rowsSelector += ":not(:last-child)"
	}
	rows := page.MustElements(rowsSelector)

	for _, row := range rows {
		name := row.MustElement(`td:first-child`).MustText()
		units := utils.ParseQtyWithPrecision(row.MustElement(`td.text-gray span`).MustText(), 3)
		total := utils.ParsePrice(row.MustElement(`td.price strong`).MustText())
		pricePerUnit := total / units

		items = append(items, internal.Item{Name: name, Amount: units, Price: pricePerUnit})
	}

	return items
}
