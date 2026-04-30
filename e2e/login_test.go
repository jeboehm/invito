//go:build e2e

package e2e_test

import (
	"strings"
	"testing"
)

// TestLogin verifies the full OIDC login flow ends on /dashboard.
func TestLogin(t *testing.T) {
	page := mustLogin(t)
	if !strings.Contains(page.URL(), "/dashboard") {
		t.Errorf("expected URL to contain /dashboard, got %s", page.URL())
	}
}
