package snapshot_test

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/stretchr/testify/require"
)

func TestSnapshotLoadFileAndRestore(t *testing.T) {
	bc := testChain(t)
	m := snapshot.NewManager(bc)

	id1, err := m.Snapshot()
	require.NoError(t, err)
	id2, err := m.Snapshot()
	require.NoError(t, err)
	require.NotEqual(t, id1, id2)

	path := filepath.Join(t.TempDir(), "snaps.json")
	require.NoError(t, m.DumpFile(path))

	m2 := snapshot.NewManager(bc)
	require.NoError(t, m2.LoadFile(path))
	require.Len(t, m2.List(), 2)

	m3 := snapshot.NewManager(bc)
	m3.Restore([]snapshot.State{{ID: id1, Height: 0}})
	require.Len(t, m3.List(), 1)
	require.Equal(t, id1, m3.List()[0].ID)
}
