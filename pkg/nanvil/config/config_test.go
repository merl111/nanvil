package config_test

import (
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/stretchr/testify/require"
)

func TestDefaultStartOptions(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	require.Equal(t, "127.0.0.1", opts.Host)
	require.Equal(t, 8545, opts.Port)
	require.Equal(t, 10, opts.Accounts)
	require.True(t, opts.Explorer)
	require.NotEmpty(t, opts.Mnemonic)
}

func TestBlockchainConfig(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	cfg := nanvilcfg.BlockchainConfig(opts)
	require.Equal(t, netmode.NanvilNet, cfg.Magic)
	require.Equal(t, uint32(nanvilcfg.MaxTraceableBlocks), cfg.MaxTraceableBlocks)
	require.True(t, cfg.VerifyTransactions)
	require.True(t, cfg.MempoolSubscriptionsEnabled)
}

func TestBlockchainConfigForFork(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	m := &fork.Manifest{
		NetworkMagic: 860833102,
		Index:        42,
	}
	cfg := nanvilcfg.BlockchainConfigForFork(m, opts)
	require.Equal(t, netmode.Magic(860833102), cfg.Magic)
	require.True(t, cfg.Ledger.NanvilForkMode)
	require.True(t, cfg.Ledger.SkipBlockVerification)
	require.Equal(t, 15*time.Second, cfg.TimePerBlock)
}

func TestApplicationConfig(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	opts.NoMining = true
	cfg := nanvilcfg.ApplicationConfig(opts)
	require.True(t, cfg.Nanvil.Enabled)
	require.False(t, cfg.Nanvil.AutoMine)
	require.Equal(t, opts.Accounts, cfg.Nanvil.Accounts)
	require.True(t, cfg.RPC.MempoolSubscriptionsEnabled)
}
