package fork

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// RemoteStateStore lazily fetches contract storage from a remote RPC node.
type RemoteStateStore struct {
	manifest      *Manifest
	client        *rpcclient.Client
	cache         *DiskCache
	noCache       bool
	mu            sync.Mutex
	mem           map[string][]byte
	contractByID  map[int32]util.Uint160
	contractByHash map[util.Uint160]int32
}

// NewRemoteStateStore creates a lazy remote reader.
func NewRemoteStateStore(ctx context.Context, m *Manifest, cache *DiskCache, noCache bool) (*RemoteStateStore, error) {
	c, err := rpcclient.New(ctx, m.RPCURL, rpcclient.Options{})
	if err != nil {
		return nil, err
	}
	rs := &RemoteStateStore{
		manifest:       m,
		client:         c,
		cache:          cache,
		noCache:        noCache,
		mem:            make(map[string][]byte),
		contractByID:   make(map[int32]util.Uint160),
		contractByHash: make(map[util.Uint160]int32),
	}
	for i := range m.Contracts {
		ci := m.Contracts[i]
		rs.contractByID[ci.ID] = ci.Hash
		rs.contractByHash[ci.Hash] = ci.ID
	}
	natives, err := c.GetNativeContracts()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("get native contracts: %w", err)
	}
	for i := range natives {
		rs.contractByID[natives[i].ID] = natives[i].Hash
		rs.contractByHash[natives[i].Hash] = natives[i].ID
	}
	if err := c.Init(); err != nil {
		c.Close()
		return nil, fmt.Errorf("init rpc client: %w", err)
	}
	return rs, nil
}

// Close closes the RPC client.
func (r *RemoteStateStore) Close() {
	if r.client != nil {
		r.client.Close()
	}
}

// GetProof fetches and verifies a single storage value.
func (r *RemoteStateStore) GetProof(contract util.Uint160, key []byte) ([]byte, error) {
	cacheKey := contract.StringBE() + "_" + fmt.Sprintf("%x", key)
	r.mu.Lock()
	if v, ok := r.mem[cacheKey]; ok {
		r.mu.Unlock()
		return v, nil
	}
	r.mu.Unlock()
	if !r.noCache && r.cache != nil {
		if raw, ok := r.cache.Get(cacheKey); ok {
			r.mu.Lock()
			r.mem[cacheKey] = raw
			r.mu.Unlock()
			return raw, nil
		}
	}
	proof, err := r.client.GetProof(r.manifest.RootHash, contract, key)
	if err != nil {
		return nil, err
	}
	val, ok := mpt.VerifyProof(r.manifest.RootHash, proof.Key, proof.Proof)
	if !ok {
		return nil, fmt.Errorf("invalid proof for key %x", key)
	}
	r.mu.Lock()
	r.mem[cacheKey] = val
	r.mu.Unlock()
	if !r.noCache && r.cache != nil {
		_ = r.cache.Put(cacheKey, val)
	}
	return val, nil
}

// PrefetchContract downloads all storage for a contract via findstates.
func (r *RemoteStateStore) PrefetchContract(contract util.Uint160) error {
	var from []byte
	for {
		res, err := r.client.FindStates(r.manifest.RootHash, contract, nil, from, nil)
		if err != nil {
			return err
		}
		r.mu.Lock()
		for _, kv := range res.Results {
			key := contract.StringBE() + "_" + fmt.Sprintf("%x", kv.Key)
			r.mem[key] = kv.Value
			if !r.noCache && r.cache != nil {
				_ = r.cache.Put(key, kv.Value)
			}
		}
		r.mu.Unlock()
		if !res.Truncated || len(res.Results) == 0 {
			break
		}
		from = res.Results[len(res.Results)-1].Key
	}
	return nil
}

// ContractHash resolves a contract hash by ID at the branch point.
func (r *RemoteStateStore) ContractHash(id int32) (util.Uint160, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	h, ok := r.contractByID[id]
	return h, ok
}

// IterateContractStorage calls cont for each remote item matching prefix/start.
func (r *RemoteStateStore) IterateContractStorage(contract util.Uint160, prefix, start []byte, cont func(key, value []byte) bool) error {
	seekPrefix := append(bytes.Clone(prefix), start...)
	var from []byte
	for {
		res, err := r.client.FindStates(r.manifest.RootHash, contract, seekPrefix, from, nil)
		if err != nil {
			return err
		}
		for _, kv := range res.Results {
			if len(prefix) > 0 && !bytes.HasPrefix(kv.Key, prefix) {
				continue
			}
			if len(start) > 0 && bytes.Compare(kv.Key, start) < 0 {
				continue
			}
			cacheKey := contract.StringBE() + "_" + fmt.Sprintf("%x", kv.Key)
			r.mu.Lock()
			r.mem[cacheKey] = kv.Value
			r.mu.Unlock()
			if !r.noCache && r.cache != nil {
				_ = r.cache.Put(cacheKey, kv.Value)
			}
			if !cont(kv.Key, kv.Value) {
				return nil
			}
		}
		if !res.Truncated || len(res.Results) == 0 {
			break
		}
		from = res.Results[len(res.Results)-1].Key
	}
	return nil
}

// GetBlockByHash fetches a block from the remote RPC node.
func (r *RemoteStateStore) GetBlockByHash(hash util.Uint256) (*block.Block, error) {
	return r.client.GetBlockByHash(hash)
}

// CachedCount returns number of cached entries.
func (r *RemoteStateStore) CachedCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.mem)
}
