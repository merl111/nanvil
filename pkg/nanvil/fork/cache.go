package fork

import (
	"encoding/binary"
	"encoding/hex"
	"os"
	"path/filepath"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DiskCache stores findstates pages on disk for fork mode.
type DiskCache struct {
	dir string
	mu  sync.RWMutex
}

// NewDiskCache creates a cache under base/network/block/.
func NewDiskCache(base string, networkMagic uint32, blockIndex uint32) (*DiskCache, error) {
	dir := filepath.Join(base, hex.EncodeToString([]byte{
		byte(networkMagic), byte(networkMagic >> 8), byte(networkMagic >> 16), byte(networkMagic >> 24),
	}), fmtUint32(blockIndex))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &DiskCache{dir: dir}, nil
}

func fmtUint32(v uint32) string {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return hex.EncodeToString(b[:])
}

// Get returns cached bytes for key or nil.
func (c *DiskCache) Get(key string) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	raw, err := os.ReadFile(filepath.Join(c.dir, key))
	if err != nil {
		return nil, false
	}
	return raw, true
}

// Put stores bytes under key.
func (c *DiskCache) Put(key string, value []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return os.WriteFile(filepath.Join(c.dir, key), value, 0o644)
}

// Dir returns cache directory path.
func (c *DiskCache) Dir() string {
	return c.dir
}

// CacheKey builds a cache key from contract and prefix.
func CacheKey(contract util.Uint160, prefix []byte) string {
	return contract.StringBE() + "_" + hex.EncodeToString(prefix)
}
