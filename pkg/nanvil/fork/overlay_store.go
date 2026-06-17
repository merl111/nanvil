package fork

import (
	"bytes"
	"cmp"
	"encoding/binary"
	"fmt"
	"math/big"
	"slices"
	"strings"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Store overlays local writes and lazy remote reads on top of a base store.
// Read order for contract storage: overlay > remote > base.
type Store struct {
	base    storage.Store
	remote  *RemoteStateStore
	overlay *TrackingOverlay
	mu      sync.RWMutex
}

// NewStore creates a fork storage overlay.
func NewStore(base storage.Store, overlay *TrackingOverlay) *Store {
	return &Store{base: base, overlay: overlay}
}

// SetRemote attaches the lazy remote reader (may be called after construction).
func (s *Store) SetRemote(remote *RemoteStateStore) {
	s.mu.Lock()
	s.remote = remote
	s.mu.Unlock()
}

// Close closes the underlying base store.
func (s *Store) Close() error {
	return s.base.Close()
}

// Get implements storage.Store.
func (s *Store) Get(key []byte) ([]byte, error) {
	if !isContractStorageKey(key) {
		return s.base.Get(key)
	}
	skey := string(key)
	if v, ok := s.overlay.Get(skey); ok {
		if v == nil {
			return nil, storage.ErrKeyNotFound
		}
		return v, nil
	}
	if v, ok := s.getRemote(key); ok {
		return v, nil
	}
	return s.base.Get(key)
}

func (s *Store) getRemote(key []byte) ([]byte, bool) {
	s.mu.RLock()
	remote := s.remote
	s.mu.RUnlock()
	if remote == nil {
		return nil, false
	}
	id, itemKey, ok := parseContractStorageKey(key)
	if !ok {
		return nil, false
	}
	hash, ok := remote.ContractHash(id)
	if !ok {
		return nil, false
	}
	val, err := remote.GetProof(hash, itemKey)
	if err != nil {
		return nil, false
	}
	return val, true
}

// PutChangeSet implements storage.Store.
func (s *Store) PutChangeSet(puts map[string][]byte, stores map[string][]byte) error {
	for k, v := range stores {
		key := []byte(k)
		if !isContractStorageKey(key) {
			continue
		}
		if v == nil {
			s.overlay.Delete(k)
		} else {
			s.overlay.Put(k, v)
		}
	}
	return s.base.PutChangeSet(puts, stores)
}

// Seek implements storage.Store.
func (s *Store) Seek(rng storage.SeekRange, f func(k, v []byte) bool) {
	if len(rng.Prefix) == 0 || !isContractStorageKey(rng.Prefix) {
		s.base.Seek(rng, f)
		return
	}
	for _, kv := range s.collectSeek(rng) {
		if !f(kv.Key, kv.Value) {
			break
		}
	}
}

// SeekGC implements storage.Store.
func (s *Store) SeekGC(rng storage.SeekRange, keepCont func(k, v []byte) (bool, bool)) error {
	if len(rng.Prefix) == 0 || !isContractStorageKey(rng.Prefix) {
		return s.base.SeekGC(rng, keepCont)
	}
	var toDelete []string
	for _, kv := range s.collectSeek(rng) {
		keep, cont := keepCont(kv.Key, kv.Value)
		if !keep {
			toDelete = append(toDelete, string(kv.Key))
			s.overlay.Delete(string(kv.Key))
		}
		if !cont {
			break
		}
	}
	if len(toDelete) > 0 {
		puts := make(map[string][]byte)
		stores := make(map[string][]byte)
		for _, k := range toDelete {
			stores[k] = nil
		}
		_ = s.base.PutChangeSet(puts, stores)
	}
	return nil
}

func (s *Store) collectSeek(rng storage.SeekRange) []storage.KeyValue {
	merged := make(map[string][]byte)

	s.base.Seek(rng, func(k, v []byte) bool {
		merged[string(k)] = bytes.Clone(v)
		return true
	})

	s.mu.RLock()
	remote := s.remote
	s.mu.RUnlock()
	if remote != nil {
		if id, subPrefix, ok := contractIDFromPrefix(rng.Prefix); ok {
			if hash, ok := remote.ContractHash(id); ok {
				_ = remote.IterateContractStorage(hash, subPrefix, rng.Start, func(itemKey, value []byte) bool {
					full := makeContractStorageKey(id, itemKey)
					merged[string(full)] = bytes.Clone(value)
					return true
				})
			}
		}
	}

	s.overlay.mu.RLock()
	for k, v := range s.overlay.writes {
		if keyInSeekRange([]byte(k), rng) {
			merged[k] = bytes.Clone(v)
		}
	}
	for k := range s.overlay.deletes {
		if keyInSeekRange([]byte(k), rng) {
			delete(merged, k)
		}
	}
	s.overlay.mu.RUnlock()

	out := make([]storage.KeyValue, 0, len(merged))
	for k, v := range merged {
		out = append(out, storage.KeyValue{Key: []byte(k), Value: v})
	}
	slices.SortFunc(out, func(a, b storage.KeyValue) int {
		return bytes.Compare(a.Key, b.Key)
	})
	return out
}

func isContractStorageKey(key []byte) bool {
	if len(key) < 1 {
		return false
	}
	p := storage.KeyPrefix(key[0])
	return p == storage.STStorage || p == storage.STTempStorage
}

func parseContractStorageKey(key []byte) (id int32, itemKey []byte, ok bool) {
	if len(key) < 5 || !isContractStorageKey(key) {
		return 0, nil, false
	}
	id = int32(binary.LittleEndian.Uint32(key[1:5]))
	return id, key[5:], true
}

func contractIDFromPrefix(prefix []byte) (id int32, subPrefix []byte, ok bool) {
	if len(prefix) < 5 || !isContractStorageKey(prefix) {
		return 0, nil, false
	}
	id = int32(binary.LittleEndian.Uint32(prefix[1:5]))
	return id, prefix[5:], true
}

func makeContractStorageKey(id int32, itemKey []byte) []byte {
	buf := make([]byte, 5+len(itemKey))
	buf[0] = byte(storage.STStorage)
	binary.LittleEndian.PutUint32(buf[1:], uint32(id))
	copy(buf[5:], itemKey)
	return buf
}

func keyInSeekRange(key []byte, rng storage.SeekRange) bool {
	sPrefix := string(rng.Prefix)
	if !strings.HasPrefix(string(key), sPrefix) {
		return false
	}
	if len(rng.Start) == 0 {
		return true
	}
	return cmp.Compare(string(key)[len(sPrefix):], string(rng.Start)) >= 0
}

// PutStorageOverlay writes a contract storage item into the overlay (used during bootstrap).
func PutStorageOverlay(overlay *TrackingOverlay, id int32, key []byte, value []byte) {
	overlay.Put(string(makeContractStorageKey(id, key)), value)
}

// PutGasBalanceOverlay funds an account in fork mode without mining.
func PutGasBalanceOverlay(overlay *TrackingOverlay, account util.Uint160, amount int64) error {
	if amount < 0 {
		return fmt.Errorf("negative GAS balance")
	}
	const gasID int32 = -6
	key := make([]byte, 1+util.Uint160Size)
	key[0] = 20
	copy(key[1:], account.BytesBE())
	bal := state.NEP17Balance{Balance: *big.NewInt(amount)}
	PutStorageOverlay(overlay, gasID, key, bal.Bytes(nil))
	return nil
}
