//go:build e2e

package e2e_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestCompleteBookingFlow exercises the end-to-end booking lifecycle:
//
//  1. Host sets up an event type and availability.
//  2. Guest navigates to the public booking page, picks a slot, and submits.
//  3. Host sees the PENDING booking in the dashboard and confirms it.
//  4. Booking is shown as CONFIRMED.
func TestCompleteBookingFlow(t *testing.T) {
	ctx, cancel := mustLogin(t)
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	// --- 1. Setup: get username ---
	var username string
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/profile"),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.Value(`input[name="username"]`, &username, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read username: %v", err)
	}
	if username == "" {
		t.Fatal("could not read current username")
	}

	// --- 2. Create event type ---
	const etSlug = "booking-flow-e2e"
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/event-types"),
		chromedp.WaitVisible(`details summary`, chromedp.ByQuery),
		chromedp.Click(`details summary`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="title"]`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="title"]`, "Booking Flow E2E", chromedp.ByQuery),
		chromedp.SetValue(`input[name="slug"]`, etSlug, chromedp.ByQuery),
		chromedp.SetValue(`input[name="duration_minutes"]`, "30", chromedp.ByQuery),
		chromedp.Click(`form[action="/dashboard/event-types"] button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`.card-title`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("create event type: %v", err)
	}

	// --- 3. Enable availability for all seven days to guarantee slots exist ---
	enableAll := `
		for (var i = 0; i < 7; i++) {
			var cb = document.querySelector('input[name="day_' + i + '_active"]');
			if (cb) cb.checked = true;
		}
	`
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/availability"),
		chromedp.WaitVisible(`input[name="day_0_start"]`, chromedp.ByQuery),
		chromedp.Evaluate(enableAll, nil),
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`input[name="day_0_start"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	// --- 4. Navigate to the public booking page and find a date with slots ---
	pickerURL := serverURL + "/calendar/" + username + "/" + etSlug
	if err := chromedp.Run(ctx,
		chromedp.Navigate(pickerURL),
		chromedp.WaitVisible(`.date-btn`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate to slot picker: %v", err)
	}

	// Iterate through the seven date buttons until we find one with slots.
	var slotFound bool
	for i := 0; i < 7; i++ {
		var hasSlot bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`(function(){
				var btns = document.querySelectorAll('.date-btn');
				if (!btns[`+itoa(i)+`]) return false;
				btns[`+itoa(i)+`].click();
				return true;
			})()`, nil),
			chromedp.Sleep(600*time.Millisecond),
			chromedp.Evaluate(`document.querySelectorAll('.slot-btn').length > 0`, &hasSlot),
		); err != nil {
			t.Fatalf("click date button %d: %v", i, err)
		}
		if hasSlot {
			slotFound = true
			break
		}
	}
	if !slotFound {
		t.Fatal("no date with available slots found in the 7-day window")
	}

	// --- 5. Select the first slot and fill in guest details ---
	if err := chromedp.Run(ctx,
		// Click the first slot button (triggers selectSlot JS which shows the form).
		chromedp.Evaluate(`document.querySelectorAll('.slot-btn')[0].click()`, nil),
		chromedp.WaitVisible(`#booking-form`, chromedp.ByQuery),
		chromedp.SetValue(`input[name="guest_name"]`, "E2E Guest", chromedp.ByQuery),
		chromedp.SetValue(`input[name="guest_email"]`, "e2e-guest@example.com", chromedp.ByQuery),
	); err != nil {
		t.Fatalf("fill booking form: %v", err)
	}

	// --- 6. Submit the booking ---
	var pageTitle string
	if err := chromedp.Run(ctx,
		chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		chromedp.Text(`h1`, &pageTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("submit booking: %v", err)
	}
	if !strings.Contains(pageTitle, "Booking request sent") {
		t.Errorf("submit page title: got %q, want it to contain %q", pageTitle, "Booking request sent")
	}

	// --- 7. Host: navigate to dashboard bookings and confirm ---
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/bookings"),
		chromedp.WaitVisible(`table`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate to bookings: %v", err)
	}

	// Verify the booking row is PENDING.
	var pendingBadge string
	if err := chromedp.Run(ctx,
		chromedp.Text(`.badge-pending`, &pendingBadge, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read pending badge: %v", err)
	}
	if !strings.EqualFold(pendingBadge, "PENDING") {
		t.Errorf("expected PENDING booking, got badge text %q", pendingBadge)
	}

	// Click the Confirm link for the booking.
	if err := chromedp.Run(ctx,
		chromedp.Click(`a[href*="/confirm"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("click confirm: %v", err)
	}

	var confirmTitle string
	if err := chromedp.Run(ctx,
		chromedp.Text(`h1`, &confirmTitle, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read confirm page title: %v", err)
	}
	if !strings.Contains(confirmTitle, "confirmed") {
		t.Errorf("confirm page title: got %q, want it to contain %q", confirmTitle, "confirmed")
	}

	// --- 8. Verify booking is CONFIRMED in the dashboard ---
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/dashboard/bookings"),
		chromedp.WaitVisible(`table`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("navigate back to bookings: %v", err)
	}

	var confirmedBadge string
	if err := chromedp.Run(ctx,
		chromedp.Text(`.badge-confirmed`, &confirmedBadge, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("read confirmed badge: %v", err)
	}
	if !strings.EqualFold(confirmedBadge, "CONFIRMED") {
		t.Errorf("expected CONFIRMED booking, got badge text %q", confirmedBadge)
	}
}

// itoa converts a small int to its decimal string — avoids importing strconv.
func itoa(n int) string {
	return [10]string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9"}[n]
}
