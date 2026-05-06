package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/jeboehm/invito/internal/db"
)

type contextKey struct{}

// resolveUser reads the session cookie, validates it, and returns a request
// with the user injected into the context. Returns (r, false) if no valid
// session exists.
func resolveUser(database *sql.DB, r *http.Request) (*http.Request, bool) {
	sessionID := SessionIDFromRequest(r)
	if sessionID == "" {
		return r, false
	}
	session, err := db.GetSession(database, sessionID)
	if err != nil {
		return r, false
	}
	user, err := db.GetUserByID(database, session.UserID)
	if err != nil {
		return r, false
	}
	return r.WithContext(context.WithValue(r.Context(), contextKey{}, user)), true
}

func RequireAuth(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, ok := resolveUser(database, r)
			if !ok {
				ClearSessionCookie(w)
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func OptionalAuth(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r, _ = resolveUser(database, r)
			next.ServeHTTP(w, r)
		})
	}
}

func UserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(contextKey{}).(*db.User)
	return u
}
