//go:build e2e

package e2e_test

import (
	"strings"
	"testing"

	playwright "github.com/playwright-community/playwright-go"
)

// TestDashboardOverview verifies the dashboard overview page loads and shows the
// correct heading and navigation links.
func TestDashboardOverview(t *testing.T) {
	page := mustLogin(t)

	if _, err := page.Goto(serverURL + "/dashboard"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	heading, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read heading: %v", err)
	}
	if heading != "Overview" {
		t.Errorf("heading: got %q, want %q", heading, "Overview")
	}

	// All nav links must be present.
	for _, link := range []string{
		"/dashboard/calendars",
		"/dashboard/availability",
		"/dashboard/event-types",
		"/dashboard/bookings",
		"/dashboard/profile",
	} {
		count, err := page.Locator(`a[href="` + link + `"]`).Count()
		if err != nil {
			t.Fatalf("count nav link %q: %v", link, err)
		}
		if count == 0 {
			t.Errorf("nav link %q not found", link)
		}
	}
}

// TestDashboardCalendars verifies the calendars page renders and that submitting
// an unreachable CalDAV URL shows a connection error.
func TestDashboardCalendars(t *testing.T) {
	page := mustLogin(t)

	if _, err := page.Goto(serverURL + "/dashboard/calendars"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	heading, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read heading: %v", err)
	}
	if heading != "CalDAV Calendars" {
		t.Errorf("heading: got %q, want %q", heading, "CalDAV Calendars")
	}

	// Open the "Add CalDAV calendar" details section.
	if err := page.Click("details summary"); err != nil {
		t.Fatalf("open add form: %v", err)
	}

	// Submit with an unreachable URL; expect an error message.
	if err := page.Fill(`input[name="caldav_url"]`, "http://localhost:9999/dav/"); err != nil {
		t.Fatalf("fill URL: %v", err)
	}
	if err := page.Fill(`input[name="username"]`, "user"); err != nil {
		t.Fatalf("fill username: %v", err)
	}
	if err := page.Fill(`input[name="password"]`, "pass"); err != nil {
		t.Fatalf("fill password: %v", err)
	}
	if err := page.Fill(`input[name="display_name"]`, "Test Cal"); err != nil {
		t.Fatalf("fill display name: %v", err)
	}
	if err := page.Click(`form[action="/dashboard/calendars"] button[type="submit"]`); err != nil {
		t.Fatalf("submit: %v", err)
	}

	// InnerText auto-waits for the element to appear.
	errText, err := page.InnerText(".alert-error")
	if err != nil {
		t.Fatalf("read error message: %v", err)
	}
	if !strings.Contains(errText, "Could not connect") {
		t.Errorf("error message: got %q, want it to contain %q", errText, "Could not connect")
	}
}

// TestDashboardAvailability verifies saving weekly availability rules and
// reading them back on the next page load.
func TestDashboardAvailability(t *testing.T) {
	page := mustLogin(t)

	if _, err := page.Goto(serverURL + "/dashboard/availability"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	heading, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read heading: %v", err)
	}
	if heading != "Weekly Availability" {
		t.Errorf("heading: got %q, want %q", heading, "Weekly Availability")
	}

	// Set Monday (day_1) to 10:00–16:00 and activate it.
	if err := page.Locator(`input[name="day_1_active"]`).Check(); err != nil {
		t.Fatalf("check Monday active: %v", err)
	}
	if err := page.Fill(`input[name="day_1_start"]`, "10:00"); err != nil {
		t.Fatalf("fill start time: %v", err)
	}
	if err := page.Fill(`input[name="day_1_end"]`, "16:00"); err != nil {
		t.Fatalf("fill end time: %v", err)
	}
	// Wait for the POST→redirect→GET sequence to complete before reading back values.
	if err := waitForFormSubmit(page, `form[action="/dashboard/availability"] button[type="submit"]`); err != nil {
		t.Fatalf("save availability: %v", err)
	}

	// Verify the saved values are rendered in the reloaded form.
	start, err := page.InputValue(`input[name="day_1_start"]`)
	if err != nil {
		t.Fatalf("read start: %v", err)
	}
	end, err := page.InputValue(`input[name="day_1_end"]`)
	if err != nil {
		t.Fatalf("read end: %v", err)
	}
	active, err := page.Locator(`input[name="day_1_active"]`).IsChecked()
	if err != nil {
		t.Fatalf("read active: %v", err)
	}

	if start != "10:00" {
		t.Errorf("start: got %q, want %q", start, "10:00")
	}
	if end != "16:00" {
		t.Errorf("end: got %q, want %q", end, "16:00")
	}
	if !active {
		t.Error("Monday active checkbox: got false, want true")
	}
}

