//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// TestLogin verifies the full OIDC login flow:
//  1. Navigate to /auth/login → redirect to Dex
//  2. Fill in dev credentials and submit
//  3. Dex redirects back to /auth/callback → /dashboard
//  4. Assert the session is established (dashboard accessible)
func TestLogin(t *testing.T) {
	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()
	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var currentURL string
	err := chromedp.Run(ctx,
		// Start the OIDC flow.
		chromedp.Navigate(serverURL+"/auth/login"),

		// Dex redirects us to its login form. Wait for the email field.
		chromedp.WaitVisible(`#login`, chromedp.ByQuery),

		// Fill in credentials. SetValue bypasses browser autofill extensions
		// (e.g. 1Password) that interfere with simulated key events.
		chromedp.SetValue(`#login`, "admin@example.com", chromedp.ByQuery),
		chromedp.SetValue(`#password`, "password", chromedp.ByQuery),

		// Submit — Dex will redirect back to /auth/callback then /dashboard.
		chromedp.Click(`#submit-login`, chromedp.ByQuery),

		// Wait until the browser lands on /dashboard.
		waitForURLContains("/dashboard", 15*time.Second),
		chromedp.Location(&currentURL),
	)
	if err != nil {
		t.Fatalf("login flow: %v", err)
	}
	if !strings.Contains(currentURL, "/dashboard") {
		t.Errorf("expected URL to contain /dashboard, got %s", currentURL)
	}
}

// waitForURLContains polls the browser's current URL until it contains substr
// or timeout elapses.
func waitForURLContains(substr string, timeout time.Duration) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			var loc string
			if err := chromedp.Location(&loc).Do(ctx); err != nil {
				return err
			}
			if strings.Contains(loc, substr) {
				return nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		var loc string
		chromedp.Location(&loc).Do(ctx) //nolint:errcheck
		return fmt.Errorf("URL %q does not contain %q after %s", loc, substr, timeout)
	})
}
