package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/explorer"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/impersonate"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/logging"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/persist"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/producer"
	nanvilrpc "github.com/nspcc-dev/neo-go/pkg/nanvil/rpc"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/nspcc-dev/neo-go/pkg/network"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"go.uber.org/zap"
)

// DevNode is a nanvil development node.
type DevNode struct {
	Opts      nanvilcfg.StartOptions
	Chain     *core.Blockchain
	NetServer *network.Server
	RPCServer *rpcsrv.Server
	accMgr    *accounts.Manager
	prod      *producer.Producer
	builder   *producer.BlockBuilder
	SnapMgr   *snapshot.Manager
	Fork      *fork.Manifest
	Remote    *fork.RemoteStateStore
	Overlay   *fork.TrackingOverlay
	forkStore *fork.Store
	baseStore *storage.MemoryStore
	loadedFromSnapshot bool
	RPCAddr      string
	Explorer     *explorer.Server
	ExplorerAddr string
	log          *zap.Logger
	errChan      chan error
}

// NewDevNode constructs but does not start a dev node.
func NewDevNode(opts nanvilcfg.StartOptions, log *zap.Logger) (*DevNode, error) {
	if log == nil {
		log = zap.NewNop()
	}
	accs, err := accounts.NewManager(opts.Mnemonic, opts.Accounts)
	if err != nil {
		return nil, err
	}
	accounts.RegisterVerificationScripts(accs.Accounts)
	pubHex, err := accounts.ValidatorPublicKeyHex()
	if err != nil {
		return nil, err
	}
	bcCfg := nanvilcfg.BlockchainConfig(opts)
	bcCfg.StandbyCommittee = []string{pubHex}
	appCfg := nanvilcfg.ApplicationConfig(opts)

	var manifest *fork.Manifest
	var chainSnap *persist.ChainSnapshot
	if opts.LoadState != "" {
		if snap, err := persist.TryLoad(opts.LoadState); err != nil {
			return nil, fmt.Errorf("load chain state: %w", err)
		} else if snap != nil {
			chainSnap = snap
			manifest = snap.Fork
		} else if m, err := fork.LoadManifest(opts.LoadState); err == nil {
			manifest = m
		} else if m, err := fork.LoadManifest(opts.LoadState + ".fork.json"); err == nil {
			manifest = m
		}
	}
	forkMode := opts.ForkURL != "" || (manifest != nil && manifest.RPCURL != "")
	if forkMode && manifest == nil {
		m, err := fork.CreateBranch(context.Background(), opts.ForkURL, opts.ForkBlock)
		if err != nil {
			return nil, fmt.Errorf("fork manifest: %w", err)
		}
		manifest = m
	}

	overlay := fork.NewTrackingOverlay()
	baseStore := storage.NewMemoryStore()
	if chainSnap != nil {
		if err := persist.Apply(chainSnap, baseStore, overlay); err != nil {
			return nil, fmt.Errorf("restore chain state: %w", err)
		}
	}
	var forkStore *fork.Store
	store := storage.Store(baseStore)
	if forkMode {
		forkStore = fork.NewStore(baseStore, overlay)
		store = forkStore
		if manifest != nil {
			bcCfg = nanvilcfg.BlockchainConfigForFork(manifest, opts)
			bcCfg.StandbyCommittee = []string{pubHex}
		}
	}

	bc, err := core.NewBlockchain(store, bcCfg, log)
	if err != nil {
		return nil, err
	}

	builder := producer.NewBlockBuilder(bc, accs.Validator, logging.IsText(opts.LogFormat), log)
	autoMine := !opts.NoMining
	prod := producer.NewProducer(builder, autoMine, opts.BlockTime, opts.MineEmptyBlocks, opts.EmptyBlockInterval, log)

	n := &DevNode{
		Opts:    opts,
		Chain:   bc,
		accMgr:  accs,
		prod:    prod,
		builder: builder,
		SnapMgr: snapshot.NewManager(bc),
		Fork:      manifest,
		Overlay:   overlay,
		forkStore: forkStore,
		baseStore: baseStore,
		loadedFromSnapshot: chainSnap != nil,
		log:       log,
		errChan:   make(chan error, 1),
		RPCAddr:   fmt.Sprintf("%s:%d", opts.Host, opts.Port),
	}
	if chainSnap != nil && len(chainSnap.Snapshots) > 0 {
		n.SnapMgr.Restore(chainSnap.Snapshots)
	}

	nanvilrpc.RegisterHandlers(n)
	nanvilrpc.RegisterTransactionTracking()
	impersonate.Global().SetEnabled(true)
	if opts.AutoImpersonate {
		impersonate.Global().SetAutoMode(true)
	}

	cfg := config.Config{
		ProtocolConfiguration:    bcCfg.ProtocolConfiguration,
		ApplicationConfiguration: appCfg,
	}
	serverConfig, err := network.NewServerConfig(cfg)
	if err != nil {
		bc.Close()
		return nil, err
	}
	serverConfig.Addresses = []config.AnnounceableAddress{{Address: ":0"}}
	netSrv, err := network.NewServer(serverConfig, bc, bc.GetStateSyncModule(), log)
	if err != nil {
		bc.Close()
		return nil, err
	}
	n.NetServer = netSrv
	netSrv.SetOnTransactionRelayed(n.prod.OnTransactionRelayed)

	rpcCfg := appCfg.RPC
	rpcCfg.BasicService = config.BasicService{
		Enabled:   true,
		Addresses: []string{n.RPCAddr},
	}
	n.RPCServer = rpcsrv.New(bc, rpcCfg, netSrv, nil, log, n.errChan)
	nanvilrpc.SetServerContext(n)

	return n, nil
}

