package auth

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Role string

const (
	RoleAdmin     Role = "admin"
	RoleOperator  Role = "operator"
	RoleViewer    Role = "viewer"
	RoleAgent     Role = "agent"
	RoleAnonymous Role = "anonymous"
)

type Permission string

const (
	PermAgentCreate  Permission = "agent:create"
	PermAgentRead    Permission = "agent:read"
	PermAgentUpdate  Permission = "agent:update"
	PermAgentDelete  Permission = "agent:delete"
	PermTaskEnqueue  Permission = "task:enqueue"
	PermTaskRead     Permission = "task:read"
	PermToolExecute  Permission = "tool:execute"
	PermDashboard    Permission = "dashboard:read"
	PermUserManage   Permission = "user:manage"
	PermTenantManage Permission = "tenant:manage"
	PermAPIKeyManage Permission = "apikey:manage"
)

type TenantID string
type UserID string
type APIKeyID string

type Tenant struct {
	ID        TenantID  `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	IsActive  bool      `json:"is_active"`
	MaxAgents int       `json:"max_agents"`
	CreatedAt time.Time `json:"created_at"`
}

type User struct {
	ID        UserID    `json:"id"`
	TenantID  TenantID  `json:"tenant_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Role      Role      `json:"role"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type APIKey struct {
	ID         APIKeyID   `json:"id"`
	UserID     UserID     `json:"user_id"`
	TenantID   TenantID   `json:"tenant_id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	KeyHash    string     `json:"-"`
	Role       Role       `json:"role"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type Session struct {
	ID        string    `json:"id"`
	UserID    UserID    `json:"user_id"`
	TenantID  TenantID  `json:"tenant_id"`
	Role      Role      `json:"role"`
	ExpiresAt time.Time `json:"expires_at"`
}

type AuthContext struct {
	UserID   UserID   `json:"user_id"`
	TenantID TenantID `json:"tenant_id"`
	Role     Role     `json:"role"`
	APIKeyID APIKeyID `json:"api_key_id,omitempty"`
	Session  bool     `json:"session"`
}

var rolePermissions = map[Role][]Permission{
	RoleAdmin: {
		PermAgentCreate, PermAgentRead, PermAgentUpdate, PermAgentDelete,
		PermTaskEnqueue, PermTaskRead, PermToolExecute, PermDashboard,
		PermUserManage, PermTenantManage, PermAPIKeyManage,
	},
	RoleOperator: {
		PermAgentCreate, PermAgentRead, PermAgentUpdate, PermAgentDelete,
		PermTaskEnqueue, PermTaskRead, PermToolExecute, PermDashboard,
	},
	RoleViewer: {
		PermAgentRead, PermTaskRead, PermDashboard,
	},
	RoleAgent: {
		PermTaskEnqueue, PermToolExecute,
	},
	RoleAnonymous: {},
}

func GenerateAPIKey(prefix string) (plainText string, hash string, err error) {
	b := make([]byte, 16)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	plain := prefix + hex.EncodeToString(b)
	return plain, HashAPIKey(plain), nil
}

func HashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func VerifyAPIKey(plainText, storedHash string) bool {
	if plainText == "" || storedHash == "" {
		return false
	}
	computed := HashAPIKey(plainText)
	return hmac.Equal([]byte(computed), []byte(storedHash))
}

func CreateSessionToken(session Session, secret string) (string, error) {
	payload := struct {
		Sub  string `json:"sub"`
		Tid  string `json:"tid"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
		Iat  int64  `json:"iat"`
	}{
		Sub:  string(session.UserID),
		Tid:  string(session.TenantID),
		Role: string(session.Role),
		Exp:  session.ExpiresAt.Unix(),
		Iat:  time.Now().Unix(),
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadB64))
	sig := mac.Sum(nil)
	sigB64 := base64.RawURLEncoding.EncodeToString(sig)
	return payloadB64 + "." + sigB64, nil
}

func VerifySessionToken(token string, secret string) (*Session, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}
	payloadB64 := parts[0]
	sigB64 := parts[1]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payloadB64))
	expectedSig := mac.Sum(nil)
	sig, err := base64.RawURLEncoding.DecodeString(sigB64)
	if err != nil {
		return nil, err
	}
	if !hmac.Equal(sig, expectedSig) {
		return nil, fmt.Errorf("invalid token signature")
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(payloadB64)
	if err != nil {
		return nil, err
	}
	var p struct {
		Sub  string `json:"sub"`
		Tid  string `json:"tid"`
		Role string `json:"role"`
		Exp  int64  `json:"exp"`
		Iat  int64  `json:"iat"`
	}
	if err := json.Unmarshal(payloadJSON, &p); err != nil {
		return nil, err
	}
	if time.Now().Unix() > p.Exp {
		return nil, fmt.Errorf("token expired")
	}
	s := &Session{
		ID:        p.Sub,
		UserID:    UserID(p.Sub),
		TenantID:  TenantID(p.Tid),
		Role:      Role(p.Role),
		ExpiresAt: time.Unix(p.Exp, 0),
	}
	return s, nil
}

func HasPermission(role Role, perm Permission) bool {
	for _, p := range rolePermissions[role] {
		if p == perm {
			return true
		}
	}
	return false
}

func HasAnyPermission(role Role, perms ...Permission) bool {
	for _, p := range perms {
		if HasPermission(role, p) {
			return true
		}
	}
	return false
}

type AuthService interface {
	AuthenticateAPIKey(ctx context.Context, key string) (*AuthContext, error)
	AuthenticateSession(ctx context.Context, token string) (*AuthContext, error)
	Authorize(ctx context.Context, ac *AuthContext, perm Permission) error
}

type UserStore interface {
	GetAPIKeyByHash(ctx context.Context, hash string) (*APIKey, error)
	GetUserByID(ctx context.Context, id UserID) (*User, error)
	UpdateAPIKeyLastUsed(ctx context.Context, id APIKeyID) error
}

type AuthServiceImpl struct {
	store     UserStore
	jwtSecret string
}

func NewAuthService(store UserStore, jwtSecret string) *AuthServiceImpl {
	return &AuthServiceImpl{store: store, jwtSecret: jwtSecret}
}

func (a *AuthServiceImpl) AuthenticateAPIKey(ctx context.Context, key string) (*AuthContext, error) {
	hash := HashAPIKey(key)
	apiKey, err := a.store.GetAPIKeyByHash(ctx, hash)
	if err != nil {
		return nil, err
	}
	if apiKey == nil {
		return nil, fmt.Errorf("invalid API key")
	}
	if apiKey.ExpiresAt != nil && apiKey.ExpiresAt.Before(time.Now()) {
		return nil, fmt.Errorf("API key expired")
	}
	if err := a.store.UpdateAPIKeyLastUsed(ctx, apiKey.ID); err != nil {
		return nil, err
	}
	return &AuthContext{
		UserID:   apiKey.UserID,
		TenantID: apiKey.TenantID,
		Role:     apiKey.Role,
		APIKeyID: apiKey.ID,
		Session:  true,
	}, nil
}

func (a *AuthServiceImpl) AuthenticateSession(ctx context.Context, token string) (*AuthContext, error) {
	sess, err := VerifySessionToken(token, a.jwtSecret)
	if err != nil {
		return nil, err
	}
	user, err := a.store.GetUserByID(ctx, sess.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil || !user.IsActive {
		return nil, fmt.Errorf("inactive or unknown user")
	}
	return &AuthContext{
		UserID:   sess.UserID,
		TenantID: sess.TenantID,
		Role:     sess.Role,
		Session:  true,
	}, nil
}

func (a *AuthServiceImpl) Authorize(ctx context.Context, ac *AuthContext, perm Permission) error {
	if ac == nil {
		return fmt.Errorf("no auth context provided")
	}
	if HasPermission(ac.Role, perm) {
		return nil
	}
	return fmt.Errorf("permission denied: %s", perm)
}
