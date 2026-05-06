//go:build e2e

package e2e_test

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	playwright "github.com/playwright-community/playwright-go"
)

// TestWidgetNoNavigation verifies the widget page renders without the site header.
func TestWidgetNoNavigation(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "Widget Nav Test", "widget-nav-test")

	page := newPage(t)
	if _, err := page.Goto(serverURL + "/widget/" + username + "/widget-nav-test"); err != nil {
		t.Fatalf("navigate to widget: %v", err)
	}

	count, err := page.Locator(".site-header").Count()
	if err != nil {
		t.Fatalf("count site-header: %v", err)
	}
	if count > 0 {
		t.Error("widget page must not render .site-header navigation")
	}
}

// TestWidgetIframeable checks that widget routes omit X-Frame-Options and set frame-ancestors CSP,
// while regular calendar routes still set X-Frame-Options: DENY.
func TestWidgetIframeable(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "Widget Frame Test", "widget-frame-test")

	widgetURL := serverURL + "/widget/" + username + "/widget-frame-test"
	resp, err := http.Get(widgetURL) //nolint:gosec
	if err != nil {
		t.Fatalf("GET widget url: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("widget route: got %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Frame-Options"); got != "" {
		t.Errorf("widget route must not set X-Frame-Options, got %q", got)
	}
	if got := resp.Header.Get("Content-Security-Policy"); !strings.Contains(got, "frame-ancestors") {
		t.Errorf("widget route must set frame-ancestors in CSP, got %q", got)
	}

	calURL := serverURL + "/calendar/" + username + "/widget-frame-test"
	calResp, err := http.Get(calURL) //nolint:gosec
	if err != nil {
		t.Fatalf("GET calendar url: %v", err)
	}
	calResp.Body.Close()
	if got := calResp.Header.Get("X-Frame-Options"); got != "DENY" {
		t.Errorf("calendar route must set X-Frame-Options: DENY, got %q", got)
	}
}

// TestWidgetEmbedCode verifies the embed snippet appears on the event-type edit page.
func TestWidgetEmbedCode(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "Widget Embed Code Test", "widget-embed-code")

	// Navigate to the edit page via the card belonging to the event type we just created.
	if err := dashPage.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "Widget Embed Code Test",
	}).Locator(`a[href*="/edit"]`).Click(); err != nil {
		t.Fatalf("click edit link: %v", err)
	}
	if err := dashPage.WaitForURL("**/event-types/**/edit"); err != nil {
		t.Fatalf("wait for edit page: %v", err)
	}

	embedCode, err := dashPage.InputValue("#embed-code")
	if err != nil {
		t.Fatalf("read embed code textarea: %v", err)
	}
	if !strings.Contains(embedCode, "/static/widget.js") {
		t.Errorf("embed code must reference widget.js, got: %q", embedCode)
	}
	if !strings.Contains(embedCode, `data-user="`+username+`"`) {
		t.Errorf("embed code must contain data-user=%q, got: %q", username, embedCode)
	}
	if !strings.Contains(embedCode, `data-slug="widget-embed-code"`) {
		t.Errorf("embed code must contain data-slug, got: %q", embedCode)
	}
}

