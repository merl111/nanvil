package persist_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/persist"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/stretchr/testify/require"
)

func TestChainSnapshotRoundTrip(t *testing.T) {
	base := storage.NewMemoryStore()
	require.NoError(t, base.PutChangeSet(
		map[string][]byte{"\xc0key": []byte("block")},
		map[string][]byte{string([]byte{byte(storage.STStorage), 0x01, 0x02, 0x03, 0x04, 'k'}): []byte("value")},
	))

	overlay := fork.NewTrackingOverlay()
	overlay.Put(string([]byte{byte(storage.STStorage), 0x05, 0, 0, 0, 'x'}), []byte("overlay"))
	overlay.Delete(string([]byte{byte(storage.STStorage), 0x06, 0, 0, 0, 'y'}))

	path := filepath.Join(t.TempDir(), "chain.state.json")
	require.NoError(t, persist.Save(path, 42, []snapshot.State{{ID: "0x1", Height: 42}}, base, overlay, nil))

	loaded, err := persist.Load(path)
	require.NoError(t, err)
	require.Equal(t, uint32(42), loaded.Height)
	require.Len(t, loaded.Snapshots, 1)

	restored := storage.NewMemoryStore()
	restoredOverlay := fork.NewTrackingOverlay()
	require.NoError(t, persist.Apply(loaded, restored, restoredOverlay))

	mem, stor := restored.Export()
	require.Equal(t, []byte("block"), mem["\xc0key"])
	require.Len(t, stor, 1)

	v, ok := restoredOverlay.Get(string([]byte{byte(storage.STStorage), 0x05, 0, 0, 0, 'x'}))
	require.True(t, ok)
	require.Equal(t, []byte("overlay"), v)
	_, del := restoredOverlay.Get(string([]byte{byte(storage.STStorage), 0x06, 0, 0, 0, 'y'}))
	require.True(t, del)
}

func TestTryLoadLegacyMetadata(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.json")
	require.NoError(t, osWrite(path, `{"height":1,"snapshots":[]}`))

	snap, err := persist.TryLoad(path)
	require.NoError(t, err)
	require.Nil(t, snap)
}

func osWrite(path, data string) error {
	return os.WriteFile(path, []byte(data), 0o644)
}
