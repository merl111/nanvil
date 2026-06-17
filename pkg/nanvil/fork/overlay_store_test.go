package fork_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

func TestStoreOverlayReadPriority(t *testing.T) {
	base := storage.NewMemoryStore()
	overlay := fork.NewTrackingOverlay()
	store := fork.NewStore(base, overlay)

	id := int32(-6)
	key := make([]byte, 21)
	key[0] = 20
	acc := util.Uint160{1}
	copy(key[1:], acc.BytesBE())
	fullKey := makeStorageKey(id, key)

	_ = base.PutChangeSet(nil, map[string][]byte{string(fullKey): []byte("base")})
	if err := fork.PutGasBalanceOverlay(overlay, acc, 42); err != nil {
		t.Fatal(err)
	}

	got, err := store.Get(fullKey)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected overlay balance bytes")
	}
}

func makeStorageKey(id int32, key []byte) []byte {
	buf := make([]byte, 5+len(key))
	buf[0] = byte(storage.STStorage)
	buf[1] = byte(id)
	buf[2] = byte(id >> 8)
	buf[3] = byte(id >> 16)
	buf[4] = byte(id >> 24)
	copy(buf[5:], key)
	return buf
}
