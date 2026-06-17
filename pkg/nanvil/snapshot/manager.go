package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core"
)

// ID identifies a snapshot.
type ID string

// State captures restorable chain metadata.
type State struct {
	ID     ID     `json:"id"`
	Height uint32 `json:"height"`
}

// Manager tracks snapshot stack for revert operations.
type Manager struct {
	mu        sync.Mutex
	chain     *core.Blockchain
	snapshots []State
	counter   int
}

// NewManager creates a snapshot manager.
func NewManager(chain *core.Blockchain) *Manager {
	return &Manager{chain: chain}
}

// Snapshot records current height as a snapshot point.
func (m *Manager) Snapshot() (ID, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	id := ID(fmt.Sprintf("0x%x", m.counter))
	m.snapshots = append(m.snapshots, State{
		ID:     id,
		Height: m.chain.BlockHeight(),
	})
	return id, nil
}

// Revert rolls chain back to a snapshot height.
func (m *Manager) Revert(id ID) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	var target uint32
	found := -1
	for i, s := range m.snapshots {
		if s.ID == id {
			target = s.Height
			found = i
			break
		}
	}
	if found < 0 {
		return fmt.Errorf("snapshot %q not found", id)
	}
	if err := m.chain.Reset(target); err != nil {
		return err
	}
	m.snapshots = m.snapshots[:found+1]
	return nil
}

// List returns known snapshots.
func (m *Manager) List() []State {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]State, len(m.snapshots))
	copy(out, m.snapshots)
	return out
}

// Restore replaces the snapshot list.
func (m *Manager) Restore(snaps []State) {
	m.mu.Lock()
	m.snapshots = append([]State(nil), snaps...)
	m.mu.Unlock()
}

// DumpFile writes chain height and snapshot list to path.
func (m *Manager) DumpFile(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data := struct {
		Height    uint32  `json:"height"`
		Snapshots []State `json:"snapshots"`
	}{
		Height:    m.chain.BlockHeight(),
		Snapshots: append([]State(nil), m.snapshots...),
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// LoadFile restores snapshot metadata (height reset must be done separately if needed).
func (m *Manager) LoadFile(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var data struct {
		Height    uint32  `json:"height"`
		Snapshots []State `json:"snapshots"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return err
	}
	m.mu.Lock()
	m.snapshots = data.Snapshots
	if data.Height > 0 && data.Height < m.chain.BlockHeight() {
		if err := m.chain.Reset(data.Height); err != nil {
			m.mu.Unlock()
			return err
		}
	}
	m.mu.Unlock()
	return nil
}
