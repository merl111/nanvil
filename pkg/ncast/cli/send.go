package cli

import (
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli/v2"
)

func wifFlag() cli.Flag {
	return &cli.StringFlag{
		Name:    "wif",
		Usage:   "WIF private key for signing",
		EnvVars: []string{"NCAST_WIF", "NANVIL_WIF"},
	}
}

func accountFromWIF(ctx *cli.Context) (*wallet.Account, error) {
	wif := ctx.String("wif")
	if wif == "" {
		return nil, fmt.Errorf("missing --wif (or NCAST_WIF env)")
	}
	return wallet.NewAccountFromWIF(wif)
}

func sendCmd() *cli.Command {
	return &cli.Command{
		Name:      "send",
		Usage:     "Send GAS (or NEP-17 token) to an address",
		ArgsUsage: "<to> <amount>",
		Flags: []cli.Flag{
			wifFlag(),
			&cli.StringFlag{Name: "token", Aliases: []string{"t"}, Value: "gas", Usage: "Token to send"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 2 {
				return fmt.Errorf("usage: ncast send --wif <wif> <to> <amount>")
			}
			acc, err := accountFromWIF(ctx)
			if err != nil {
				return err
			}
			to, err := ncast.ResolveHash160(ctx.Args().Get(0))
			if err != nil {
				return err
			}
			amount, err := ncast.ParseGAS(ctx.Args().Get(1))
			if err != nil {
				return err
			}
			var hash util.Uint256
			_, err = ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				a, err := actor.NewSimple(h.Client, acc)
				if err != nil {
					return nil, err
				}
				ctr, err := ncast.ResolveContract(h.Client, ctx.String("token"))
				if err != nil {
					return nil, err
				}
				token := nep17.New(a, ctr)
				hash, _, err = token.Transfer(acc.Contract.ScriptHash(), to, amount, nil)
				return hash, err
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(map[string]string{"txhash": hash.StringLE()})
			}
			fmt.Println(hash.StringLE())
			return nil
		},
	}
}

func callCmd() *cli.Command {
	return &cli.Command{
		Name:      "call",
		Usage:     "Call a read-only contract method",
		ArgsUsage: "<contract> <method> [args...]",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 2 {
				return fmt.Errorf("usage: ncast call <contract> <method> [args...]")
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
				return inv.Call(ctr, method, args...)
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			return printInvoke(res.(*result.Invoke))
		},
	}
}

func sendCallCmd() *cli.Command {
	return &cli.Command{
		Name:      "send-call",
		Aliases:   []string{"invoke"},
		Usage:     "Send a state-changing contract call",
		ArgsUsage: "<contract> <method> [args...]",
		Flags:     []cli.Flag{wifFlag()},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 2 {
				return fmt.Errorf("usage: ncast send-call --wif <wif> <contract> <method> [args...]")
			}
			acc, err := accountFromWIF(ctx)
			if err != nil {
				return err
			}
			contract := ctx.Args().Get(0)
			method := ctx.Args().Get(1)
			args, err := parseCallArgs(ctx.Args().Slice()[2:])
			if err != nil {
				return err
			}
			var hash util.Uint256
			_, err = ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				a, err := actor.NewSimple(h.Client, acc)
				if err != nil {
					return nil, err
				}
				ctr, err := ncast.ResolveContract(h.Client, contract)
				if err != nil {
					return nil, err
				}
				hash, _, err = a.SendCall(ctr, method, args...)
				return hash, err
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(map[string]string{"txhash": hash.StringLE()})
			}
			fmt.Println(hash.StringLE())
			return nil
		},
	}
}

func printInvoke(inv *result.Invoke) error {
	fmt.Printf("state: %s\n", inv.State)
	if inv.FaultException != "" {
		fmt.Printf("exception: %s\n", inv.FaultException)
	}
	if inv.GasConsumed != 0 {
		fmt.Printf("gas: %d\n", inv.GasConsumed)
	}
	for i, item := range inv.Stack {
		fmt.Printf("stack[%d]: %s\n", i, item)
	}
	return nil
}

func parseCallArgs(args []string) ([]any, error) {
	out := make([]any, len(args))
	for i, s := range args {
		v, err := parseCallArg(s)
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func parseCallArg(s string) (any, error) {
	if s == "true" {
		return true, nil
	}
	if s == "false" {
		return false, nil
	}
	if len(s) > 0 && s[0] == 'N' {
		u, err := ncast.ResolveHash160(s)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	if len(s) > 2 && (s[:2] == "0x" || s[:2] == "0X") {
		return s, nil
	}
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return n, nil
	}
	return s, nil
}
