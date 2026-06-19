package config

import (
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
)

const (
	// DefaultRPCPort is the default nanvil JSON-RPC port (Anvil-compatible).
	DefaultRPCPort = 8545
	// DefaultExplorerPort is the default block explorer port.
	DefaultExplorerPort = 8546
	// DefaultHost is the default bind address.
	DefaultHost = "127.0.0.1"
	// MaxTraceableBlocks is the traceable window for historic RPC and snapshots.
	MaxTraceableBlocks = 100_000
)

// StartOptions configures a nanvil dev node.
type StartOptions struct {
	Host                   string
	Port                   int
	Accounts               int
	Balance                int64
	Mnemonic               string
	BlockTime              time.Duration
	MineEmptyBlocks        bool
	EmptyBlockInterval     time.Duration
	NoMining               bool
	AutoImpersonate        bool
	DisableBalanceChecks   bool
	PrintTraces            bool
	DumpState              string
	LoadState              string
	DataDir                string
	StateInterval          time.Duration
	ForkURL                string
	ForkBlock              uint32
	ForkCachePath          string
	NoStorageCaching       bool
	ForkTimeout            time.Duration
	ForkRetries            int
	ConfigPath             string
	Explorer               bool
	ExplorerHost           string
	ExplorerPort           int
	LogFormat              string
	LogLevel               string
}

// DefaultStartOptions returns Anvil-like defaults.
func DefaultStartOptions() StartOptions {
	return StartOptions{
		Host:         DefaultHost,
		Port:         DefaultRPCPort,
		Explorer:     true,
		ExplorerHost: DefaultHost,
		ExplorerPort: DefaultExplorerPort,
		Accounts:     10,
		Balance:      10_000_0000_0000,
		Mnemonic:     "test test test test test test test test test test test junk",
		LogFormat:    "text",
		LogLevel:     "info",
	}
}

// BlockchainConfig builds neo-go blockchain config for nanvil.
func BlockchainConfig(opts StartOptions) config.Blockchain {
	return localBlockchainConfig(opts)
}

// BlockchainConfigForFork builds blockchain config using fork manifest network settings.
func BlockchainConfigForFork(m *fork.Manifest, opts StartOptions) config.Blockchain {
	cfg := localBlockchainConfig(opts)
	cfg.Magic = netmode.Magic(m.NetworkMagic)
	cfg.Hardforks = hardforksForMagic(m.NetworkMagic)
	cfg.Ledger.SkipBlockVerification = true
	cfg.Ledger.NanvilForkMode = true
	cfg.ValidatorsCount = 1
	cfg.TimePerBlock = 15 * time.Second
	cfg.Genesis.TimePerBlock = 15 * time.Second
	return cfg
}

func localBlockchainConfig(opts StartOptions) config.Blockchain {
	cfg := config.Blockchain{
		ProtocolConfiguration: config.ProtocolConfiguration{
			Magic:                       netmode.NanvilNet,
			MaxTraceableBlocks:          MaxTraceableBlocks,
			MaxBlockSystemFee:           900000000000,
			MaxValidUntilBlockIncrement: MaxTraceableBlocks / 2,
			TimePerBlock:                1 * time.Second,
			Genesis: config.Genesis{
				TimePerBlock: 1 * time.Second,
			},
			ValidatorsCount:    1,
			VerifyTransactions: true,
			P2PSigExtensions:   true,
		Hardforks: map[string]uint32{
			"Aspidochelone": 1,
			"Basilisk":      2,
			"Cockatrice":    3,
			"Domovoi":       4,
			"Echidna":       5,
			"Faun":          6,
		},
		},
		Ledger: config.Ledger{
			KeepOnlyLatestState: false,
			SaveInvocations:     true,
		},
		MempoolSubscriptionsEnabled: true,
	}
	return cfg
}

func hardforksForMagic(magic uint32) map[string]uint32 {
	switch netmode.Magic(magic) {
	case netmode.MainNet:
		return map[string]uint32{
			"Aspidochelone": 1730000,
			"Basilisk":      4120000,
			"Cockatrice":    5450000,
			"Domovoi":       5570000,
			"Echidna":       7300000,
			"Faun":          8800000,
		}
	case netmode.TestNet:
		return map[string]uint32{
			"Aspidochelone": 0,
			"Basilisk":      2100000,
			"Cockatrice":    3967000,
			"Domovoi":       4144000,
			"Echidna":       5870000,
			"Faun":          12960000,
		}
	default:
		return map[string]uint32{
			"Aspidochelone": 1,
			"Basilisk":      2,
			"Cockatrice":    3,
			"Domovoi":       4,
			"Echidna":       5,
			"Faun":          6,
		}
	}
}

// ApplicationConfig builds application config for nanvil RPC server.
func ApplicationConfig(opts StartOptions) config.ApplicationConfiguration {
	autoMine := !opts.NoMining
	return config.ApplicationConfiguration{
		Ledger: config.Ledger{
			KeepOnlyLatestState: false,
			SaveInvocations:     true,
		},
		DBConfiguration: dbconfig.DBConfiguration{
			Type: "inmemory",
		},
		Relay: true,
		RPC: config.RPC{
			BasicService: config.BasicService{
				Enabled:   true,
				Addresses: []string{opts.Host + ":0"},
			},
			MaxGasInvoke:                fixedn.Fixed8FromInt64(15),
			SessionEnabled:              true,
			SessionLifetime:             30 * time.Second,
			MempoolSubscriptionsEnabled: true,
		},
		P2P: config.P2P{
			Addresses:  []string{":0"},
			MaxPeers:   0,
			MinPeers:   0,
			DialTimeout: 3 * time.Second,
		},
		Nanvil: config.NanvilConfiguration{
			Enabled:                  true,
			ImpersonationEnabled:     true,
			DisablePoolBalanceChecks: opts.DisableBalanceChecks,
			PrintTraces:              opts.PrintTraces,
			AutoMine:                 autoMine,
			BlockTimeSeconds:         uint32(opts.BlockTime.Seconds()),
			MineEmptyBlocks:          opts.MineEmptyBlocks,
			EmptyBlockIntervalSeconds: uint32(opts.EmptyBlockInterval.Seconds()),
			Accounts:                 opts.Accounts,
			Balance:                  opts.Balance,
			Mnemonic:                 opts.Mnemonic,
		},
	}
}
