package cli

import (
	"context"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/urfave/cli/v2"
)

// NewApp creates the ncast CLI application.
func NewApp() *cli.App {
	app := &cli.App{
		Name:  "ncast",
		Usage: "Neo chain CLI (cast-like tool for nanvil/neo-go nodes)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "rpc",
				Aliases: []string{"r"},
				Value:   ncast.DefaultRPC,
				Usage:   "RPC endpoint URL",
				EnvVars: []string{"NCAST_RPC", "NANVIL_RPC"},
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output raw JSON",
			},
		},
		Commands: []*cli.Command{
			rpcCmd(),
			blockCmd(),
			blockNumberCmd(),
			txCmd(),
			balanceCmd(),
			sendCmd(),
			callCmd(),
			sendCallCmd(),
			estimateCmd(),
			storageCmd(),
			contractCmd(),
			deployCmd(),
			burstCmd(),
			mempoolCmd(),
			chainIDCmd(),
			hashCmd(),
			addressCmd(),
			toDatoshiCmd(),
			fromDatoshiCmd(),
			watchCmd(),
		},
	}
	return app
}

func rpcEndpoint(ctx *cli.Context) string {
	return ctx.String("rpc")
}

func jsonOut(ctx *cli.Context) bool {
	return ctx.Bool("json")
}

func withClient(ctx *cli.Context, fn func(context.Context, *cli.Context) error) error {
	cctx, cancel := context.WithTimeout(context.Background(), ctx.Duration("timeout"))
	defer cancel()
	return fn(cctx, ctx)
}

// satisfy unused
var _ = os.Stderr
