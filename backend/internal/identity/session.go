package identity

import (
	"context"
	"crypto/sha256"
	"strings"
	"time"
)

type SessionStore interface {
	PrincipalBySessionHash(context.Context, [sha256.Size]byte, time.Time) (Principal, bool, error)
	RevokeSessionHash(context.Context, [sha256.Size]byte, time.Time, string) error
}

func (manager *SessionManager) Logout(ctx context.Context, token string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil
	}
	return manager.store.RevokeSessionHash(
		ctx,
		sha256.Sum256([]byte(token)),
		manager.now(),
		"user_logout",
	)
}

type SessionManager struct {
	store SessionStore
	now   func() time.Time
}

func NewSessionManager(store SessionStore) *SessionManager {
	return &SessionManager{
		store: store,
		now:   func() time.Time { return time.Now().UTC() },
	}
}

func (manager *SessionManager) Authenticate(ctx context.Context, token string) (Principal, bool, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return Principal{}, false, nil
	}
	return manager.store.PrincipalBySessionHash(ctx, sha256.Sum256([]byte(token)), manager.now())
}
