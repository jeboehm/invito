package handler

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"net/http"
	"time"

	"github.com/jeboehm/invito/internal/auth"
	"github.com/jeboehm/invito/internal/config"
	"github.com/jeboehm/invito/internal/db"
)

const oidcStateCookie = "invito_oidc_state"

type AuthHandler struct {
	cfg      *config.Config
	db       *sql.DB
	provider *auth.Provider
}

func NewAuthHandler(cfg *config.Config, database *sql.DB, provider *auth.Provider) *AuthHandler {
	return &AuthHandler{cfg: cfg, db: database, provider: provider}
}

func (h *AuthHandler) HandleLogin(w http.ResponseWriter, r *http.Request) {
	state := randomHex(16)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.provider.AuthCodeURL(state), http.StatusFound)
}

func (h *AuthHandler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil || stateCookie.Value != r.URL.Query().Get("state") {
		http.Error(w, "invalid state", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, MaxAge: -1, Path: "/"})

	claims, err := h.provider.Exchange(r.Context(), r.URL.Query().Get("code"))
	if err != nil {
		http.Error(w, fmt.Sprintf("authentication failed: %v", err), http.StatusInternalServerError)
		return
	}

	username := auth.SlugifyUsername(claims.PreferredUsername)
	if username == "" || username == "user" {
		username = auth.SlugifyUsername(claims.Sub[:min(8, len(claims.Sub))])
	}

	// Ensure username uniqueness
	base := username
	for i := 2; ; i++ {
		existing, err := db.GetUserByUsername(h.db, username)
		if err == sql.ErrNoRows {
			break
		}
		if err != nil {
			http.Error(w, "db error", http.StatusInternalServerError)
			return
		}
		// If the existing user has the same sub, that's fine — same person
		if existing.OIDCSub == claims.Sub {
			break
		}
		username = fmt.Sprintf("%s-%d", base, i)
	}

	user, err := db.UpsertUser(h.db, claims.Sub, claims.Email, claims.Name, username)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	sessionID, err := db.CreateSession(h.db, user.ID, 24*time.Hour)
	if err != nil {
		http.Error(w, "session error", http.StatusInternalServerError)
		return
	}

	auth.SetSessionCookie(w, sessionID, h.cfg.BaseURL)
	http.Redirect(w, r, "/dashboard", http.StatusFound)
}

func (h *AuthHandler) HandleLogout(w http.ResponseWriter, r *http.Request) {
	sessionID := auth.SessionIDFromRequest(r)
	if sessionID != "" {
		_ = db.DeleteSession(h.db, sessionID)
	}
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
