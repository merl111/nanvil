package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/management"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

func deployCmd() *cli.Command {
	return &cli.Command{
		Name:    "deploy",
		Aliases: []string{"publish"},
		Usage:   "Deploy a smart contract from NEF and manifest files",
		Flags: []cli.Flag{
			wifFlag(),
			&cli.StringFlag{Name: "nef", Aliases: []string{"i"}, Required: true, Usage: "Path to contract .nef file"},
			&cli.StringFlag{Name: "manifest", Aliases: []string{"m"}, Required: true, Usage: "Path to contract .manifest.json file"},
			&cli.StringFlag{Name: "data", Usage: "Optional deploy data (JSON value passed to _deploy)"},
			&cli.BoolFlag{Name: "wait", Value: true, Usage: "Wait until the transaction is included in a block"},
			&cli.DurationFlag{Name: "timeout", Value: 30 * time.Second, Usage: "Timeout when waiting for confirmation"},
		},
		Action: func(ctx *cli.Context) error {
			acc, err := accountFromWIF(ctx)
			if err != nil {
				return err
			}
			nefFile, err := readNEFFile(ctx.String("nef"))
			if err != nil {
				return err
			}
			manif, err := readManifestFile(ctx.String("manifest"))
			if err != nil {
				return err
			}
			var data any
			if ctx.IsSet("data") {
				if err := json.Unmarshal([]byte(ctx.String("data")), &data); err != nil {
					return fmt.Errorf("invalid --data JSON: %w", err)
				}
			}
			contractHash := state.CreateContractHash(acc.Contract.ScriptHash(), nefFile.Checksum, manif.Name)

			var (
				txHash util.Uint256
				vub    uint32
			)
			_, err = ncast.ResolveClientHolder(ctx, func(h *ncast.RPCClientHolder) (any, error) {
				a, err := actor.NewSimple(h.Client, acc)
				if err != nil {
					return nil, err
				}
				mgmt := management.New(a)
				txHash, vub, err = mgmt.Deploy(nefFile, manif, data)
				if err != nil {
					return nil, err
				}
				if ctx.Bool("wait") {
					if err := waitForTx(h.Client, txHash, ctx.Duration("timeout")); err != nil {
						return nil, err
					}
				}
				return nil, nil
			})
			if err != nil {
				return err
			}
			out := map[string]any{
				"txhash":   txHash.StringLE(),
				"contract": contractHash.StringLE(),
				"vub":      vub,
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(out)
			}
			fmt.Printf("txhash: %s\n", txHash.StringLE())
			fmt.Printf("contract: %s\n", contractHash.StringLE())
			return nil
		},
	}
}

func readNEFFile(path string) (*nef.File, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read nef: %w", err)
	}
	f, err := nef.FileFromBytes(raw)
	if err != nil {
		return nil, fmt.Errorf("parse nef: %w", err)
	}
	return &f, nil
}

func readManifestFile(path string) (*manifest.Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	m := new(manifest.Manifest)
	if err := json.Unmarshal(raw, m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if err := m.IsValid(util.Uint160{}, true); err != nil {
		return nil, fmt.Errorf("invalid manifest: %w", err)
	}
	return m, nil
}

func waitForTx(c *rpcclient.Client, hash util.Uint256, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		tx, err := c.GetRawTransactionVerbose(hash)
		if err == nil && tx != nil && tx.Blockhash != (util.Uint256{}) {
			if tx.VMState != "" && tx.VMState != "HALT" {
				return fmt.Errorf("transaction %s failed with VM state %s", hash.StringLE(), tx.VMState)
			}
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for transaction %s", hash.StringLE())
}
