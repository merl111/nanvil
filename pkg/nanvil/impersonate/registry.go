package impersonate

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Registry tracks impersonated script hashes for dev-mode witness bypass.
type Registry struct {
	mu       sync.RWMutex
	accounts map[util.Uint160]struct{}
	autoMode bool
	enabled  bool
}

var global = &Registry{
	accounts: make(map[util.Uint160]struct{}),
	enabled:  true,
}

// NewRegistry creates an isolated impersonation registry (for tests).
func NewRegistry() *Registry {
	return &Registry{
		accounts: make(map[util.Uint160]struct{}),
		enabled:  true,
	}
}

// Global returns the process-wide impersonation registry.
func Global() *Registry {
	return global
}

// SetEnabled toggles impersonation support.
func (r *Registry) SetEnabled(v bool) {
	r.mu.Lock()
	r.enabled = v
	r.mu.Unlock()
}

// Enabled reports whether impersonation is active.
func (r *Registry) Enabled() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.enabled
}

// Impersonate registers an account for witness bypass.
func (r *Registry) Impersonate(hash util.Uint160) {
	r.mu.Lock()
	r.accounts[hash] = struct{}{}
	r.mu.Unlock()
}

// StopImpersonating removes an account from the registry.
func (r *Registry) StopImpersonating(hash util.Uint160) {
	r.mu.Lock()
	delete(r.accounts, hash)
	r.mu.Unlock()
}

// SetAutoMode enables impersonation for any signer in transactions.
func (r *Registry) SetAutoMode(v bool) {
	r.mu.Lock()
	r.autoMode = v
	r.mu.Unlock()
}

// AutoMode reports auto-impersonate state.
func (r *Registry) AutoMode() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.autoMode
}

// IsImpersonated returns true if hash should bypass witness verification.
func (r *Registry) IsImpersonated(hash util.Uint160) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if !r.enabled {
		return false
	}
	if r.autoMode {
		return true
	}
	_, ok := r.accounts[hash]
	return ok
}

// List returns currently impersonated hashes.
func (r *Registry) List() []util.Uint160 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]util.Uint160, 0, len(r.accounts))
	for h := range r.accounts {
		out = append(out, h)
	}
	return out
}

// Reset clears all impersonated accounts and auto mode.
func (r *Registry) Reset() {
	r.mu.Lock()
	r.accounts = make(map[util.Uint160]struct{})
	r.autoMode = false
	r.mu.Unlock()
}
