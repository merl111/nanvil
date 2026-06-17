package cli

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/invoker"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/unwrap"
	"github.com/urfave/cli/v2"
)

func txCmd() *cli.Command {
	return &cli.Command{
		Name:      "tx",
		Usage:     "Get transaction by hash",
		ArgsUsage: "<hash>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "verbose", Aliases: []string{"v"}, Usage: "Include block metadata and application log"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast tx <hash>")
			}
			hash, err := ncast.ParseHash256(ctx.Args().Get(0))
			if err != nil {
				return err
			}
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				if ctx.Bool("verbose") {
					tx, err := h.Client.GetRawTransactionVerbose(hash)
					if err != nil {
						return nil, err
					}
					out := map[string]any{"transaction": tx}
					if log, lerr := h.Client.GetApplicationLog(hash, nil); lerr == nil {
						out["applicationlog"] = log
					}
					return out, nil
				}
				return h.Client.GetRawTransaction(hash)
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			return printTxSummary(res)
		},
	}
}

func balanceCmd() *cli.Command {
	return &cli.Command{
		Name:      "balance",
		Usage:     "Get NEP-17 token balance for an address",
		ArgsUsage: "<address>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "token", Aliases: []string{"t"}, Value: "gas", Usage: "Token name or contract hash"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast balance <address>")
			}
			addr, err := ncast.ResolveHash160(ctx.Args().Get(0))
			if err != nil {
				return err
			}
			bal, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				ctr, err := ncast.ResolveContract(h.Client, ctx.String("token"))
				if err != nil {
					return nil, err
				}
				inv := invoker.New(h.Client, nil)
				return unwrap.BigInt(inv.Call(ctr, "balanceOf", addr))
			})
			if err != nil {
				return err
			}
			amount := bal.(*big.Int)
			if jsonOut(ctx) {
				return ncast.PrintJSON(map[string]any{
					"address": ctx.Args().Get(0),
					"token":   ctx.String("token"),
					"balance": amount.String(),
					"gas":     ncast.FormatGASBig(amount),
				})
			}
			fmt.Println(ncast.FormatGASBig(amount))
			return nil
		},
	}
}

func printTxSummary(v any) error {
	if m, ok := v.(map[string]any); ok {
		data, _ := json.Marshal(m["transaction"])
		var fields map[string]any
		json.Unmarshal(data, &fields)
		ncast.PrintKV(
			"hash", fmt.Sprint(fields["hash"]),
			"sender", fmt.Sprint(fields["sender"]),
			"sysfee", formatFeeField(fields["sysfee"]),
			"netfee", formatFeeField(fields["netfee"]),
			"vub", fmt.Sprint(fields["validuntilblock"]),
			"block", fmt.Sprint(fields["blockhash"]),
			"vmstate", fmt.Sprint(fields["vmstate"]),
		)
		return nil
	}
	if tx, ok := v.(*result.TransactionOutputRaw); ok {
		sender := ""
		if len(tx.Signers) > 0 {
			sender = address.Uint160ToString(tx.Signers[0].Account)
		}
		ncast.PrintKV(
			"hash", tx.Hash().StringLE(),
			"sender", sender,
			"sysfee", ncast.FormatGAS(tx.SystemFee),
			"netfee", ncast.FormatGAS(tx.NetworkFee),
			"vub", fmt.Sprint(tx.ValidUntilBlock),
		)
		return nil
	}
	return ncast.PrintJSON(v)
}

func formatFeeField(v any) string {
	return ncast.FormatGAS(asInt(v))
}

func asInt(v any) int64 {
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	case string:
		var i int64
		fmt.Sscan(n, &i)
		return i
	default:
		return 0
	}
}
