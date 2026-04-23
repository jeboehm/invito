package auth

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/jboehm/invito/internal/db"
)

type contextKey struct{}

func RequireAuth(database *sql.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessionID := SessionIDFromRequest(r)
			if sessionID == "" {
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			session, err := db.GetSession(database, sessionID)
			if err != nil {
				ClearSessionCookie(w)
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			user, err := db.GetUserByID(database, session.UserID)
			if err != nil {
				ClearSessionCookie(w)
				http.Redirect(w, r, "/auth/login", http.StatusFound)
				return
			}

			ctx := context.WithValue(r.Context(), contextKey{}, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserFromContext(ctx context.Context) *db.User {
	u, _ := ctx.Value(contextKey{}).(*db.User)
	return u
}
