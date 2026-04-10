package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

type LocalRegistry struct {
	mu        sync.RWMutex
	providers map[ProviderID]Provider
	configs   map[ProviderID]ProviderConfig
}

func NewLocalRegistry() *LocalRegistry {
	return &LocalRegistry{
		providers: make(map[ProviderID]Provider),
		configs:   make(map[ProviderID]ProviderConfig),
	}
}

func (r *LocalRegistry) Register(_ context.Context, p Provider) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[p.ID()] = p
	return nil
}

func (r *LocalRegistry) Get(_ context.Context, id ProviderID) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[id]
	if !ok {
		return nil, &ProviderNotFoundError{ID: id}
	}
	return p, nil
}

func (r *LocalRegistry) List(_ context.Context) []ProviderID {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]ProviderID, 0, len(r.providers))
	for id := range r.providers {
		ids = append(ids, id)
	}
	return ids
}

func (r *LocalRegistry) Configure(_ context.Context, id ProviderID, config ProviderConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.providers[id]
	if !ok {
		return &ProviderNotFoundError{ID: id}
	}
	if err := p.ValidateConfig(config); err != nil {
		return fmt.Errorf("invalid config for %s: %w", id, err)
	}
	r.configs[id] = config
	return nil
}

func (r *LocalRegistry) GetConfig(_ context.Context, id ProviderID) (*ProviderConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.configs[id]
	if !ok {
		return nil, &ProviderNotFoundError{ID: id}
	}
	return &c, nil
}

func (r *LocalRegistry) ResolveProvider(ctx context.Context, modelRef string) (Provider, string, error) {
	if parts := strings.SplitN(modelRef, "/", 2); len(parts) == 2 {
		p, err := r.Get(ctx, ProviderID(parts[0]))
		if err != nil {
			return nil, "", err
		}
		return p, p.NormalizeModel(parts[1]), nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, p := range r.providers {
		normalized := p.NormalizeModel(modelRef)
		if normalized != modelRef {
			return p, normalized, nil
		}
	}

	if len(r.providers) > 0 {
		for _, p := range r.providers {
			return p, modelRef, nil
		}
	}

	return nil, "", &ProviderNotFoundError{ID: ProviderID(modelRef)}
}

func (r *LocalRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs []error
	for _, p := range r.providers {
		if err := p.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}
	return nil
}
