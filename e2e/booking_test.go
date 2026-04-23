//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestSlotPickerActiveDateIndicator verifies that clicking a date in the slot picker
// updates the active-date highlight immediately (Bug: HTMX only swaps #slots, leaving
// the active class on the old button unless the onclick handler updates it).
func TestSlotPickerActiveDateIndicator(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// --- ensure a known event-type exists under the current user ---

	// Get current username from the profile page.
	var username string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/profile"),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="username"]`, &username, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read username: %v", err)
	}
	if username == "" {
		t.Fatal("could not determine current username")
	}

	// Create a test event type (details are inside a <details> element).
	const etSlug = "slot-picker-test"
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/event-types"),
		chromedp.WaitVisible(`details summary`, chromedp.ByQuery),
		chromedp.Click(`details summary`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="title"]`, "Slot Picker Test", chromedp.ByQuery),
		chromedp.SetValue(`input[name="slug"]`, etSlug, chromedp.ByQuery),
		chromedp.SetValue(`input[name="duration_minutes"]`, "30", chromedp.ByQuery),
		chromedp.Click(`form[action="/dashboard/event-types"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("create event type: %v", err)
	}

	// Enable Monday availability so there are slots to display.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/availability"),
		chromedp.WaitVisible(`input[name="day_1_start"]`, chromedp.ByQuery),
		chromedp.Evaluate(`document.querySelector('input[name="day_1_active"]').checked = true`, nil),
		chromedp.SetValue(`input[name="day_1_start"]`, "09:00", chromedp.ByQuery),
		chromedp.SetValue(`input[name="day_1_end"]`, "17:00", chromedp.ByQuery),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="day_1_start"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	// --- navigate to the slot picker ---
	pickerURL := serverURL + "/calendar/" + username + "/" + etSlug
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pickerURL),
		chromedp.WaitVisible(`.date-btn`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate to slot picker: %v", err)
	}

	// The first date button (today) should have the active class initially.
	var firstActive bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelector('.date-btn').classList.contains('active')`, &firstActive),
	); err != nil {
		t.Fatalf("check initial active: %v", err)
	}
	if !firstActive {
		t.Error("first (today) date button should start with the active class")
	}

	// Click the second date button (tomorrow).
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.date-btn')[1].click()`, nil),
		// Wait for the slot panel to update via HTMX.
		chromedp.Sleep(500*time.Millisecond),
	); err != nil {
		t.Fatalf("click second date: %v", err)
	}

	// After the click: the second button must be active, the first must not.
	var secondActive, firstStillActive bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.date-btn')[1].classList.contains('active')`, &secondActive),
		chromedp.Evaluate(`document.querySelectorAll('.date-btn')[0].classList.contains('active')`, &firstStillActive),
	); err != nil {
		t.Fatalf("check active after click: %v", err)
	}
	if !secondActive {
		t.Error("second date button should have active class after click")
	}
	if firstStillActive {
		t.Error("first (today) date button should NOT have active class after clicking the second date")
	}

	// Reload the page with the second date in the URL; verify the active class is
	// applied server-side (not just via JS).
	var secondDateURL string
	if err := chromedp.Run(ctx,
		chromedp.Location(&secondDateURL),
	); err != nil {
		t.Fatalf("get URL after click: %v", err)
	}
	if !strings.Contains(secondDateURL, "date=") {
		t.Fatalf("URL should contain date param after HTMX push, got %q", secondDateURL)
	}

	if err := chromedp.Run(ctx,
		chromedp.Navigate(secondDateURL),
		chromedp.WaitVisible(`.date-btn`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("reload with date param: %v", err)
	}

	var secondActiveAfterReload bool
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.querySelectorAll('.date-btn')[1].classList.contains('active')`, &secondActiveAfterReload),
	); err != nil {
		t.Fatalf("check active after reload: %v", err)
	}
	if !secondActiveAfterReload {
		t.Error("second date button should be active after full page reload with date param in URL")
	}
}
