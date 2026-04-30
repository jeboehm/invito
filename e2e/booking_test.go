//go:build e2e

package e2e_test

import (
	"strings"
	"testing"
)

// TestSlotPickerActiveDateIndicator verifies that clicking a date in the slot picker
// updates the active-date highlight immediately (Bug: HTMX only swaps #slots, leaving
// the active class on the old button unless the onclick handler updates it).
func TestSlotPickerActiveDateIndicator(t *testing.T) {
	page := mustLogin(t)

	// Get current username from the profile page.
	if _, err := page.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := page.InputValue(`input[name="username"]`)
	if err != nil {
		t.Fatalf("read username: %v", err)
	}
	if username == "" {
		t.Fatal("could not determine current username")
	}

	// Create a test event type (details are inside a <details> element).
	const etSlug = "slot-picker-test"
	if _, err := page.Goto(serverURL + "/dashboard/event-types"); err != nil {
		t.Fatalf("navigate to event types: %v", err)
	}
	if err := page.Click("details summary"); err != nil {
		t.Fatalf("open create form: %v", err)
	}
	if err := page.Fill(`input[name="title"]`, "Slot Picker Test"); err != nil {
		t.Fatalf("fill title: %v", err)
	}
	if err := page.Fill(`input[name="slug"]`, etSlug); err != nil {
		t.Fatalf("fill slug: %v", err)
	}
	if err := page.Fill(`input[name="duration_minutes"]`, "30"); err != nil {
		t.Fatalf("fill duration: %v", err)
	}
	// Use waitForFormSubmit so the POST+redirect completes before proceeding.
	if err := waitForFormSubmit(page, `form[action="/dashboard/event-types"] button[type="submit"]`); err != nil {
		t.Fatalf("create event type: %v", err)
	}

	// Enable Monday availability so there are slots to display.
	if _, err := page.Goto(serverURL + "/dashboard/availability"); err != nil {
		t.Fatalf("navigate to availability: %v", err)
	}
	if err := page.Locator(`input[name="day_1_active"]`).Check(); err != nil {
		t.Fatalf("check Monday active: %v", err)
	}
	if err := page.Fill(`input[name="day_1_start"]`, "09:00"); err != nil {
		t.Fatalf("fill start: %v", err)
	}
	if err := page.Fill(`input[name="day_1_end"]`, "17:00"); err != nil {
		t.Fatalf("fill end: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/availability"] button[type="submit"]`); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	// Navigate to the slot picker.
	pickerURL := serverURL + "/calendar/" + username + "/" + etSlug
	if _, err := page.Goto(pickerURL); err != nil {
		t.Fatalf("navigate to slot picker: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}

	// The first date button (today) should have the active class initially.
	firstActiveRaw, err := page.Locator(".date-btn").First().Evaluate("el => el.classList.contains('active')", nil)
	if err != nil {
		t.Fatalf("check initial active: %v", err)
	}
	if active, ok := firstActiveRaw.(bool); !ok || !active {
		t.Error("first (today) date button should start with the active class")
	}

	// Click the second date button (tomorrow).
	if err := page.Locator(".date-btn").Nth(1).Click(); err != nil {
		t.Fatalf("click second date: %v", err)
	}
	// Wait for the HTMX slot panel swap to complete.
	page.WaitForTimeout(500)

	// After the click: the second button must be active, the first must not.
	secondActiveRaw, err := page.Locator(".date-btn").Nth(1).Evaluate("el => el.classList.contains('active')", nil)
	if err != nil {
		t.Fatalf("check second active: %v", err)
	}
	firstStillActiveRaw, err := page.Locator(".date-btn").Nth(0).Evaluate("el => el.classList.contains('active')", nil)
	if err != nil {
		t.Fatalf("check first still active: %v", err)
	}
	if active, ok := secondActiveRaw.(bool); !ok || !active {
		t.Error("second date button should have active class after click")
	}
	if active, ok := firstStillActiveRaw.(bool); ok && active {
		t.Error("first (today) date button should NOT have active class after clicking the second date")
	}

	// Reload the page with the second date in the URL; verify the active class is
	// applied server-side (not just via JS).
	currentURL := page.URL()
	if !strings.Contains(currentURL, "date=") {
		t.Fatalf("URL should contain date param after HTMX push, got %q", currentURL)
	}
	if _, err := page.Goto(currentURL); err != nil {
		t.Fatalf("reload with date param: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons after reload: %v", err)
	}

	secondActiveAfterReloadRaw, err := page.Locator(".date-btn").Nth(1).Evaluate("el => el.classList.contains('active')", nil)
	if err != nil {
		t.Fatalf("check active after reload: %v", err)
	}
	if active, ok := secondActiveAfterReloadRaw.(bool); !ok || !active {
		t.Error("second date button should be active after full page reload with date param in URL")
	}
}