// TestDashboardEventTypeCreateAndEdit covers the full event-type lifecycle:
// create, verify in list, edit title and guest message, toggle active state.
func TestDashboardEventTypeCreateAndEdit(t *testing.T) {
	page := mustLogin(t)

	// --- Create ---
	if _, err := page.Goto(serverURL + "/dashboard/event-types"); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if err := page.Click("details summary"); err != nil {
		t.Fatalf("open create form: %v", err)
	}
	if err := page.Fill(`input[name="title"]`, "E2E Test Meeting"); err != nil {
		t.Fatalf("fill title: %v", err)
	}
	if err := page.Fill(`input[name="slug"]`, "e2e-test-meeting"); err != nil {
		t.Fatalf("fill slug: %v", err)
	}
	if err := page.Fill(`input[name="duration_minutes"]`, "45"); err != nil {
		t.Fatalf("fill duration: %v", err)
	}
	if err := page.Click(`form[action="/dashboard/event-types"] button[type="submit"]`); err != nil {
		t.Fatalf("submit create: %v", err)
	}
	// Wait for our new card to appear (there may be others from earlier tests).
	if err := page.Locator(".card-title").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Test Meeting",
	}).WaitFor(); err != nil {
		t.Fatalf("wait for card: %v", err)
	}

	// --- Edit: click the Edit link belonging to the E2E Test Meeting card ---
	if err := page.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Test Meeting",
	}).Locator(`a[href*="/edit"]`).Click(); err != nil {
		t.Fatalf("click edit: %v", err)
	}
	if err := page.Fill(`input[name="title"]`, "E2E Updated Meeting"); err != nil {
		t.Fatalf("fill updated title: %v", err)
	}
	if err := page.Fill(`textarea[name="confirmed_message"]`, "Thanks for booking!"); err != nil {
		t.Fatalf("fill confirmed message: %v", err)
	}
	if err := page.Click(`form[action*="/dashboard/event-types/"] button[type="submit"]`); err != nil {
		t.Fatalf("submit edit: %v", err)
	}
	if err := page.Locator(".card-title").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Updated Meeting",
	}).WaitFor(); err != nil {
		t.Fatalf("wait for updated card: %v", err)
	}

	// --- Toggle (deactivate) the E2E Updated Meeting card ---
	if err := page.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Updated Meeting",
	}).Locator(`form[action*="/toggle"] button`).Click(); err != nil {
		t.Fatalf("toggle deactivate: %v", err)
	}
	if err := page.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Updated Meeting",
	}).Locator(".card-meta").Filter(playwright.LocatorFilterOptions{
		HasText: "Inactive",
	}).WaitFor(); err != nil {
		t.Fatalf("wait for inactive state: %v", err)
	}

	// --- Toggle back (activate) ---
	if err := page.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Updated Meeting",
	}).Locator(`form[action*="/toggle"] button`).Click(); err != nil {
		t.Fatalf("toggle activate: %v", err)
	}
	if err := page.Locator(".card").Filter(playwright.LocatorFilterOptions{
		HasText: "E2E Updated Meeting",
	}).Locator(".card-meta").Filter(playwright.LocatorFilterOptions{
		HasText: "Active",
	}).WaitFor(); err != nil {
		t.Fatalf("wait for active state: %v", err)
	}
}

