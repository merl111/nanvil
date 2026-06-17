package txregistry

import (
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Status describes a transaction relay outcome tracked by nanvil.
type Status uint8

const (
	// StatusRelayed means sendrawtransaction accepted the transaction.
	StatusRelayed Status = iota
	// StatusRejected means sendrawtransaction rejected the transaction.
	StatusRejected
)

// Entry is a recorded relay outcome.
type Entry struct {
	Status Status
	Err    error
}

var (
	mu      sync.RWMutex
	entries = make(map[util.Uint256]Entry)
)

// RecordRelayed records a successfully relayed transaction hash.
func RecordRelayed(hash util.Uint256) {
	mu.Lock()
	entries[hash] = Entry{Status: StatusRelayed}
	mu.Unlock()
}

// RecordRejected records a rejected transaction hash and relay error.
func RecordRejected(hash util.Uint256, err error) {
	mu.Lock()
	entries[hash] = Entry{Status: StatusRejected, Err: err}
	mu.Unlock()
}

// Lookup returns a recorded relay outcome.
func Lookup(hash util.Uint256) (Entry, bool) {
	mu.RLock()
	defer mu.RUnlock()
	entry, ok := entries[hash]
	return entry, ok
}

// Reset clears relay history (used on dev chain reset).
func Reset() {
	mu.Lock()
	entries = make(map[util.Uint256]Entry)
	mu.Unlock()
}
