package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/crypto/hash"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

func hashCmd() *cli.Command {
	return &cli.Command{
		Name:      "hash",
		Usage:     "Hash data (sha256, sha256ripemd160)",
		ArgsUsage: "<data>",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Value: "sha256", Usage: "sha256 | script"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast hash <hex-data>")
			}
			data, err := hex.DecodeString(trimHex(ctx.Args().Get(0)))
			if err != nil {
				return fmt.Errorf("invalid hex: %w", err)
			}
			switch ctx.String("type") {
			case "script":
				pub := data
				if len(pub) == 33 {
					h := hash.Hash160(pub)
					fmt.Println(h.StringLE())
					return nil
				}
				return fmt.Errorf("script hash requires 33-byte public key")
			default:
				sum := sha256.Sum256(data)
				fmt.Println(hex.EncodeToString(sum[:]))
			}
			return nil
		},
	}
}

func addressCmd() *cli.Command {
	return &cli.Command{
		Name:      "address",
		Usage:     "Convert between address and script hash",
		ArgsUsage: "<address|scripthash>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast address <address|scripthash>")
			}
			arg := ctx.Args().Get(0)
			if arg[0] == 'N' {
				u, err := address.StringToUint160(arg)
				if err != nil {
					return err
				}
				fmt.Println(u.StringLE())
				return nil
			}
			u, err := util.Uint160DecodeStringLE(trimHex(arg))
			if err != nil {
				return err
			}
			fmt.Println(address.Uint160ToString(u))
			return nil
		},
	}
}

func toDatoshiCmd() *cli.Command {
	return &cli.Command{
		Name:      "to-datoshi",
		Aliases:   []string{"to-wei"},
		Usage:     "Convert GAS amount to datoshi (8 decimals)",
		ArgsUsage: "<amount>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast to-datoshi <amount>")
			}
			amt, err := ncast.ParseGAS(ctx.Args().Get(0))
			if err != nil {
				return err
			}
			fmt.Println(amt.String())
			return nil
		},
	}
}

func fromDatoshiCmd() *cli.Command {
	return &cli.Command{
		Name:      "from-datoshi",
		Aliases:   []string{"from-wei"},
		Usage:     "Convert datoshi to GAS",
		ArgsUsage: "<datoshi>",
		Action: func(ctx *cli.Context) error {
			if ctx.NArg() < 1 {
				return fmt.Errorf("usage: ncast from-datoshi <datoshi>")
			}
			var n int64
			if _, err := fmt.Sscan(ctx.Args().Get(0), &n); err != nil {
				return err
			}
			fmt.Println(ncast.FormatGAS(n))
			return nil
		},
	}
}
