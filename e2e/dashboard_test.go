//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestDashboardOverview verifies the dashboard overview page loads and shows the
// correct heading and navigation links.
func TestDashboardOverview(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var heading string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate dashboard: %v", err)
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
		var exists bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`!!document.querySelector('a[href="'+`+"`"+link+"`"+`+'"]')`, &exists),
		); err != nil || !exists {
			t.Errorf("nav link %q not found", link)
		}
	}
}

// TestDashboardCalendars verifies the calendars page renders and that submitting
// an unreachable CalDAV URL shows a connection error.
func TestDashboardCalendars(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var heading string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/calendars"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if heading != "CalDAV Calendars" {
		t.Errorf("heading: got %q, want %q", heading, "CalDAV Calendars")
	}

	// Open the "Add CalDAV calendar" details section.
	if err := chromedp.Run(ctx,
		chromedp.Click(`details summary`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="caldav_url"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("open add form: %v", err)
	}

	// Submit with an unreachable URL; expect an error message.
	var errText string
	if err := chromedp.Run(ctx,
		chromedp.SetValue(`input[name="caldav_url"]`, "http://localhost:9999/dav/", chromedp.ByQuery),
		chromedp.SetValue(`input[name="username"]`, "user", chromedp.ByQuery),
		chromedp.SetValue(`input[name="password"]`, "pass", chromedp.ByQuery),
		chromedp.SetValue(`input[name="display_name"]`, "Test Cal", chromedp.ByQuery),
		chromedp.Click(`form[action="/dashboard/calendars"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.alert-error`, chromedp.ByQuery),
		chromedp.Text(`.alert-error`, &errText, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("bad calendar submit: %v", err)
	}

	if !strings.Contains(errText, "Could not connect") {
		t.Errorf("error message: got %q, want it to contain %q", errText, "Could not connect")
	}
}

// TestDashboardAvailability verifies saving weekly availability rules and
// reading them back on the next page load.
func TestDashboardAvailability(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var heading string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/availability"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if heading != "Weekly Availability" {
		t.Errorf("heading: got %q, want %q", heading, "Weekly Availability")
	}

	// Set Monday (day_1) to 10:00–16:00 and activate it.
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('input[name="day_1_active"]').checked = true`, nil),
		chromedp.SetValue(`input[name="day_1_start"]`, "10:00", chromedp.ByQuery),
		chromedp.SetValue(`input[name="day_1_end"]`, "16:00", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("save availability: %v", err)
	}
	// Wait for the POST→redirect→GET sequence to complete before reading back values.
	if err := waitForFormSubmit(ctx, `form[action="/dashboard/availability"] button[type="submit"]`); err != nil {
		t.Fatalf("save availability: %v", err)
	}

	// Verify the saved values are rendered in the reloaded form.
	var start, end string
	var active bool
	if err := chromedp.Run(ctx,
		chromedp.Value(`input[name="day_1_start"]`, &start, chromedp.ByQuery),
		chromedp.Value(`input[name="day_1_end"]`, &end, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="day_1_active"]').checked`, &active),
	); err != nil {
		t.Fatalf("read back: %v", err)
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
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// --- Create ---
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/event-types"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		// Open the create form (inside a <details> element).
		chromedp.Click(`details summary`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="title"]`, "E2E Test Meeting", chromedp.ByQuery),
		chromedp.SetValue(`input[name="slug"]`, "e2e-test-meeting", chromedp.ByQuery),
		chromedp.SetValue(`input[name="duration_minutes"]`, "45", chromedp.ByQuery),
		chromedp.Click(`form[action="/dashboard/event-types"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("create event type: %v", err)
	}

	// Verify our card is present (there may be others from earlier tests).
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`//span[contains(@class,"card-title") and contains(.,"E2E Test Meeting")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("wait for card title: %v", err)
	}

	// --- Edit: click the Edit link belonging to the E2E Test Meeting card ---
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var card = Array.from(document.querySelectorAll('.card'))
				.find(c => c.querySelector('.card-title').textContent.includes('E2E Test Meeting'));
			card.querySelector('a[href*="/edit"]').click();
		`, nil),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="title"]`, "E2E Updated Meeting", chromedp.ByQuery),
		chromedp.SetValue(`textarea[name="confirmed_message"]`, "Thanks for booking!", chromedp.ByQuery),
		chromedp.Click(`form[action*="/dashboard/event-types/"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`//span[contains(@class,"card-title") and contains(.,"E2E Updated Meeting")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("edit event type: %v", err)
	}

	// --- Toggle (deactivate) the E2E Updated Meeting card ---
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var card = Array.from(document.querySelectorAll('.card'))
				.find(c => c.querySelector('.card-title').textContent.includes('E2E Updated Meeting'));
			card.querySelector('form[action*="/toggle"] button').click();
		`, nil),
		chromedp.WaitVisible(`//div[contains(@class,"card-meta") and contains(.,"Inactive")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("toggle event type: %v", err)
	}

	// --- Toggle back (activate) ---
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			var card = Array.from(document.querySelectorAll('.card'))
				.find(c => c.querySelector('.card-title').textContent.includes('E2E Updated Meeting'));
			card.querySelector('form[action*="/toggle"] button').click();
		`, nil),
		chromedp.WaitVisible(`//div[contains(@class,"card-meta") and contains(.,"Active")]`, chromedp.BySearch),
	); err != nil {
		t.Fatalf("re-toggle event type: %v", err)
	}
}

