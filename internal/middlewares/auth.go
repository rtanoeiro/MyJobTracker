package middlewares

import (
	"context"
	"job-applications/internal/session"

	"log/slog"
	"net/http"
)

type contextKey string

const (
	SessionContextKey contextKey = "session"
	UserIDContextKey  contextKey = "user_id"
)

func RequireAuth(sessionMgr session.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Failed to get session from cookie, redirect user back to login page
			sessionID, err := session.GetSessionIDFromRequest(r)
			if err != nil {
				slog.Debug("no session cookie found", "error", err)
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			// Failed to get session, redirect user back to login page
			sess, err := sessionMgr.Get(r.Context(), sessionID)
			if err != nil {
				slog.Debug("invalid or expired session", "error", err, "session_id", sessionID)
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}

			// If any action is performed while logged in, we update the cookie extending the session for another 12 hours
			if err := sessionMgr.UpdateActivity(r.Context(), sessionID); err != nil {
				slog.Warn("failed to update session activity", "error", err, "session_id", sessionID)
			}

			ctx := context.WithValue(r.Context(), SessionContextKey, sess)
			ctx = context.WithValue(ctx, UserIDContextKey, sess.UserID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSessionFromContext(ctx context.Context) (*session.Session, bool) {
	sess, ok := ctx.Value(SessionContextKey).(*session.Session)
	return sess, ok
}

func GetUserIDFromContext(ctx context.Context) (int32, bool) {
	userID, ok := ctx.Value(UserIDContextKey).(int32)
	return userID, ok
}
