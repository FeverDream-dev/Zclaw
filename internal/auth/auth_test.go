package auth

import (
	"context"
	"testing"
	"time"
)

type MockStore struct {
	keys  map[string]*APIKey
	users map[UserID]*User
}

func NewMockStore() *MockStore {
	return &MockStore{keys: make(map[string]*APIKey), users: make(map[UserID]*User)}
}

func (m *MockStore) GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error) {
	if k, ok := m.keys[hash]; ok {
		return k, nil
	}
	return nil, nil
}

func (m *MockStore) GetUserByID(ctx context.Context, id UserID) (*User, error) {
	if u, ok := m.users[id]; ok {
		return u, nil
	}
	return nil, nil
}

func (m *MockStore) UpdateAPIKeyLastUsed(ctx context.Context, id APIKeyID) error {
	for _, k := range m.keys {
		if k.ID == id {
			now := time.Now().UTC()
			k.LastUsedAt = &now
			return nil
		}
	}
	return nil
}

func TestGenerateAPIKey(t *testing.T) {
	plain, hash, err := GenerateAPIKey("prefix-")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain == "" || hash == "" {
		t.Fatalf("expected non-empty key and hash")
	}
	if len(plain) != len("prefix-")+32 {
		t.Fatalf("unexpected plain key length: %d", len(plain))
	}
	if len(hash) != 64 {
		t.Fatalf("unexpected hash length: %d", len(hash))
	}
}

func TestHashAndVerifyAPIKey(t *testing.T) {
	plain := "testkey-0123456789abcdef0123456789abcdef"
	hash := HashAPIKey(plain)
	if !VerifyAPIKey(plain, hash) {
		t.Fatalf("expected API key verification to succeed for correct key")
	}
	if VerifyAPIKey(plain+"x", hash) {
		t.Fatalf("verification should fail for incorrect key")
	}
}

func TestCreateAndVerifySessionToken(t *testing.T) {
	sess := Session{
		UserID:    UserID("user1"),
		TenantID:  TenantID("tenant1"),
		Role:      RoleAdmin,
		ExpiresAt: time.Now().Add(1 * time.Hour),
	}
	token, err := CreateSessionToken(sess, "secret123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parsed, err := VerifySessionToken(token, "secret123")
	if err != nil {
		t.Fatalf("unexpected error verifying token: %v", err)
	}
	if string(parsed.UserID) != string(sess.UserID) || string(parsed.TenantID) != string(sess.TenantID) || parsed.Role != sess.Role {
		t.Fatalf("token payload mismatch: %+v != %+v", parsed, sess)
	}
}

func TestVerifySessionToken_Expired(t *testing.T) {
	sess := Session{UserID: UserID("u"), TenantID: TenantID("t"), Role: RoleViewer, ExpiresAt: time.Now().Add(-time.Hour)}
	token, _ := CreateSessionToken(sess, "secret-")
	if _, err := VerifySessionToken(token, "secret-"); err == nil {
		t.Fatalf("expected error for expired token")
	}
}

func TestVerifySessionToken_WrongSecret(t *testing.T) {
	sess := Session{UserID: UserID("u"), TenantID: TenantID("t"), Role: RoleViewer, ExpiresAt: time.Now().Add(1 * time.Hour)}
	token, _ := CreateSessionToken(sess, "secret-A")
	if _, err := VerifySessionToken(token, "secret-B"); err == nil {
		t.Fatalf("expected error when verifying with wrong secret")
	}
}

func TestHasPermission(t *testing.T) {
	cases := []struct {
		role       Role
		permChecks []Permission
		expected   []bool
	}{
		{RoleAdmin, []Permission{PermAgentCreate, PermTaskEnqueue, PermDashboard}, []bool{true, true, true}},
		{RoleOperator, []Permission{PermAgentDelete, PermTaskRead, PermDashboard}, []bool{true, true, true}},
		{RoleViewer, []Permission{PermAgentRead, PermToolExecute}, []bool{true, false}},
		{RoleAgent, []Permission{PermTaskEnqueue, PermToolExecute}, []bool{true, true}},
		{RoleAnonymous, []Permission{PermAgentRead}, []bool{false}},
	}
	for _, c := range cases {
		for i, p := range c.permChecks {
			ok := HasPermission(c.role, p)
			if ok != c.expected[i] {
				t.Fatalf("HasPermission failed for role=%q perm=%q: got %v want %v", c.role, p, ok, c.expected[i])
			}
		}
	}
}

func TestHasAnyPermission(t *testing.T) {
	if !HasAnyPermission(RoleAdmin, PermAgentCreate, PermAPIKeyManage) {
		t.Fatalf("expected Admin to have some permissions in the set")
	}
	if HasAnyPermission(RoleAnonymous, PermAgentRead, PermTaskRead) {
		t.Fatalf("expected Anonymous to have no permissions in the set")
	}
}

type mockAuthService struct{ store UserStore }
