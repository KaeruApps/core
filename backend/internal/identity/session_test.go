package identity

import (
	"context"
	"crypto/sha256"
	"testing"
	"time"
)

type stubSessionStore struct {
	hash             [sha256.Size]byte
	revokedHash      [sha256.Size]byte
	revocationReason string
}

func (store *stubSessionStore) PrincipalBySessionHash(_ context.Context, hash [sha256.Size]byte, _ time.Time) (Principal, bool, error) {
	store.hash = hash
	return Principal{Name: "Admin"}, true, nil
}

func (store *stubSessionStore) RevokeSessionHash(_ context.Context, hash [sha256.Size]byte, _ time.Time, reason string) error {
	store.revokedHash = hash
	store.revocationReason = reason
	return nil
}

func TestSessionManagerHashesToken(t *testing.T) {
	store := &stubSessionStore{}
	manager := NewSessionManager(store)

	_, authenticated, err := manager.Authenticate(context.Background(), "secret-session-token")
	if err != nil || !authenticated {
		t.Fatalf("Authenticate() authenticated = %v, error = %v", authenticated, err)
	}
	if store.hash != sha256.Sum256([]byte("secret-session-token")) {
		t.Fatal("session store did not receive the token hash")
	}
}

func TestSessionManagerRevokesHashedTokenOnLogout(t *testing.T) {
	store := &stubSessionStore{}
	manager := NewSessionManager(store)

	if err := manager.Logout(context.Background(), "secret-session-token"); err != nil {
		t.Fatalf("Logout() error = %v", err)
	}
	if store.revokedHash != sha256.Sum256([]byte("secret-session-token")) {
		t.Fatal("session store did not receive the token hash")
	}
	if store.revocationReason != "user_logout" {
		t.Fatalf("revocation reason = %q, want user_logout", store.revocationReason)
	}
}
