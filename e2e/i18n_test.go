//go:build e2e

package e2e_test

import (
	"regexp"
	"strings"
	"testing"

	playwright "github.com/playwright-community/playwright-go"
)

var (
	reDate   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	reISO8601 = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`)
)

func newPageWithLocale(t *testing.T, locale string) playwright.Page {
	t.Helper()
	bctx, err := browser.NewContext(playwright.BrowserNewContextOptions{
		Locale: playwright.String(locale),
	})
	if err != nil {
		t.Fatalf("new browser context: %v", err)
	}
	t.Cleanup(func() { bctx.Close() })
	page, err := bctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	return page
}

// TestDatePickerDataAttributes verifies that data-timezone and data-date attributes
// are rendered correctly into the HTML by the server templates.
func TestDatePickerDataAttributes(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "I18n Attributes Test", "i18n-attrs-test")

	page := newPage(t)
	if _, err := page.Goto(serverURL + "/calendar/" + username + "/i18n-attrs-test"); err != nil {
		t.Fatalf("navigate to picker: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}

	tz, err := page.Evaluate(`document.querySelector('.date-nav').dataset.timezone`)
	if err != nil {
		t.Fatalf("read data-timezone: %v", err)
	}
	tzStr, _ := tz.(string)
	if tzStr == "" {
		t.Error("data-timezone on .date-nav must be non-empty")
	}

	dateAttr, err := page.Evaluate(`document.querySelector('.date-btn').dataset.date`)
	if err != nil {
		t.Fatalf("read data-date: %v", err)
	}
	dateStr, _ := dateAttr.(string)
	if !reDate.MatchString(dateStr) {
		t.Errorf("data-date on .date-btn must match YYYY-MM-DD, got %q", dateStr)
	}
}

// TestSlotButtonDataTime verifies data-time is present on slot buttons for the initial
// page load and after an HTMX partial swap triggered by clicking a second date.
func TestSlotButtonDataTime(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "I18n Slot Data Test", "i18n-slot-data-test")
	enableAllDays(t, dashPage)

	page := newPage(t)
	if _, err := page.Goto(serverURL + "/calendar/" + username + "/i18n-slot-data-test"); err != nil {
		t.Fatalf("navigate to picker: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}

	iso8601 := reISO8601

	// Find initial date with slots, then verify data-time.
	var initialDateIdx int
	for i := 0; i < 7; i++ {
		count, _ := page.Locator(".slot-btn").Count()
		if count > 0 {
			initialDateIdx = i
			break
		}
		if err := page.Locator(".date-btn").Nth(i + 1).Click(); err != nil {
			t.Fatalf("click date btn %d: %v", i+1, err)
		}
		page.WaitForTimeout(600)
	}

	dataTime, err := page.Evaluate(`document.querySelector('.slot-btn').dataset.time`)
	if err != nil {
		t.Fatalf("read data-time from initial slot: %v", err)
	}
	dtStr, _ := dataTime.(string)
	if !iso8601.MatchString(dtStr) {
		t.Errorf("data-time on .slot-btn must match ISO 8601 UTC, got %q", dtStr)
	}

	// Click a different date to trigger HTMX swap and verify data-time on swapped slots.
	nextIdx := (initialDateIdx + 1) % 7
	_, err = page.ExpectResponse("**/calendar/**", func() error {
		return page.Locator(".date-btn").Nth(nextIdx).Click()
	})
	if err != nil {
		t.Fatalf("wait for HTMX swap response: %v", err)
	}
	page.WaitForTimeout(300)

	count, _ := page.Locator(".slot-btn").Count()
	if count > 0 {
		dataTime2, err := page.Evaluate(`document.querySelector('.slot-btn').dataset.time`)
		if err != nil {
			t.Fatalf("read data-time after HTMX swap: %v", err)
		}
		dt2Str, _ := dataTime2.(string)
		if !iso8601.MatchString(dt2Str) {
			t.Errorf("data-time after HTMX swap must match ISO 8601 UTC, got %q", dt2Str)
		}
	}
}

// TestLocaleFormattingGerman verifies that the JS locale IIFE rewrites date and time
// display text to German format when the browser locale is de-DE.
func TestLocaleFormattingGerman(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "I18n German Test", "i18n-german-test")
	enableAllDays(t, dashPage)

	page := newPageWithLocale(t, "de-DE")
	if _, err := page.Goto(serverURL + "/calendar/" + username + "/i18n-german-test"); err != nil {
		t.Fatalf("navigate to picker: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}
	page.WaitForTimeout(300)

	lang, err := page.Evaluate(`document.documentElement.lang`)
	if err != nil {
		t.Fatalf("read html lang: %v", err)
	}
	if lang != "de-DE" {
		t.Errorf("<html lang> must be %q, got %q", "de-DE", lang)
	}

	dayText, err := page.Evaluate(`document.querySelector('.date-btn-day').textContent.trim()`)
	if err != nil {
		t.Fatalf("read .date-btn-day text: %v", err)
	}
	englishDays := []string{"Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"}
	dayStr, _ := dayText.(string)
	for _, eng := range englishDays {
		if strings.EqualFold(dayStr, eng) {
			t.Errorf(".date-btn-day still shows English day %q for de-DE locale", dayStr)
		}
	}

	// Find a date with slots to test time formatting.
	var slotsFound bool
	for i := 0; i < 7; i++ {
		count, _ := page.Locator(".slot-btn").Count()
		if count > 0 {
			slotsFound = true
			break
		}
		if err := page.Locator(".date-btn").Nth(i + 1).Click(); err != nil {
			break
		}
		page.WaitForTimeout(600)
	}
	if !slotsFound {
		t.Skip("no slots available to test time formatting")
	}

	slotText, err := page.Evaluate(`document.querySelector('.slot-btn').textContent.trim()`)
	if err != nil {
		t.Fatalf("read .slot-btn text: %v", err)
	}
	slotStr, _ := slotText.(string)
	if strings.Contains(slotStr, "AM") || strings.Contains(slotStr, "PM") {
		t.Errorf(".slot-btn shows 12h format %q for de-DE locale, expected 24h", slotStr)
	}
}