// Start runs the dev node.
func (n *DevNode) Start(ctx context.Context) error {
	skipInit := n.loadedFromSnapshot && n.Chain.BlockHeight() > 0
	if n.Fork != nil {
		if n.Opts.ForkURL == "" && n.Fork.RPCURL != "" {
			n.Opts.ForkURL = n.Fork.RPCURL
		}
		if err := n.initFork(ctx); err != nil {
			return fmt.Errorf("fork init: %w", err)
		}
		if n.forkStore != nil {
			n.forkStore.SetRemote(n.Remote)
		}
		if !skipInit {
			if err := fork.Bootstrap(ctx, fork.BootstrapOptions{
				Manifest: n.Fork,
				Remote:   n.Remote,
				Overlay:  n.Overlay,
				Chain:    n.Chain,
				Accounts: n.accMgr,
				Balance:  n.Opts.Balance,
			}); err != nil {
				return fmt.Errorf("fork bootstrap: %w", err)
			}
		}
		n.logFundedAccounts()
	}

	go n.Chain.Run()

	if n.Fork == nil && !skipInit {
		if err := n.fundDevAccounts(); err != nil {
			return fmt.Errorf("fund accounts: %w", err)
		}
		n.logFundedAccounts()
	}

	n.NetServer.Start()
	n.RPCServer.Start()
	if addrs := n.RPCServer.Addresses(); len(addrs) > 0 {
		n.RPCAddr = addrs[0]
	}
	n.prod.Start(ctx)

	if n.Opts.Explorer {
		n.Explorer = explorer.New(n.RPCAddr, n.Opts.ExplorerHost, n.Opts.ExplorerPort, n.log)
		if err := n.Explorer.Start(); err != nil {
			return fmt.Errorf("explorer: %w", err)
		}
		n.ExplorerAddr = n.Explorer.Addr()
	}

	if n.Opts.LoadState != "" && n.SnapMgr != nil && !n.loadedFromSnapshot {
		if err := n.SnapMgr.LoadFile(n.Opts.LoadState); err != nil {
			n.log.Warn("load state file", zap.Error(err))
		}
	}

	if logging.IsText(n.Opts.LogFormat) {
		fmt.Fprintf(os.Stdout, "\nNanvil listening on http://%s\n", n.RPCAddr)
		fmt.Fprintf(os.Stdout, "Chain height: %d (%s)\n", n.Chain.BlockHeight(), netmode.Magic(n.Chain.GetConfig().Magic).String())
		if n.ExplorerAddr != "" {
			fmt.Fprintf(os.Stdout, "Explorer: http://%s\n", n.ExplorerAddr)
		}
	} else {
		fields := []zap.Field{
			zap.String("rpc", "http://"+n.RPCAddr),
			zap.Uint32("height", n.Chain.BlockHeight()),
			zap.String("magic", netmode.Magic(n.Chain.GetConfig().Magic).String()),
			zap.Int("accounts", len(n.accMgr.Accounts)),
		}
		if n.ExplorerAddr != "" {
			fields = append(fields, zap.String("explorer", "http://"+n.ExplorerAddr))
		}
		n.log.Info("nanvil started", fields...)
	}
	return nil
}

