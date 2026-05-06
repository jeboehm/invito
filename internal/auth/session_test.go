package auth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jeboehm/invito/internal/auth"
)

func TestSetSessionCookie_HTTPS(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.SetSessionCookie(rec, "sess123", "https://example.com")
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies set")
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName {
		t.Errorf("cookie name: got %q, want %q", c.Name, auth.SessionCookieName)
	}
	if c.Value != "sess123" {
		t.Errorf("cookie value: got %q, want sess123", c.Value)
	}
	if !c.Secure {
		t.Error("expected Secure=true for https base URL")
	}
	if !c.HttpOnly {
		t.Error("expected HttpOnly=true")
	}
}

func TestSetSessionCookie_HTTP(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.SetSessionCookie(rec, "sess456", "http://localhost:8080")
	c := rec.Result().Cookies()[0]
	if c.Secure {
		t.Error("expected Secure=false for http base URL")
	}
}

func TestClearSessionCookie(t *testing.T) {
	rec := httptest.NewRecorder()
	auth.ClearSessionCookie(rec)
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("no cookies set by ClearSessionCookie")
	}
	c := cookies[0]
	if c.Name != auth.SessionCookieName {
		t.Errorf("cookie name: got %q, want %q", c.Name, auth.SessionCookieName)
	}
	if c.MaxAge != -1 {
		t.Errorf("MaxAge: got %d, want -1", c.MaxAge)
	}
}

func TestSessionIDFromRequest_WithCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: "my-session-id"})
	got := auth.SessionIDFromRequest(req)
	if got != "my-session-id" {
		t.Errorf("got %q, want my-session-id", got)
	}
}

func TestSessionIDFromRequest_NoCookie(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	got := auth.SessionIDFromRequest(req)
	if got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}