// TestDashboardBookings verifies the bookings list page and its status filter links.
func TestDashboardBookings(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var heading string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/bookings"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if heading != "Bookings" {
		t.Errorf("heading: got %q, want %q", heading, "Bookings")
	}

	// Filter links should navigate to the correct URLs.
	for _, status := range []string{"PENDING", "CONFIRMED", "REJECTED", "CANCELLED"} {
		var currentURL string
		if err := chromedp.Run(ctx,
			chromedp.Click(`a[href="/dashboard/bookings?status=`+status+`"]`, chromedp.ByQuery),
			waitForURLContains("status="+status, 10*time.Second),
			chromedp.Location(&currentURL),
		); err != nil {
			t.Errorf("filter %s: %v", status, err)
			continue
		}
		if !strings.Contains(currentURL, "status="+status) {
			t.Errorf("filter %s: URL %q does not contain status param", status, currentURL)
		}
		// Navigate back so the next filter click starts from the right page.
		if err := chromedp.Run(ctx,
			chromedp.Navigate(serverURL+"/dashboard/bookings"),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("navigate back for filter %s: %v", status, err)
		}
	}
}

// TestDashboardProfile verifies changing the display name and username slug.
func TestDashboardProfile(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Navigate and check heading.
	var heading string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/profile"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if heading != "Profile" {
		t.Errorf("heading: got %q, want %q", heading, "Profile")
	}

	// Change display name.
	if err := chromedp.Run(ctx,
		chromedp.SetValue(`input[name="name"]`, "E2E Admin", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("set name: %v", err)
	}
	if err := waitForFormSubmit(ctx, `form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("save name: %v", err)
	}
	var savedName string
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`input[name="name"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="name"]`, &savedName, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read back name: %v", err)
	}
	if savedName != "E2E Admin" {
		t.Errorf("saved name: got %q, want %q", savedName, "E2E Admin")
	}

	// Change username slug.
	if err := chromedp.Run(ctx,
		chromedp.SetValue(`input[name="username"]`, "e2e-admin", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("set slug: %v", err)
	}
	if err := waitForFormSubmit(ctx, `form[action="/dashboard/profile"] button[type="submit"]`); err != nil {
		t.Fatalf("save slug: %v", err)
	}
	var savedSlug string
	if err := chromedp.Run(ctx,
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="username"]`, &savedSlug, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read back slug: %v", err)
	}
	if savedSlug != "e2e-admin" {
		t.Errorf("saved slug: got %q, want %q", savedSlug, "e2e-admin")
	}

	// Public booking page should be accessible at the new slug.
	var bookingPageTitle string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/calendar/e2e-admin/"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &bookingPageTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("booking page at new slug: %v", err)
	}
	if !strings.Contains(bookingPageTitle, "E2E Admin") {
		t.Errorf("booking page title: got %q, want it to contain %q", bookingPageTitle, "E2E Admin")
	}

	// Validate that an empty name shows a server-side error.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/profile"),
		chromedp.WaitVisible(`input[name="name"]`, chromedp.ByQuery),
		// Use JS to blank the name and remove required in one atomic step,
		// avoiding a stale-node-ID race between SetValue and Evaluate.
		chromedp.Evaluate(`
			var n = document.querySelector('input[name="name"]');
			n.removeAttribute('required');
			n.value = '';
		`, nil),
		chromedp.Click(`form[action="/dashboard/profile"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.alert-error`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("empty name validation: %v", err)
	}
}
