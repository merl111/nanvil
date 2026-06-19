package persist_test

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/persist"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestTryLoadChainSnapshotWithFork(t *testing.T) {
	base := storage.NewMemoryStore()
	path := filepath.Join(t.TempDir(), "chain.state.json")
	m := &fork.Manifest{
		RPCURL:       "http://127.0.0.1:20331",
		NetworkMagic: 860833102,
		Index:        3,
		IndexHash:    util.Uint256{1},
		RootHash:     util.Uint256{2},
	}
	require.NoError(t, persist.Save(path, 3, nil, base, fork.NewTrackingOverlay(), m))

	snap, err := persist.TryLoad(path)
	require.NoError(t, err)
	require.NotNil(t, snap)
	require.NotNil(t, snap.Fork)
	require.Equal(t, uint32(3), snap.Fork.Index)
}

func TestTryLoadMissingFile(t *testing.T) {
	snap, err := persist.TryLoad(filepath.Join(t.TempDir(), "missing.json"))
	require.NoError(t, err)
	require.Nil(t, snap)
}