// TestWidgetBookingFlow exercises the full booking lifecycle through the widget URL.
func TestWidgetBookingFlow(t *testing.T) {
	dashPage := mustLogin(t)

	if _, err := dashPage.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := dashPage.InputValue(`input[name="username"]`)
	if err != nil || username == "" {
		t.Fatalf("read username: %v", err)
	}

	createEventType(t, dashPage, "Widget Booking Flow", "widget-booking-flow")
	enableAllDays(t, dashPage)

	// Guest loads the widget URL directly.
	guestPage := newPage(t)
	if _, err := guestPage.Goto(serverURL + "/widget/" + username + "/widget-booking-flow"); err != nil {
		t.Fatalf("navigate to widget: %v", err)
	}
	if _, err := guestPage.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}

	// Iterate date buttons until a date with slots is found.
	var slotFound bool
	for i := 0; i < 7; i++ {
		if err := guestPage.Locator(".date-btn").Nth(i).Click(); err != nil {
			t.Fatalf("click date button %d: %v", i, err)
		}
		guestPage.WaitForTimeout(600)
		count, err := guestPage.Locator(".slot-btn").Count()
		if err != nil {
			t.Fatalf("count slots for date %d: %v", i, err)
		}
		if count > 0 {
			slotFound = true
			break
		}
	}
	if !slotFound {
		t.Fatal("no date with available slots found in the 7-day window")
	}

	if err := guestPage.Locator(".slot-btn").First().Click(); err != nil {
		t.Fatalf("click first slot: %v", err)
	}
	if _, err := guestPage.WaitForSelector("#booking-form"); err != nil {
		t.Fatalf("wait for booking form: %v", err)
	}
	if err := guestPage.Fill(`input[name="guest_name"]`, "Widget Guest"); err != nil {
		t.Fatalf("fill guest name: %v", err)
	}
	if err := guestPage.Fill(`input[name="guest_email"]`, "widget-guest@example.com"); err != nil {
		t.Fatalf("fill guest email: %v", err)
	}
	if err := guestPage.Click(`button[type="submit"]`); err != nil {
		t.Fatalf("submit booking: %v", err)
	}

	pageTitle, err := guestPage.InnerText("h1")
	if err != nil {
		t.Fatalf("read page title: %v", err)
	}
	if !strings.Contains(pageTitle, "Booking request sent") {
		t.Errorf("submit page title: got %q, want it to contain %q", pageTitle, "Booking request sent")
	}

	// Confirmation page must also have no site header.
	count, err := guestPage.Locator(".site-header").Count()
	if err != nil {
		t.Fatalf("count site-header on confirm page: %v", err)
	}
	if count > 0 {
		t.Error("widget confirm page must not render .site-header navigation")
	}

	// Host verifies the booking appears as PENDING.
	if _, err := dashPage.Goto(serverURL + "/dashboard/bookings"); err != nil {
		t.Fatalf("navigate to bookings: %v", err)
	}
	if _, err := dashPage.WaitForSelector("table"); err != nil {
		t.Fatalf("wait for bookings table: %v", err)
	}
	pendingBadge, err := dashPage.InnerText(".badge-pending")
	if err != nil {
		t.Fatalf("read pending badge: %v", err)
	}
	if !strings.EqualFold(pendingBadge, "PENDING") {
		t.Errorf("expected PENDING booking, got %q", pendingBadge)
	}
}

// createEventType navigates to the event types dashboard and creates a new one.
// The caller must already be logged in and the dashPage must be at any dashboard URL.
func createEventType(t *testing.T, page playwright.Page, title, slug string) {
	t.Helper()
	if _, err := page.Goto(serverURL + "/dashboard/event-types"); err != nil {
		t.Fatalf("navigate to event types: %v", err)
	}
	if err := page.Click("details summary"); err != nil {
		t.Fatalf("open create form: %v", err)
	}
	if err := page.Fill(`input[name="title"]`, title); err != nil {
		t.Fatalf("fill title: %v", err)
	}
	if err := page.Fill(`input[name="slug"]`, slug); err != nil {
		t.Fatalf("fill slug: %v", err)
	}
	if err := page.Fill(`input[name="duration_minutes"]`, "30"); err != nil {
		t.Fatalf("fill duration: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/event-types"] button[type="submit"]`); err != nil {
		t.Fatalf("submit create event type: %v", err)
	}
}

// enableAllDays activates all 7 weekdays in the availability settings.
func enableAllDays(t *testing.T, page playwright.Page) {
	t.Helper()
	if _, err := page.Goto(serverURL + "/dashboard/availability"); err != nil {
		t.Fatalf("navigate to availability: %v", err)
	}
	for i := 0; i < 7; i++ {
		if err := page.Locator(fmt.Sprintf(`input[name="day_%d_active"]`, i)).Check(); err != nil {
			t.Fatalf("check day %d: %v", i, err)
		}
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/availability"] button[type="submit"]`); err != nil {
		t.Fatalf("submit availability: %v", err)
	}
}
