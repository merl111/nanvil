package cli

import (
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli/v2"
)

func estimateCmd() *cli.Command {
	return &cli.Command{
		Name:    "estimate",
		Aliases: []string{"gas"},
		Usage:   "Estimate fees for a contract call",
		ArgsUsage: "<contract> <method> [args...]",
		Flags: []cli.Flag{
			wifFlag(),
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 2 {
				return fmt.Errorf("usage: ncast estimate <contract> <method> [args...]")
			}
			contract := ctx.Args().Get(0)
			method := ctx.Args().Get(1)
			args, err := parseCallArgs(ctx.Args().Slice()[2:])
			if err != nil {
				return err
			}
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				ctr, err := ncast.ResolveContract(h.Client, contract)
				if err != nil {
					return nil, err
				}
				inv := invoker.New(h.Client, nil)
				sim, err := inv.Call(ctr, method, args...)
				if err != nil {
					return nil, err
				}
				out := map[string]any{
					"state":       sim.State,
					"gasconsumed": sim.GasConsumed,
					"exception":   sim.FaultException,
				}
				wif := ctx.String("wif")
				if wif != "" {
					acc, err := wallet.NewAccountFromWIF(wif)
					if err != nil {
						return nil, err
					}
					a, err := actor.NewSimple(h.Client, acc)
					if err != nil {
						return nil, err
					}
					tx, err := a.MakeCall(ctr, method, args...)
					if err != nil {
						return nil, err
					}
					netFee, err := h.Client.CalculateNetworkFee(tx)
					if err != nil {
						return nil, err
					}
					out["networkfee"] = netFee
					out["sysfee"] = tx.SystemFee
				}
				return out, nil
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			m := res.(map[string]any)
			fmt.Printf("state: %v\n", m["state"])
			fmt.Printf("gas consumed: %v\n", m["gasconsumed"])
			if nf, ok := m["networkfee"]; ok {
				fmt.Printf("network fee: %s\n", ncast.FormatGAS(nf.(int64)))
			}
			if sf, ok := m["sysfee"]; ok {
				fmt.Printf("system fee: %s\n", ncast.FormatGAS(sf.(int64)))
			}
			if ex, ok := m["exception"].(string); ok && ex != "" {
				fmt.Printf("exception: %s\n", ex)
			}
			return nil
		},
	}
}
