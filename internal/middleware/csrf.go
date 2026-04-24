package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net/http"
)

const csrfCookieName = "invito_csrf"

type csrfCtxKey struct{}

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := csrfToken(w, r)
		r = r.WithContext(context.WithValue(r.Context(), csrfCtxKey{}, token))

		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
			submitted := r.FormValue("csrf_token")
			if submitted == "" {
				submitted = r.Header.Get("X-CSRF-Token")
			}
			if submitted != token {
				http.Error(w, "invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

func csrfToken(w http.ResponseWriter, r *http.Request) string {
	if c, err := r.Cookie(csrfCookieName); err == nil && c.Value != "" {
		return c.Value
	}
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)
	http.SetCookie(w, &http.Cookie{
		Name:     csrfCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})
	return token
}

func CSRFToken(r *http.Request) string {
	if t, ok := r.Context().Value(csrfCtxKey{}).(string); ok && t != "" {
		return t
	}
	if c, err := r.Cookie(csrfCookieName); err == nil {
		return c.Value
	}
	return ""
}

func CSRFField(r *http.Request) template.HTML {
	return template.HTML(`<input type="hidden" name="csrf_token" value="` + CSRFToken(r) + `">`)
}
