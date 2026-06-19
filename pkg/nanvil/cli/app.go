package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/logging"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/persist"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

// NewApp creates the nanvil CLI application.
func NewApp() *cli.App {
	app := &cli.App{
		Name:  "nanvil",
		Usage: "Neo3 local development node (Anvil-compatible)",
		Commands: []*cli.Command{
			{
				Name:  "start",
				Usage: "Start the nanvil dev node",
				Flags: startFlags(),
				Action: func(c *cli.Context) error {
					return runStart(c)
				},
			},
			{
				Name:  "fork",
				Usage: "Fork management commands",
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "Create a fork manifest from remote RPC",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "rpc-url", Required: true},
							&cli.UintFlag{Name: "block", Usage: "branch block (0 = latest state)"},
							&cli.StringFlag{Name: "out", Value: "fork.json"},
						},
						Action: func(c *cli.Context) error {
							ctx := c.Context
							if ctx == nil {
								ctx = context.Background()
							}
							m, err := fork.CreateBranch(ctx, c.String("rpc-url"), uint32(c.Uint("block")))
							if err != nil {
								return err
							}
							if err := m.Save(c.String("out")); err != nil {
								return err
							}
							fmt.Printf("Fork manifest saved to %s (height %d, %d contracts)\n", c.String("out"), m.Index, len(m.Contracts))
							return nil
						},
					},
					{
						Name:  "prefetch",
						Usage: "Prefetch contract storage from remote fork",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "manifest", Required: true},
							&cli.StringFlag{Name: "contract", Required: true},
							&cli.StringFlag{Name: "cache-path"},
						},
						Action: func(c *cli.Context) error {
							ctx := c.Context
							if ctx == nil {
								ctx = context.Background()
							}
							m, err := fork.LoadManifest(c.String("manifest"))
							if err != nil {
								return err
							}
							cachePath := c.String("cache-path")
							if cachePath == "" {
								cachePath = os.TempDir() + "/nanvil-cache"
							}
							cache, err := fork.NewDiskCache(cachePath, m.NetworkMagic, m.Index)
							if err != nil {
								return err
							}
							rs, err := fork.NewRemoteStateStore(ctx, m, cache, false)
							if err != nil {
								return err
							}
							defer rs.Close()
							h, err := parseContractHash(c.String("contract"))
							if err != nil {
								return err
							}
							if err := rs.PrefetchContract(h); err != nil {
								return err
							}
							fmt.Printf("Prefetched contract %s (%d cache entries)\n", h.StringBE(), rs.CachedCount())
							return nil
						},
					},
					{
						Name:  "info",
						Usage: "Show fork manifest info",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "manifest", Required: true},
						},
						Action: func(c *cli.Context) error {
							m, err := fork.LoadManifest(c.String("manifest"))
							if err != nil {
								return err
							}
							fmt.Printf("RPC: %s\nHeight: %d\nRoot: %s\nContracts: %d\n", m.RPCURL, m.Index, m.RootHash.StringLE(), len(m.Contracts))
							return nil
						},
					},
				},
			},
			{
				Name:  "policy",
				Usage: "Policy utilities",
				Subcommands: []*cli.Command{
					{
						Name:  "sync",
						Usage: "Fetch remote policy settings (read-only display)",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "rpc-url", Required: true},
						},
						Action: func(c *cli.Context) error {
							fmt.Println("Policy sync: connect to remote and compare Policy native contract settings manually via invokefunction")
							fmt.Println("Remote RPC:", c.String("rpc-url"))
							return nil
						},
					},
				},
			},
		},
	}
	return app
}

func startFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "host", Value: nanvilcfg.DefaultHost},
		&cli.IntFlag{Name: "port", Value: nanvilcfg.DefaultRPCPort},
		&cli.IntFlag{Name: "accounts", Value: 10},
		&cli.Int64Flag{Name: "balance", Value: 10_000_0000_0000},
		&cli.StringFlag{Name: "mnemonic", Value: "test test test test test test test test test test test junk"},
		&cli.DurationFlag{Name: "block-time", Usage: "block interval; mines empty blocks too unless --no-mine-empty (0 = mine on tx)"},
		&cli.BoolFlag{Name: "no-mine-empty", Usage: "with --block-time, only mine when mempool has transactions"},
		&cli.DurationFlag{Name: "empty-block-interval", Usage: "mine empty blocks on interval when idle (use with block-time=0)"},
		&cli.BoolFlag{Name: "no-mining"},
		&cli.BoolFlag{Name: "auto-impersonate", Usage: "auto-impersonate transaction signers (default on for forks)"},
		&cli.BoolFlag{Name: "print-traces"},
		&cli.StringFlag{Name: "dump-state"},
		&cli.StringFlag{Name: "load-state"},
		&cli.StringFlag{Name: "data-dir", Usage: "persistent chain directory (auto load/dump chain.state.json)"},
		&cli.DurationFlag{Name: "state-interval"},
		&cli.StringFlag{Name: "fork-url", Aliases: []string{"rpc-url"}},
		&cli.UintFlag{Name: "fork-block-number"},
		&cli.StringFlag{Name: "fork-cache-path"},
		&cli.BoolFlag{Name: "no-storage-caching"},
		&cli.BoolFlag{Name: "with-explorer", Value: true, Usage: "enable the block explorer UI"},
		&cli.BoolFlag{Name: "no-explorer", Usage: "disable the block explorer UI"},
		&cli.StringFlag{Name: "explorer-host", Usage: "explorer bind host"},
		&cli.IntFlag{Name: "explorer-port", Usage: "explorer port", Value: nanvilcfg.DefaultExplorerPort},
		&cli.StringFlag{Name: "log-format", Usage: "log output format: text (human-readable) or json", Value: "text"},
		&cli.StringFlag{Name: "log-level", Usage: "log level: debug, info, warn, error", Value: "info"},
	}
}

func runStart(c *cli.Context) error {
	opts := nanvilcfg.DefaultStartOptions()
	opts.Host = c.String("host")
	opts.Port = c.Int("port")
	opts.Accounts = c.Int("accounts")
	opts.Balance = c.Int64("balance")
	opts.Mnemonic = c.String("mnemonic")
	opts.BlockTime = c.Duration("block-time")
	opts.MineEmptyBlocks = opts.BlockTime > 0 && !c.Bool("no-mine-empty")
	opts.EmptyBlockInterval = c.Duration("empty-block-interval")
	opts.NoMining = c.Bool("no-mining")
	opts.AutoImpersonate = c.Bool("auto-impersonate")
	if isForkStart(c, opts) && !c.IsSet("auto-impersonate") {
		opts.AutoImpersonate = true
	}
	opts.PrintTraces = c.Bool("print-traces")
	opts.DumpState = c.String("dump-state")
	opts.LoadState = c.String("load-state")
	opts.DataDir = c.String("data-dir")
	if opts.DataDir != "" {
		if err := os.MkdirAll(opts.DataDir, 0o755); err != nil {
			return fmt.Errorf("create data dir: %w", err)
		}
		statePath := filepath.Join(opts.DataDir, "chain.state.json")
		if opts.DumpState == "" {
			opts.DumpState = statePath
		}
		if opts.LoadState == "" {
			opts.LoadState = statePath
		}
	}
	opts.StateInterval = c.Duration("state-interval")
	opts.ForkURL = c.String("fork-url")
	opts.ForkBlock = uint32(c.Uint("fork-block-number"))
	opts.ForkCachePath = c.String("fork-cache-path")
	opts.NoStorageCaching = c.Bool("no-storage-caching")
	opts.Explorer = c.Bool("with-explorer")
	if c.Bool("no-explorer") {
		opts.Explorer = false
	}
	if c.IsSet("explorer-host") {
		opts.ExplorerHost = c.String("explorer-host")
	} else {
		opts.ExplorerHost = opts.Host
	}
	opts.ExplorerPort = c.Int("explorer-port")
	opts.LogFormat = c.String("log-format")
	opts.LogLevel = c.String("log-level")

	log, err := logging.New(opts.LogFormat, opts.LogLevel)
	if err != nil {
		return err
	}
	defer log.Sync() //nolint:errcheck

	dev, err := node.NewDevNode(opts, log)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := dev.Start(ctx); err != nil {
		return err
	}
	dev.DumpStateInterval(ctx, opts.StateInterval, opts.DumpState)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	dev.Shutdown()
	return nil
}

func isForkStart(c *cli.Context, opts nanvilcfg.StartOptions) bool {
	if c.String("fork-url") != "" {
		return true
	}
	if opts.LoadState != "" {
		if snap, err := persist.TryLoad(opts.LoadState); err == nil && snap != nil && snap.Fork != nil {
			return true
		}
		if m, err := fork.LoadManifest(opts.LoadState); err == nil && m.RPCURL != "" {
			return true
		}
		if m, err := fork.LoadManifest(opts.LoadState + ".fork.json"); err == nil && m.RPCURL != "" {
			return true
		}
	}
	return false
}

func parseContractHash(s string) (util.Uint160, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		return util.Uint160DecodeStringLE(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X"))
	}
	return address.StringToUint160(s)
}
