package storage

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zclaw/zclaw/internal/auth"
)

// TenantRepository manages tenant persistence.
type TenantRepository struct {
	db *DB
}

// NewTenantRepository creates a new tenant repository.
func NewTenantRepository(db *DB) *TenantRepository {
	return &TenantRepository{db: db}
}

// Create inserts a new tenant.
func (r *TenantRepository) Create(ctx context.Context, name, slug string, maxAgents int) (*auth.Tenant, error) {
	id := auth.TenantID(newID())
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO tenants (id, name, slug, is_active, max_agents, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?, ?)`,
		string(id), name, slug, maxAgents, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert tenant: %w", err)
	}
	return r.Get(ctx, id)
}

// Get retrieves a tenant by ID.
func (r *TenantRepository) Get(ctx context.Context, id auth.TenantID) (*auth.Tenant, error) {
	var t auth.Tenant
	var isActive int
	var createdAt string
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, name, slug, is_active, max_agents, created_at FROM tenants WHERE id = ?`,
		string(id),
	).Scan(&t.ID, &t.Name, &t.Slug, &isActive, &t.MaxAgents, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get tenant %s: %w", id, err)
	}
	t.IsActive = isActive == 1
	t.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &t, nil
}

// GetBySlug retrieves a tenant by slug.
func (r *TenantRepository) GetBySlug(ctx context.Context, slug string) (*auth.Tenant, error) {
	var id string
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id FROM tenants WHERE slug = ?`, slug,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("get tenant by slug %s: %w", slug, err)
	}
	return r.Get(ctx, auth.TenantID(id))
}

// List returns all tenants, optionally filtered by active status.
func (r *TenantRepository) List(ctx context.Context, activeOnly bool) ([]auth.Tenant, error) {
	query := `SELECT id FROM tenants WHERE 1=1`
	if activeOnly {
		query += ` AND is_active = 1`
	}
	query += ` ORDER BY created_at`

	rows, err := r.db.sqlite.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list tenants: %w", err)
	}
	defer rows.Close()

	var result []auth.Tenant
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		t, err := r.Get(ctx, auth.TenantID(id))
		if err != nil {
			continue
		}
		result = append(result, *t)
	}
	return result, nil
}

// Update modifies a tenant.
func (r *TenantRepository) Update(ctx context.Context, id auth.TenantID, name string, isActive bool, maxAgents int) (*auth.Tenant, error) {
	_, err := r.db.sqlite.ExecContext(ctx,
		`UPDATE tenants SET name=?, is_active=?, max_agents=?, updated_at=? WHERE id=?`,
		name, boolToInt(isActive), maxAgents,
		time.Now().UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return nil, fmt.Errorf("update tenant %s: %w", id, err)
	}
	return r.Get(ctx, id)
}

// Delete permanently removes a tenant and all related data (cascading).
func (r *TenantRepository) Delete(ctx context.Context, id auth.TenantID) error {
	_, err := r.db.sqlite.ExecContext(ctx, `DELETE FROM tenants WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete tenant %s: %w", id, err)
	}
	return nil
}

// UserRepository manages user persistence.
type UserRepository struct {
	db *DB
}

// NewUserRepository creates a new user repository.
func NewUserRepository(db *DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user.
func (r *UserRepository) Create(ctx context.Context, tenantID auth.TenantID, email, name, role string) (*auth.User, error) {
	id := auth.UserID(newID())
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO users (id, tenant_id, email, name, role, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 1, ?, ?)`,
		string(id), string(tenantID), email, name, role, now, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return r.Get(ctx, id)
}

// Get retrieves a user by ID.
func (r *UserRepository) Get(ctx context.Context, id auth.UserID) (*auth.User, error) {
	var u auth.User
	var isActive int
	var createdAt string
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, tenant_id, email, name, role, is_active, created_at FROM users WHERE id = ?`,
		string(id),
	).Scan(&u.ID, &u.TenantID, &u.Email, &u.Name, &u.Role, &isActive, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	u.IsActive = isActive == 1
	u.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &u, nil
}

// GetByEmail retrieves a user by email.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*auth.User, error) {
	var id string
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id FROM users WHERE email = ?`, email,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("get user by email %s: %w", email, err)
	}
	return r.Get(ctx, auth.UserID(id))
}

// ListByTenant returns users for a tenant.
func (r *UserRepository) ListByTenant(ctx context.Context, tenantID auth.TenantID) ([]auth.User, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT id FROM users WHERE tenant_id = ? ORDER BY created_at`, string(tenantID),
	)
	if err != nil {
		return nil, fmt.Errorf("list users for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var result []auth.User
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		u, err := r.Get(ctx, auth.UserID(id))
		if err != nil {
			continue
		}
		result = append(result, *u)
	}
	return result, nil
}

// Update modifies a user.
func (r *UserRepository) Update(ctx context.Context, id auth.UserID, name, role string, isActive bool) (*auth.User, error) {
	_, err := r.db.sqlite.ExecContext(ctx,
		`UPDATE users SET name=?, role=?, is_active=?, updated_at=? WHERE id=?`,
		name, role, boolToInt(isActive),
		time.Now().UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return nil, fmt.Errorf("update user %s: %w", id, err)
	}
	return r.Get(ctx, id)
}

// Delete permanently removes a user.
func (r *UserRepository) Delete(ctx context.Context, id auth.UserID) error {
	_, err := r.db.sqlite.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete user %s: %w", id, err)
	}
	return nil
}

// APIKeyRepository manages API key persistence.
type APIKeyRepository struct {
	db *DB
}

// NewAPIKeyRepository creates a new API key repository.
func NewAPIKeyRepository(db *DB) *APIKeyRepository {
	return &APIKeyRepository{db: db}
}

