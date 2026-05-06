//go:build e2e

package e2e_test

import (
	"fmt"
	"strings"
	"testing"
)

// TestCompleteBookingFlow exercises the end-to-end booking lifecycle:
//
//  1. Host sets up an event type and availability.
//  2. Guest navigates to the public booking page, picks a slot, and submits.
//  3. Host sees the PENDING booking in the dashboard and confirms it.
//  4. Booking is shown as CONFIRMED.
func TestCompleteBookingFlow(t *testing.T) {
	page := mustLogin(t)

	// --- 1. Setup: get username ---
	if _, err := page.Goto(serverURL + "/dashboard/profile"); err != nil {
		t.Fatalf("navigate to profile: %v", err)
	}
	username, err := page.InputValue(`input[name="username"]`)
	if err != nil {
		t.Fatalf("read username: %v", err)
	}
	if username == "" {
		t.Fatal("could not read current username")
	}

	// --- 2. Create event type ---
	const etSlug = "booking-flow-e2e"
	if _, err := page.Goto(serverURL + "/dashboard/event-types"); err != nil {
		t.Fatalf("navigate to event types: %v", err)
	}
	if err := page.Click("details summary"); err != nil {
		t.Fatalf("open create form: %v", err)
	}
	if err := page.Fill(`input[name="title"]`, "Booking Flow E2E"); err != nil {
		t.Fatalf("fill title: %v", err)
	}
	if err := page.Fill(`input[name="slug"]`, etSlug); err != nil {
		t.Fatalf("fill slug: %v", err)
	}
	if err := page.Fill(`input[name="duration_minutes"]`, "30"); err != nil {
		t.Fatalf("fill duration: %v", err)
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/event-types"] button[type="submit"]`); err != nil {
		t.Fatalf("submit create: %v", err)
	}

	// --- 3. Enable availability for all seven days to guarantee slots exist ---
	if _, err := page.Goto(serverURL + "/dashboard/availability"); err != nil {
		t.Fatalf("navigate to availability: %v", err)
	}
	for i := 0; i < 7; i++ {
		if err := page.Locator(fmt.Sprintf(`input[name="day_%d_active"]`, i)).Check(); err != nil {
			t.Fatalf("check day %d active: %v", i, err)
		}
	}
	if err := waitForFormSubmit(page, `form[action="/dashboard/availability"] button[type="submit"]`); err != nil {
		t.Fatalf("set availability: %v", err)
	}

	// --- 4. Navigate to the public booking page and find a date with slots ---
	pickerURL := serverURL + "/calendar/" + username + "/" + etSlug
	if _, err := page.Goto(pickerURL); err != nil {
		t.Fatalf("navigate to slot picker: %v", err)
	}
	if _, err := page.WaitForSelector(".date-btn"); err != nil {
		t.Fatalf("wait for date buttons: %v", err)
	}

	// Iterate through the seven date buttons until we find one with slots.
	var slotFound bool
	for i := 0; i < 7; i++ {
		if err := page.Locator(".date-btn").Nth(i).Click(); err != nil {
			t.Fatalf("click date button %d: %v", i, err)
		}
		page.WaitForTimeout(600)
		count, err := page.Locator(".slot-btn").Count()
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

	// --- 5. Select the first slot and fill in guest details ---
	if err := page.Locator(".slot-btn").First().Click(); err != nil {
		t.Fatalf("click first slot: %v", err)
	}
	if _, err := page.WaitForSelector("#booking-form"); err != nil {
		t.Fatalf("wait for booking form: %v", err)
	}
	if err := page.Fill(`input[name="guest_name"]`, "E2E Guest"); err != nil {
		t.Fatalf("fill guest name: %v", err)
	}
	if err := page.Fill(`input[name="guest_email"]`, "e2e-guest@example.com"); err != nil {
		t.Fatalf("fill guest email: %v", err)
	}

	// --- 6. Submit the booking ---
	if err := page.Click(`button[type="submit"]`); err != nil {
		t.Fatalf("submit booking: %v", err)
	}
	pageTitle, err := page.InnerText("h1")
	if err != nil {
		t.Fatalf("read page title: %v", err)
	}
	if !strings.Contains(pageTitle, "Booking request sent") {
		t.Errorf("submit page title: got %q, want it to contain %q", pageTitle, "Booking request sent")
	}

	// --- 7. Host: navigate to dashboard bookings and confirm ---
	if _, err := page.Goto(serverURL + "/dashboard/bookings"); err != nil {
		t.Fatalf("navigate to bookings: %v", err)
	}
	if _, err := page.WaitForSelector("table"); err != nil {
		t.Fatalf("wait for bookings table: %v", err)
	}

	// Verify the booking row is PENDING.
	pendingBadge, err := page.InnerText(".badge-pending")
	if err != nil {
		t.Fatalf("read pending badge: %v", err)
	}
	if !strings.EqualFold(pendingBadge, "PENDING") {
		t.Errorf("expected PENDING booking, got badge text %q", pendingBadge)
	}

	// Click the Confirm link — redirects back to the bookings page with a flash message.
	if err := page.Click(`a[href*="/confirm"]`); err != nil {
		t.Fatalf("click confirm: %v", err)
	}
	flashText, err := page.InnerText(".alert-success")
	if err != nil {
		t.Fatalf("read flash message: %v", err)
	}
	if !strings.Contains(flashText, "confirmed") {
		t.Errorf("flash message: got %q, want it to contain %q", flashText, "confirmed")
	}

	// --- 8. Verify booking is CONFIRMED in the dashboard ---
	if _, err := page.WaitForSelector("table"); err != nil {
		t.Fatalf("wait for table: %v", err)
	}

	confirmedBadge, err := page.InnerText(".badge-confirmed")
	if err != nil {
		t.Fatalf("read confirmed badge: %v", err)
	}
	if !strings.EqualFold(confirmedBadge, "CONFIRMED") {
		t.Errorf("expected CONFIRMED booking, got badge text %q", confirmedBadge)
	}
}
