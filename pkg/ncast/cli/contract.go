package cli

import (
	"encoding/hex"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

func storageCmd() *cli.Command {
	return &cli.Command{
		Name:      "storage",
		Usage:     "Read contract storage at a key",
		ArgsUsage: "<contract> <key-hex>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 2 {
				return fmt.Errorf("usage: ncast storage <contract> <key-hex>")
			}
			keyHex := ctx.Args().Get(1)
			key, err := hex.DecodeString(trimHex(keyHex))
			if err != nil {
				return fmt.Errorf("invalid key hex: %w", err)
			}
			val, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				ctr, err := ncast.ResolveContract(h.Client, ctx.Args().Get(0))
				if err != nil {
					return nil, err
				}
				return h.Client.GetStorageByHash(ctr, key)
			})
			if err != nil {
				return err
			}
			data := val.([]byte)
			if jsonOut(ctx) {
				return ncast.PrintJSON(map[string]string{"value": "0x" + hex.EncodeToString(data)})
			}
			fmt.Printf("0x%x\n", data)
			return nil
		},
	}
}

func contractCmd() *cli.Command {
	return &cli.Command{
		Name:      "contract",
		Aliases:   []string{"code"},
		Usage:     "Get contract state",
		ArgsUsage: "<contract>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast contract <contract>")
			}
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				ctr, err := ncast.ResolveContract(h.Client, ctx.Args().Get(0))
				if err != nil {
					return nil, err
				}
				return h.Client.GetContractStateByHash(ctr)
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			cs := res.(*state.Contract)
			ncast.PrintKV(
				"name", cs.Manifest.Name,
				"hash", cs.Hash.StringLE(),
				"id", fmt.Sprint(cs.ID),
				"update", fmt.Sprint(cs.UpdateCounter),
			)
			return nil
		},
	}
}

func mempoolCmd() *cli.Command {
	return &cli.Command{
		Name:    "mempool",
		Aliases: []string{"tx-pending"},
		Usage:   "List pending transaction hashes",
		Action: func(ctx *cli.Context) error {
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				return h.Client.GetRawMemPool()
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			for _, h := range res.([]util.Uint256) {
				fmt.Println(h.StringLE())
			}
			return nil
		},
	}
}

func chainIDCmd() *cli.Command {
	return &cli.Command{
		Name:    "chain-id",
		Aliases: []string{"version"},
		Usage:   "Get network magic and node version",
		Action: func(ctx *cli.Context) error {
			res, err := ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				return h.Client.GetVersion()
			})
			if err != nil {
				return err
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(res)
			}
			v := res.(*result.Version)
			fmt.Printf("network: %d (0x%x)\n", v.Protocol.Network, uint32(v.Protocol.Network))
			fmt.Printf("useragent: %s\n", v.UserAgent)
			fmt.Printf("height interval: %dms\n", v.Protocol.MillisecondsPerBlock)
			return nil
		},
	}
}

func trimHex(s string) string {
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		return s[2:]
	}
	return s
}
