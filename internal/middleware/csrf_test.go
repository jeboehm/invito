package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jeboehm/invito/internal/middleware"
)

// ok is a trivial handler that writes 200.
var ok = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

var csrf = middleware.CSRF(false)

// getToken performs a GET through the CSRF middleware and returns the cookie token.
func getToken(t *testing.T) string {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	csrf(ok).ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "invito_csrf" {
			return c.Value
		}
	}
	t.Fatal("no invito_csrf cookie in GET response")
	return ""
}

func TestCSRF_GetSetsCookie(t *testing.T) {
	token := getToken(t)
	if token == "" {
		t.Fatal("expected non-empty CSRF token cookie")
	}
}

func TestCSRF_GetDoesNotValidate(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	csrf(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET without token: got %d, want 200", rec.Code)
	}
}

func TestCSRF_TokenInContextOnFirstVisit(t *testing.T) {
	var gotToken string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = middleware.CSRFToken(r)
		w.WriteHeader(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	csrf(handler).ServeHTTP(httptest.NewRecorder(), req)
	if gotToken == "" {
		t.Fatal("CSRFToken(r) returned empty on first visit — context propagation broken")
	}
}

func TestCSRF_PostWithFormToken(t *testing.T) {
	token := getToken(t)

	form := url.Values{"csrf_token": {token}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "invito_csrf", Value: token})

	rec := httptest.NewRecorder()
	csrf(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST with valid form token: got %d, want 200", rec.Code)
	}
}

func TestCSRF_PostWithHeaderToken(t *testing.T) {
	token := getToken(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-CSRF-Token", token)
	req.AddCookie(&http.Cookie{Name: "invito_csrf", Value: token})

	rec := httptest.NewRecorder()
	csrf(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST with valid header token: got %d, want 200", rec.Code)
	}
}

func TestCSRF_PostWithWrongToken(t *testing.T) {
	token := getToken(t)

	form := url.Values{"csrf_token": {"wrong-token"}}
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "invito_csrf", Value: token})

	rec := httptest.NewRecorder()
	csrf(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST with wrong token: got %d, want 403", rec.Code)
	}
}

func TestCSRF_PostWithMissingToken(t *testing.T) {
	token := getToken(t)

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.AddCookie(&http.Cookie{Name: "invito_csrf", Value: token})

	rec := httptest.NewRecorder()
	csrf(ok).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("POST with missing token: got %d, want 403", rec.Code)
	}
}
