package session

import (
	"context"
	"errors"
	"job-applications/internal/db"

	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const (
	SessionCookieName = "job-tracker-session"
	SessionIDLength   = 32
	DefaultExpiration = 24 * time.Hour
)

var (
	ErrSessionNotFound = errors.New("session not found")
	ErrSessionExpired  = errors.New("session expired")
)

type Session struct {
	ID           string
	UserID       int32
	UserName     string
	IpAddress    string
	UserAgent    string
	CreatedAt    pgtype.Timestamp
	LastActivity pgtype.Timestamp
	ExpiresAt    pgtype.Timestamp
}

type SessionStore interface {
	GetSessionByID(ctx context.Context, id string) (db.Session, error)
	UpdateSessionActivity(ctx context.Context, arg db.UpdateSessionActivityParams) error
	DeleteSession(ctx context.Context, id string) error
}

type Manager struct {
	store SessionStore
}

func NewManager(store SessionStore) *Manager {
	return &Manager{store: store}
}

func (manager *Manager) Get(ctx context.Context, sessionID string) (*Session, error) {
	dbSession, err := manager.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrSessionNotFound
		}
		return nil, err
	}

	return &Session{
		ID:           dbSession.ID,
		UserID:       dbSession.UserID,
		IpAddress:    dbSession.IpAddress,
		UserAgent:    dbSession.UserAgent,
		CreatedAt:    dbSession.CreatedAt,
		LastActivity: dbSession.LastActivity,
		ExpiresAt:    dbSession.ExpiresAt,
	}, nil
}

func (manager *Manager) UpdateActivity(ctx context.Context, sessionID string) error {
	return manager.store.UpdateSessionActivity(ctx, db.UpdateSessionActivityParams{
		LastActivity: pgtype.Timestamp{Time: time.Now(), Valid: true},
		ExpiresAt:    pgtype.Timestamp{Time: time.Now().Add(DefaultExpiration), Valid: true},
		ID:           sessionID,
	})
}

func (manager *Manager) Delete(ctx context.Context, sessionID string) error {
	return manager.store.DeleteSession(ctx, sessionID)
}

func SetCookie(writer http.ResponseWriter, request *http.Request, sessionID string, expiration time.Duration) {
	cookie := getCookie(request, sessionID, int(expiration.Seconds()))
	http.SetCookie(writer, cookie)
}

func ClearCookie(writer http.ResponseWriter, request *http.Request) {
	cookie := getCookie(request, "", -1)
	http.SetCookie(writer, cookie)
}

func GetSessionIDFromRequest(request *http.Request) (string, error) {
	cookie, err := request.Cookie(SessionCookieName)
	if err != nil {
		return "", err
	}
	return cookie.Value, nil
}

// #nosec G124
func getCookie(request *http.Request, value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	}
}
