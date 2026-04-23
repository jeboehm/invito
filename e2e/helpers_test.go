//go:build e2e

package e2e_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// waitForFormSubmit clicks a form submit button and waits for the resulting
// POST→redirect→GET sequence to complete. Use this instead of Click+WaitVisible
// when the same page is shown before and after the form submit (same element visible
// throughout), which would cause WaitVisible to return before the POST completes.
func waitForFormSubmit(ctx context.Context, submitSelector string) error {
	// Inject a reload marker; a new page load removes it.
	if err := chromedp.Run(ctx,
		chromedp.Evaluate(`document.body.dataset.reloadPending = '1'`, nil),
		chromedp.Click(submitSelector, chromedp.ByQuery),
	); err != nil {
		return err
	}
	// Poll until the marker is gone (page has been replaced by the redirect GET).
	// Evaluation errors during the navigation (context destroyed, ERR_ABORTED)
	// are transient — swallow them and keep polling.
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var present bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.body.dataset.reloadPending === '1'`, &present),
		); err == nil && !present {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("form submit: page did not reload within 10s")
}

// mustLogin starts a new Chrome context, performs the OIDC login flow with the
// dev Dex credentials, and returns the logged-in context. The caller must call
// cancel when done.
func mustLogin(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	// Use ctx directly for login — creating a derived context and cancelling it
	// before returning disrupts chromedp's internal CDP state and makes the
	// returned ctx unusable.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(serverURL+"/auth/login"),
		chromedp.WaitVisible(`#login`, chromedp.ByQuery),
		chromedp.SetValue(`#login`, "admin@example.com", chromedp.ByQuery),
		chromedp.SetValue(`#password`, "password", chromedp.ByQuery),
		chromedp.Click(`#submit-login`, chromedp.ByQuery),
		waitForURLContains("/dashboard", 15*time.Second),
	); err != nil {
		cancel()
		t.Fatalf("mustLogin: %v", err)
	}

	return ctx, cancel
}
