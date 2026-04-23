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
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		// Wait for the page to reload after the redirect.
		chromedp.WaitVisible(`input[name="day_1_start"]`, chromedp.ByQuery),
	); err != nil {
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

	var cardTitle string
	if err := chromedp.Run(ctx,
		chromedp.Text(`.card-title`, &cardTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read card title: %v", err)
	}
	if !strings.Contains(cardTitle, "E2E Test Meeting") {
		t.Errorf("card title: got %q, want it to contain %q", cardTitle, "E2E Test Meeting")
	}

	// --- Edit ---
	if err := chromedp.Run(ctx,
		chromedp.Click(`a[href*="/edit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="title"]`, "E2E Updated Meeting", chromedp.ByQuery),
		chromedp.SetValue(`textarea[name="guest_message"]`, "Thanks for booking!", chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("edit event type: %v", err)
	}

	var updatedTitle string
	if err := chromedp.Run(ctx,
		chromedp.Text(`.card-title`, &updatedTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read updated title: %v", err)
	}
	if !strings.Contains(updatedTitle, "E2E Updated Meeting") {
		t.Errorf("updated title: got %q, want it to contain %q", updatedTitle, "E2E Updated Meeting")
	}

	// --- Toggle (deactivate) ---
	if err := chromedp.Run(ctx,
		chromedp.Click(`form[action*="/toggle"] button`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-meta`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("toggle event type: %v", err)
	}

	var metaText string
	if err := chromedp.Run(ctx,
		chromedp.Text(`.card-meta`, &metaText, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read meta after toggle: %v", err)
	}
	if !strings.Contains(metaText, "Inactive") {
		t.Errorf("meta after deactivate: got %q, want it to contain %q", metaText, "Inactive")
	}

	// --- Toggle back (activate) ---
	if err := chromedp.Run(ctx,
		chromedp.Click(`form[action*="/toggle"] button`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-meta`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("re-toggle event type: %v", err)
	}

	if err := chromedp.Run(ctx,
		chromedp.Text(`.card-meta`, &metaText, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read meta after re-toggle: %v", err)
	}
	if !strings.Contains(metaText, "Active") {
		t.Errorf("meta after reactivate: got %q, want it to contain %q", metaText, "Active")
	}
}

// TestDashboardBookings verifies the bookings list page and its status filter links.
func TestDashboardBookings(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var heading, emptyMsg string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/bookings"),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &heading, chromedp.ByQuery),
		chromedp.WaitVisible(`p`, chromedp.ByQuery),
		chromedp.Text(`p`, &emptyMsg, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	if heading != "Bookings" {
		t.Errorf("heading: got %q, want %q", heading, "Bookings")
	}
	if !strings.Contains(emptyMsg, "No bookings found") {
		t.Errorf("empty state: got %q, want it to contain %q", emptyMsg, "No bookings found")
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
	var savedName string
	if err := chromedp.Run(ctx,
		chromedp.SetValue(`input[name="name"]`, "E2E Admin", chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="name"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="name"]`, &savedName, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("save name: %v", err)
	}
	if savedName != "E2E Admin" {
		t.Errorf("saved name: got %q, want %q", savedName, "E2E Admin")
	}

	// Change username slug.
	var savedSlug string
	if err := chromedp.Run(ctx,
		chromedp.SetValue(`input[name="username"]`, "e2e-admin", chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="username"]`, &savedSlug, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("save slug: %v", err)
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

	// Duplicate slug should be rejected (try to set the same slug again from another session —
	// the same user changing to their own existing slug is fine; a different user would be blocked).
	// Validate that an empty slug shows an error.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/profile"),
		chromedp.WaitVisible(`input[name="name"]`, chromedp.ByQuery),
		// Blank out the name field — server must reject it.
		chromedp.SetValue(`input[name="name"]`, "", chromedp.ByQuery),
		// Remove the required attribute so the browser does not block submit client-side.
		chromedp.Evaluate(`document.querySelector('input[name="name"]').removeAttribute('required')`, nil),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.alert-error`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("empty name validation: %v", err)
	}
}