// TestDashboardBookings verifies the bookings list page and its status filter links.
func TestDashboardBookings(t *testing.T) {
	page := mustLogin(t)

	if _, err := page.Goto(serverURL + "/dashboard/bookings"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	heading, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read heading: %v", err)
	}
	if heading != "Bookings" {
		t.Errorf("heading: got %q, want %q", heading, "Bookings")
	}

	// Filter links should navigate to the correct URLs.
	for _, status := range []string{"PENDING", "CONFIRMED", "REJECTED", "CANCELLED"} {
		// Wrap in ExpectNavigation to reliably capture the redirect even when
		// the server responds before WaitForURL can set up its listener.
		if _, err := page.ExpectNavigation(func() error {
			return page.Click(`a[href="/dashboard/bookings?status=` + status + `"]`)
		}); err != nil {
			t.Errorf("filter %s: navigate: %v", status, err)
			continue
		}
		if !strings.Contains(page.URL(), "status="+status) {
			t.Errorf("filter %s: URL %q does not contain status param", status, page.URL())
		}
		// Navigate back so the next filter click starts from the right page.
		if _, err := page.Goto(serverURL + "/dashboard/bookings"); err != nil {
			t.Fatalf("navigate back for filter %s: %v", status, err)
		}
	}
}

// TestDashboardProfile verifies changing the display name and username slug.
func TestDashboardProfile(t *testing.T) {
	page := mustLogin(t)

	if _, err := page.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	heading, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read heading: %v", err)
	}
	if heading != "Profile" {
		t.Errorf("heading: got %q, want %q", heading, "Profile")
	}

	// Change display name.
	if err := page.Fill(`input[name="name"]`, "E2E Admin"); err != nil {
		t.Fatalf("fill name: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("save name: %v", err)
	}
	savedName, err := page.InputValue(`input[name="name"]`)
	if err != nil {
		t.Fatalf("read saved name: %v", err)
	}
	if savedName != "E2E Admin" {
		t.Errorf("saved name: got %q, want %q", savedName, "E2E Admin")
	}

	// Change username slug.
	if err := page.Fill(`input[name="username"]`, "e2e-admin"); err != nil {
		t.Fatalf("fill slug: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("save slug: %v", err)
	}
	savedSlug, err := page.InputValue(`input[name="username"]`)
	if err != nil {
		t.Fatalf("read saved slug: %v", err)
	}
	if savedSlug != "e2e-admin" {
		t.Errorf("saved slug: got %q, want %q", savedSlug, "e2e-admin")
	}

	// Public booking page should be accessible at the new slug.
	if _, err := page.Goto(serverURL + "/calendar/e2e-admin/"); err != nil {
		t.Fatalf("navigate to booking page: %v", err)
	}
	bookingPageTitle, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read booking page title: %v", err)
	}
	if !strings.Contains(bookingPageTitle, "E2E Admin") {
		t.Errorf("booking page title: got %q, want it to contain %q", bookingPageTitle, "E2E Admin")
	}

	// Navigate back to profile to change email address.
	if _, err := page.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile for email change: %v", err)
	}

	// Change email address.
	if err := page.Fill(`input[name="email"]`, "changed@example.com"); err != nil {
		t.Fatalf("fill email: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("save email: %v", err)
	}
	savedEmail, err := page.InputValue(`input[name="email"]`)
	if err != nil {
		t.Fatalf("read saved email: %v", err)
	}
	if savedEmail != "changed@example.com" {
		t.Errorf("saved email: got %q, want %q", savedEmail, "changed@example.com")
	}

	// Validate that an empty name shows a server-side error.
	if _, err := page.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate back to profile: %v", err)
	}
	// Use JS to blank the name and remove required in one atomic step,
	// avoiding a stale-node-ID race between Fill and Evaluate.
	if _, err := page.Evaluate(`
		var n = document.querySelector('input[name="name"]');
		n.removeAttribute('required');
		n.value = '';
	`); err != nil {
		t.Fatalf("clear name via JS: %v", err)
	}
	if err := page.Click(`form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("submit empty name: %v", err)
	}
	// WaitFor auto-waits for the error element to appear after the form submission.
	if err := page.Locator(".alert-error").WaitFor(); err != nil {
		t.Fatalf("wait for error: %v", err)
	}
}
