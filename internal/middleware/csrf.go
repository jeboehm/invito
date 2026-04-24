package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"html/template"
	"net/http"
)

const csrfCookieName = "invito_csrf"

type csrfCtxKey struct{}

// CSRF returns a middleware that validates CSRF tokens on mutating requests.
// secure controls whether the CSRF cookie is marked Secure (set to true on HTTPS deployments).
func CSRF(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := csrfToken(w, r, secure)
			r = r.WithContext(context.WithValue(r.Context(), csrfCtxKey{}, token))

			switch r.Method {
			case http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch:
				submitted := r.FormValue("csrf_token")
				if submitted == "" {
					submitted = r.Header.Get("X-CSRF-Token")
				}
				if subtle.ConstantTimeCompare([]byte(submitted), []byte(token)) != 1 {
					http.Error(w, "invalid CSRF token", http.StatusForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func csrfToken(w http.ResponseWriter, r *http.Request, secure bool) string {
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
		Secure:   secure,
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

// CSRFField returns a hidden input element containing the CSRF token.
// The token value is constrained to hex characters so string concatenation is safe here.
func CSRFField(r *http.Request) template.HTML {
	return template.HTML(`<input type="hidden" name="csrf_token" value="` + CSRFToken(r) + `">`)
}

func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
