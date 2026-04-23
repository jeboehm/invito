//go:build e2e

package e2e_test

import (
	"context"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// mustLogin starts a new Chrome context, performs the OIDC login flow with the
// dev Dex credentials, and returns the logged-in context. The caller must call
// cancel when done.
func mustLogin(t *testing.T) (context.Context, context.CancelFunc) {
	t.Helper()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))

	loginCtx, loginCancel := context.WithTimeout(ctx, 30*time.Second)
	defer loginCancel()

	if err := chromedp.Run(loginCtx,
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
