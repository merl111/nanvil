package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/urfave/cli/v2"
)

func rpcCmd() *cli.Command {
	return &cli.Command{
		Name:      "rpc",
		Usage:     "Perform a raw JSON-RPC call",
		ArgsUsage: "<method> [params-json]",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast rpc <method> [params]")
			}
			method := ctx.Args().Get(0)
			var params []any
			if ctx.NArg() > 1 {
				if err := json.Unmarshal([]byte(ctx.Args().Get(1)), &params); err != nil {
					return fmt.Errorf("invalid params JSON: %w", err)
				}
			}
			raw, err := ncast.RawRPC(rpcEndpoint(ctx), method, params)
			if err != nil {
				return err
			}
			if jsonOut(ctx) || len(raw) == 0 {
				fmt.Println(string(raw))
				return nil
			}
			var pretty any
			if err := json.Unmarshal(raw, &pretty); err == nil {
				return ncast.PrintJSON(pretty)
			}
			fmt.Println(string(raw))
			return nil
		},
	}
}

func blockCmd() *cli.Command {
	return &cli.Command{
		Name:      "block",
		Usage:     "Get block by index or hash",
		ArgsUsage: "<index|hash>",
		Flags: []cli.Flag{
			&cli.BoolFlag{Name: "full", Aliases: []string{"f"}, Usage: "Include full transaction objects"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast block <index|hash>")
			}
			arg := ctx.Args().Get(0)
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				hashStr := arg
				if idx, perr := strconv.ParseUint(arg, 10, 32); perr == nil {
					hash, herr := h.Client.GetBlockHash(uint32(idx))
					if herr != nil {
						return nil, herr
					}
					hashStr = hash.StringLE()
				}
				hash, herr := ncast.ParseHash256(hashStr)
				if herr != nil {
					return nil, herr
				}
				if ctx.Bool("full") {
					return h.Client.GetBlockByHashVerbose(hash)
				}
				return h.Client.GetBlockByHash(hash)
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			return printBlockSummary(res)
		},
	}
}

func blockNumberCmd() *cli.Command {
	return &cli.Command{
		Name:    "block-number",
		Aliases: []string{"height"},
		Usage:   "Get current block height (latest block index)",
		Action: func(ctx *cli.Context) error {
			height, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				n, err := h.Client.GetBlockCount()
				if err != nil {
					return nil, err
				}
				if n == 0 {
					return uint32(0), nil
				}
				return n - 1, nil
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(height)
			}
			fmt.Println(height)
			return nil
		},
	}
}

func printBlockSummary(v any) error {
	if b, ok := v.(*result.Block); ok {
		ncast.PrintKV(
			"index", fmt.Sprint(b.Index),
			"hash", b.Hash().StringLE(),
			"previous", b.PrevHash.StringLE(),
			"timestamp", ncast.FormatTimeMs(b.Timestamp),
			"tx count", fmt.Sprint(len(b.Transactions)),
			"size", fmt.Sprint(b.Size),
		)
		return nil
	}
	return ncast.PrintJSON(v)
}
