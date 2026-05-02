package auth_test

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jeboehm/invito/internal/auth"
	"github.com/jeboehm/invito/internal/db"
)

func openAuthTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestRequireAuth_NoSession_Redirects(t *testing.T) {
	database := openAuthTestDB(t)
	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	auth.RequireAuth(database)(okHandler).ServeHTTP(rec, req)
	if rec.Code != http.StatusFound {
		t.Fatalf("got %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/auth/login" {
		t.Errorf("redirect location: got %q, want /auth/login", loc)
	}
}

func TestRequireAuth_ValidSession_PassesThrough(t *testing.T) {
	database := openAuthTestDB(t)
	user, err := db.UpsertUser(database, "sub1", "user@example.com", "Test User", "testuser")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	sessionID, err := db.CreateSession(database, user.ID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	var got *db.User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	auth.RequireAuth(database)(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if got == nil {
		t.Fatal("user not in context")
	}
	if got.ID != user.ID {
		t.Errorf("user ID: got %d, want %d", got.ID, user.ID)
	}
}

func TestOptionalAuth_NoSession(t *testing.T) {
	database := openAuthTestDB(t)
	var got *db.User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	rec := httptest.NewRecorder()
	auth.OptionalAuth(database)(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if got != nil {
		t.Error("expected nil user in context without session")
	}
}

func TestOptionalAuth_ValidSession(t *testing.T) {
	database := openAuthTestDB(t)
	user, err := db.UpsertUser(database, "sub2", "user2@example.com", "User Two", "usertwo")
	if err != nil {
		t.Fatalf("upsert user: %v", err)
	}
	sessionID, err := db.CreateSession(database, user.ID, time.Hour)
	if err != nil {
		t.Fatalf("create session: %v", err)
	}

	var got *db.User
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = auth.UserFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/public", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: sessionID})
	rec := httptest.NewRecorder()
	auth.OptionalAuth(database)(handler).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}
	if got == nil {
		t.Fatal("expected user in context with valid session")
	}
	if got.ID != user.ID {
		t.Errorf("user ID: got %d, want %d", got.ID, user.ID)
	}
}

func TestUserFromContext_NilWhenMissing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if got := auth.UserFromContext(req.Context()); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}
