package snapshot_test

import (
	"path/filepath"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testChain(t *testing.T) *core.Blockchain {
	pub, err := accounts.ValidatorPublicKeyHex()
	require.NoError(t, err)
	cfg := config.Blockchain{
		ProtocolConfiguration: config.ProtocolConfiguration{
			Magic:              netmode.UnitTestNet,
			MaxTraceableBlocks: 1000,
			ValidatorsCount:    1,
			StandbyCommittee:   []string{pub},
		},
	}
	bc, err := core.NewBlockchain(storage.NewMemoryStore(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	return bc
}

func TestSnapshotManager(t *testing.T) {
	bc := testChain(t)
	go bc.Run()
	t.Cleanup(bc.Close)
	m := snapshot.NewManager(bc)
	id, err := m.Snapshot()
	require.NoError(t, err)
	require.NotEmpty(t, id)
	list := m.List()
	require.Len(t, list, 1)

	path := filepath.Join(t.TempDir(), "state.json")
	require.NoError(t, m.DumpFile(path))
}

func TestSnapshotRevertRequiresStoppedChain(t *testing.T) {
	bc := testChain(t)
	// chain not running — Reset is allowed
	m := snapshot.NewManager(bc)
	id, err := m.Snapshot()
	require.NoError(t, err)
	// chain not running - reset should work at same height
	err = m.Revert(id)
	require.NoError(t, err)
}