// Shutdown stops the dev node.
func (n *DevNode) Shutdown() {
	n.prod.Stop()
	if n.Explorer != nil {
		n.Explorer.Shutdown()
	}
	if n.RPCServer != nil {
		n.RPCServer.Shutdown()
	}
	if n.NetServer != nil {
		n.NetServer.Shutdown()
	}
	if n.Remote != nil {
		n.Remote.Close()
	}
	if n.Chain != nil {
		if n.Opts.DumpState != "" {
			if err := n.Chain.Flush(); err != nil {
				n.log.Warn("flush chain before dump", zap.Error(err))
			}
			if err := n.dumpState(); err != nil {
				n.log.Warn("dump state", zap.Error(err))
			}
		}
		n.Chain.Close()
	}
}

func (n *DevNode) fundDevAccounts() error {
	return n.accMgr.FundAll(n.Chain, n.Opts.Balance, func(txs ...*transaction.Transaction) error {
		_, err := n.builder.Mine(txs...)
		return err
	})
}

func (n *DevNode) logFundedAccounts() {
	n.accMgr.PrintStartupInfo(os.Stdout, n.Opts.Balance, n.Opts.Mnemonic)
	if logging.IsText(n.Opts.LogFormat) {
		return
	}
	for _, acc := range n.accMgr.Accounts {
		n.log.Info("dev account funded",
			zap.Int("index", acc.Index),
			zap.String("address", acc.Address),
			zap.String("wif", acc.WIF),
			zap.Int64("balance", n.Opts.Balance),
			zap.String("balanceGas", accounts.FormatGAS(n.Opts.Balance)),
		)
	}
	n.log.Info("validator account",
		zap.String("address", address.Uint160ToString(n.accMgr.Validator.ScriptHash())),
		zap.String("wif", accounts.ValidatorWIF()),
	)
}

func (n *DevNode) initFork(ctx context.Context) error {
	var (
		manifest *fork.Manifest
		err      error
	)
	cacheBase := n.Opts.ForkCachePath
	if cacheBase == "" {
		cacheBase = filepath.Join(os.TempDir(), "nanvil-cache")
	}
	if n.Fork != nil && n.Fork.RPCURL == n.Opts.ForkURL {
		manifest = n.Fork
	} else {
		manifest, err = fork.CreateBranch(ctx, n.Opts.ForkURL, n.Opts.ForkBlock)
		if err != nil {
			return err
		}
		n.Fork = manifest
	}
	cache, err := fork.NewDiskCache(cacheBase, manifest.NetworkMagic, manifest.Index)
	if err != nil {
		return err
	}
	n.Remote, err = fork.NewRemoteStateStore(ctx, manifest, cache, n.Opts.NoStorageCaching)
	if err != nil {
		return err
	}
	if logging.IsText(n.Opts.LogFormat) {
		fmt.Fprintf(os.Stderr, "Fork ready at block %d (root %s, %d contracts)\n",
			manifest.Index, manifest.RootHash.StringLE(), len(manifest.Contracts))
	} else {
		n.log.Info("fork ready",
			zap.Uint32("branch", manifest.Index),
			zap.Stringer("root", manifest.RootHash),
			zap.Int("contracts", len(manifest.Contracts)),
		)
	}
	return nil
}

func (n *DevNode) dumpState() error {
	path := n.Opts.DumpState
	if n.baseStore == nil {
		return fmt.Errorf("dump state: no backing store")
	}
	return persist.Save(path, n.Chain.BlockHeight(), n.SnapMgr.List(), n.baseStore, n.Overlay, n.Fork)
}

// MineBlock mines up to count blocks, draining the mempool into each block.
func (n *DevNode) MineBlock(count int) error {
	if count <= 0 {
		count = 1
	}
	for range count {
		if _, err := n.builder.Mine(); err != nil {
			return err
		}
	}
	return nil
}

// ResetChain resets to genesis or fork branch height.
func (n *DevNode) ResetChain() error {
	height := uint32(0)
	if n.Fork != nil {
		height = n.Fork.Index
	}
	n.prod.Stop()
	n.Chain.Close()
	// Re-open not implemented; reset in-place requires non-running chain.
	// For dev, mine empty blocks back is expensive; use snapshot revert instead.
	return fmt.Errorf("reset to height %d requires node restart; use nanvil_snapshot/revert", height)
}

// RevertSnapshot reverts to a snapshot (best-effort while running).
func (n *DevNode) RevertSnapshot(id snapshot.ID) error {
	return n.SnapMgr.Revert(id)
}

// DumpStateInterval starts periodic state dumps.
func (n *DevNode) DumpStateInterval(ctx context.Context, interval time.Duration, path string) {
	if interval <= 0 || path == "" {
		return
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := n.Chain.Flush(); err != nil {
					n.log.Warn("flush chain before periodic dump", zap.Error(err))
				}
				_ = n.dumpState()
			}
		}
	}()
}