// Create inserts a new API key.
func (r *APIKeyRepository) Create(ctx context.Context, userID auth.UserID, tenantID auth.TenantID, name, keyPrefix, keyHash, role string, expiresAt *time.Time) (*auth.APIKey, error) {
	id := auth.APIKeyID(newID())
	now := time.Now().UTC().Format(time.RFC3339)
	var expiresAtVal sql.NullString
	if expiresAt != nil {
		expiresAtVal = sql.NullString{String: expiresAt.Format(time.RFC3339), Valid: true}
	}
	_, err := r.db.sqlite.ExecContext(ctx,
		`INSERT INTO api_keys (id, user_id, tenant_id, name, key_prefix, key_hash, role, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		string(id), string(userID), string(tenantID), name, keyPrefix, keyHash, role, expiresAtVal, now,
	)
	if err != nil {
		return nil, fmt.Errorf("insert api key: %w", err)
	}
	return r.Get(ctx, id)
}

// Get retrieves an API key by ID.
func (r *APIKeyRepository) Get(ctx context.Context, id auth.APIKeyID) (*auth.APIKey, error) {
	var k auth.APIKey
	var createdAt string
	var expiresAt, lastUsedAt sql.NullString
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id, user_id, tenant_id, name, key_prefix, key_hash, role, expires_at, last_used_at, created_at
		FROM api_keys WHERE id = ?`, string(id),
	).Scan(&k.ID, &k.UserID, &k.TenantID, &k.Name, &k.KeyPrefix, &k.KeyHash, &k.Role, &expiresAt, &lastUsedAt, &createdAt)
	if err != nil {
		return nil, fmt.Errorf("get api key %s: %w", id, err)
	}
	k.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	if expiresAt.Valid && expiresAt.String != "" {
		t, _ := time.Parse(time.RFC3339, expiresAt.String)
		k.ExpiresAt = &t
	}
	if lastUsedAt.Valid && lastUsedAt.String != "" {
		t, _ := time.Parse(time.RFC3339, lastUsedAt.String)
		k.LastUsedAt = &t
	}
	return &k, nil
}

// GetByHash retrieves an API key by its SHA256 hash.
func (r *APIKeyRepository) GetByHash(ctx context.Context, hash string) (*auth.APIKey, error) {
	var id string
	err := r.db.sqlite.QueryRowContext(ctx,
		`SELECT id FROM api_keys WHERE key_hash = ?`, hash,
	).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("get api key by hash: %w", err)
	}
	return r.Get(ctx, auth.APIKeyID(id))
}

// ListByUser returns API keys for a user.
func (r *APIKeyRepository) ListByUser(ctx context.Context, userID auth.UserID) ([]auth.APIKey, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT id FROM api_keys WHERE user_id = ? ORDER BY created_at`, string(userID),
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys for user %s: %w", userID, err)
	}
	defer rows.Close()

	var result []auth.APIKey
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		k, err := r.Get(ctx, auth.APIKeyID(id))
		if err != nil {
			continue
		}
		result = append(result, *k)
	}
	return result, nil
}

// ListByTenant returns API keys for a tenant.
func (r *APIKeyRepository) ListByTenant(ctx context.Context, tenantID auth.TenantID) ([]auth.APIKey, error) {
	rows, err := r.db.sqlite.QueryContext(ctx,
		`SELECT id FROM api_keys WHERE tenant_id = ? ORDER BY created_at`, string(tenantID),
	)
	if err != nil {
		return nil, fmt.Errorf("list api keys for tenant %s: %w", tenantID, err)
	}
	defer rows.Close()

	var result []auth.APIKey
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			continue
		}
		k, err := r.Get(ctx, auth.APIKeyID(id))
		if err != nil {
			continue
		}
		result = append(result, *k)
	}
	return result, nil
}

// UpdateLastUsed sets last_used_at to now.
func (r *APIKeyRepository) UpdateLastUsed(ctx context.Context, id auth.APIKeyID) error {
	_, err := r.db.sqlite.ExecContext(ctx,
		`UPDATE api_keys SET last_used_at = ? WHERE id = ?`,
		time.Now().UTC().Format(time.RFC3339), string(id),
	)
	if err != nil {
		return fmt.Errorf("update api key last used %s: %w", id, err)
	}
	return nil
}

// Delete permanently removes an API key.
func (r *APIKeyRepository) Delete(ctx context.Context, id auth.APIKeyID) error {
	_, err := r.db.sqlite.ExecContext(ctx, `DELETE FROM api_keys WHERE id = ?`, string(id))
	if err != nil {
		return fmt.Errorf("delete api key %s: %w", id, err)
	}
	return nil
}

// AuthStore adapts storage repositories to implement the auth.UserStore interface.
type AuthStore struct {
	apiKeys *APIKeyRepository
	users   *UserRepository
}

// NewAuthStore creates a new AuthStore.
func NewAuthStore(apiKeys *APIKeyRepository, users *UserRepository) *AuthStore {
	return &AuthStore{apiKeys: apiKeys, users: users}
}

// GetAPIKeyByHash retrieves an API key by its SHA256 hash.
func (s *AuthStore) GetAPIKeyByHash(ctx context.Context, hash string) (*auth.APIKey, error) {
	return s.apiKeys.GetByHash(ctx, hash)
}

// GetUserByID retrieves a user by ID.
func (s *AuthStore) GetUserByID(ctx context.Context, id auth.UserID) (*auth.User, error) {
	return s.users.Get(ctx, id)
}

// UpdateAPIKeyLastUsed sets last_used_at to now.
func (s *AuthStore) UpdateAPIKeyLastUsed(ctx context.Context, id auth.APIKeyID) error {
	return s.apiKeys.UpdateLastUsed(ctx, id)
}
