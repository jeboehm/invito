//go:build e2e

package e2e_test

import (
	"testing"

	playwright "github.com/playwright-community/playwright-go"
)

// newPage creates an isolated browser context per test so cookies and storage
// never leak between tests.
func newPage(t *testing.T) playwright.Page {
	t.Helper()
	bctx, err := browser.NewContext()
	if err != nil {
		t.Fatalf("new browser context: %v", err)
	}
	t.Cleanup(func() { bctx.Close() })
	page, err := bctx.NewPage()
	if err != nil {
		t.Fatalf("new page: %v", err)
	}
	return page
}

// waitForFormSubmit clicks a submit button and waits for the POST→redirect→GET
// sequence. ExpectNavigation sets up the listener before the click so fast
// local redirects can't race past the wait.
func waitForFormSubmit(page playwright.Page, submitSelector string) error {
	_, err := page.ExpectNavigation(func() error {
		return page.Click(submitSelector)
	})
	return err
}

// mustLogin performs the OIDC login flow and returns a page already on /dashboard.
func mustLogin(t *testing.T) playwright.Page {
	t.Helper()
	page := newPage(t)
	if _, err := page.Goto(serverURL + "/auth/login"); err != nil {
		t.Fatalf("navigate to login: %v", err)
	}
	if err := page.Fill("#login", "admin@example.com"); err != nil {
		t.Fatalf("fill email: %v", err)
	}
	if err := page.Fill("#password", "password"); err != nil {
		t.Fatalf("fill password: %v", err)
	}
	if err := page.Click("#submit-login"); err != nil {
		t.Fatalf("click submit: %v", err)
	}
	if err := page.WaitForURL("**/dashboard**"); err != nil {
		t.Fatalf("wait for dashboard: %v", err)
	}
	return page
}
