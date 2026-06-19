package integration_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/stretchr/testify/require"
)

func TestDevNodeDumpState(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "chain.state.json")

	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.Explorer = false
	opts.NoMining = true
	opts.Accounts = 2
	opts.DumpState = statePath

	dev, err := node.NewDevNode(opts, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, dev.Start(ctx))
	defer dev.Shutdown()

	dev.DumpStateInterval(ctx, 50*time.Millisecond, statePath)
	time.Sleep(150 * time.Millisecond)
}
