package fork

import (
	"sync"
)

// TrackingOverlay stores local writes over a forked remote state (in-memory).
type TrackingOverlay struct {
	mu      sync.RWMutex
	writes  map[string][]byte
	deletes map[string]struct{}
}

// NewTrackingOverlay creates an empty overlay.
func NewTrackingOverlay() *TrackingOverlay {
	return &TrackingOverlay{
		writes:  make(map[string][]byte),
		deletes: make(map[string]struct{}),
	}
}

// Put records a local write.
func (o *TrackingOverlay) Put(key string, value []byte) {
	o.mu.Lock()
	delete(o.deletes, key)
	o.writes[key] = append([]byte(nil), value...)
	o.mu.Unlock()
}

// Delete records a local delete.
func (o *TrackingOverlay) Delete(key string) {
	o.mu.Lock()
	delete(o.writes, key)
	o.deletes[key] = struct{}{}
	o.mu.Unlock()
}

// Get returns overlay value if present.
func (o *TrackingOverlay) Get(key string) ([]byte, bool) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	if _, del := o.deletes[key]; del {
		return nil, true
	}
	v, ok := o.writes[key]
	if !ok {
		return nil, false
	}
	return append([]byte(nil), v...), true
}

// Reset clears overlay writes (keeps remote cache intact).
func (o *TrackingOverlay) Reset() {
	o.mu.Lock()
	o.writes = make(map[string][]byte)
	o.deletes = make(map[string]struct{})
	o.mu.Unlock()
}

// Export returns copies of overlay writes and deletes.
func (o *TrackingOverlay) Export() (writes map[string][]byte, deletes []string) {
	o.mu.RLock()
	defer o.mu.RUnlock()
	writes = make(map[string][]byte, len(o.writes))
	for k, v := range o.writes {
		writes[k] = append([]byte(nil), v...)
	}
	deletes = make([]string, 0, len(o.deletes))
	for k := range o.deletes {
		deletes = append(deletes, k)
	}
	return writes, deletes
}

// Import replaces overlay contents.
func (o *TrackingOverlay) Import(writes map[string][]byte, deletes []string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.writes = make(map[string][]byte, len(writes))
	for k, v := range writes {
		o.writes[k] = append([]byte(nil), v...)
	}
	o.deletes = make(map[string]struct{}, len(deletes))
	for _, k := range deletes {
		o.deletes[k] = struct{}{}
	}
}

// Len returns overlay entry count.
func (o *TrackingOverlay) Len() int {
	o.mu.RLock()
	defer o.mu.RUnlock()
	return len(o.writes) + len(o.deletes)
}
