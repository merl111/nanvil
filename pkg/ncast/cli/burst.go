package cli

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/actor"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient/nep17"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/wallet"
	"github.com/urfave/cli/v2"
)

func burstCmd() *cli.Command {
	return &cli.Command{
		Name:  "burst",
		Usage: "Send many GAS transfers in parallel (load test)",
		Flags: []cli.Flag{
			wifFlag(),
			&cli.StringFlag{Name: "wif-alt", Usage: "Alternate WIF for ping-pong sends"},
			&cli.StringFlag{Name: "to", Required: true, Usage: "Primary receiver address"},
			&cli.StringFlag{Name: "to-alt", Usage: "Alternate receiver for ping-pong"},
			&cli.IntFlag{Name: "count", Aliases: []string{"n"}, Value: 100, Usage: "Number of transfers"},
			&cli.IntFlag{Name: "parallel", Aliases: []string{"p"}, Value: 16, Usage: "Concurrent workers"},
			&cli.BoolFlag{Name: "paced", Usage: "Wait for each tx to be mined before sending the next (per worker)"},
			&cli.StringFlag{Name: "amount", Value: "0.00000001", Usage: "GAS amount per transfer"},
			&cli.StringFlag{Name: "token", Aliases: []string{"t"}, Value: "gas", Usage: "Token to send"},
		},
		Action: func(ctx *cli.Context) error {
			count := ctx.Int("count")
			if count <= 0 {
				return fmt.Errorf("count must be positive")
			}
			parallel := ctx.Int("parallel")
			if parallel <= 0 {
				parallel = 1
			}
			if parallel > count {
				parallel = count
			}

			acc, err := accountFromWIF(ctx)
			if err != nil {
				return err
			}
			to, err := ncast.ResolveHash160(ctx.String("to"))
			if err != nil {
				return err
			}
			amount, err := ncast.ParseGAS(ctx.String("amount"))
			if err != nil {
				return err
			}

			var (
				accAlt   *wallet.Account
				toAlt    util.Uint160
				useAlt   bool
			)
			if ctx.IsSet("wif-alt") && ctx.IsSet("to-alt") {
				accAlt, err = wallet.NewAccountFromWIF(ctx.String("wif-alt"))
				if err != nil {
					return err
				}
				toAlt, err = ncast.ResolveHash160(ctx.String("to-alt"))
				if err != nil {
					return err
				}
				useAlt = true
			}

			endpoint := rpcEndpoint(ctx)
			cctx, cancel := context.WithTimeout(context.Background(), time.Duration(count)*200*time.Millisecond+120*time.Second)
			defer cancel()

			paced := ctx.Bool("paced")

			type worker struct {
				client *rpcclientHolder
				token  *nep17.Token
				from   util.Uint160
				dest   util.Uint160
			}
			type clientBundle struct {
				primary *worker
				alt     *worker
			}

			bundles := make([]*clientBundle, parallel)
			for i := range bundles {
				c, err := ncast.RPCClient(cctx, endpoint)
				if err != nil {
					return fmt.Errorf("worker %d: %w", i, err)
				}
				a, err := actor.NewSimple(c, acc)
				if err != nil {
					c.Close()
					return err
				}
				ctr, err := ncast.ResolveContract(c, ctx.String("token"))
				if err != nil {
					c.Close()
					return err
				}
				b := &clientBundle{
					primary: &worker{
						client: &rpcclientHolder{c},
						token:  nep17.New(a, ctr),
						from:   acc.Contract.ScriptHash(),
						dest:   to,
					},
				}
				if useAlt {
					aAlt, err := actor.NewSimple(c, accAlt)
					if err != nil {
						c.Close()
						return err
					}
					b.alt = &worker{
						client: &rpcclientHolder{c},
						token:  nep17.New(aAlt, ctr),
						from:   accAlt.Contract.ScriptHash(),
						dest:   toAlt,
					}
				}
				bundles[i] = b
			}
			defer func() {
				for _, b := range bundles {
					if b != nil && b.primary != nil && b.primary.client != nil {
						b.primary.client.Close()
					}
				}
			}()

			var (
				okCount  atomic.Int64
				failCount atomic.Int64
				wg       sync.WaitGroup
				jobs     = make(chan int, parallel*2)
			)

			start := time.Now()
			for w := 0; w < parallel; w++ {
				wg.Add(1)
				go func(bundle *clientBundle) {
					defer wg.Done()
					for i := range jobs {
						var wrk *worker
						if useAlt && i%2 == 0 {
							wrk = bundle.alt
						} else {
							wrk = bundle.primary
						}
						hash, _, err := wrk.token.Transfer(wrk.from, wrk.dest, amount, nil)
						if err != nil {
							failCount.Add(1)
							continue
						}
						okCount.Add(1)
						if paced {
							if err := waitTxMined(wrk.client.Client, hash, 3*time.Second); err != nil {
								failCount.Add(1)
								okCount.Add(-1)
							}
						}
					}
				}(bundles[w])
			}

			for i := 1; i <= count; i++ {
				jobs <- i
			}
			close(jobs)
			wg.Wait()
			elapsed := time.Since(start)

			out := map[string]any{
				"ok":       okCount.Load(),
				"failed":   failCount.Load(),
				"count":    count,
				"elapsed":  elapsed.String(),
				"parallel": parallel,
			}
			if elapsed > 0 {
				out["tx_per_sec"] = float64(okCount.Load()) / elapsed.Seconds()
			}
			if jsonOut(ctx) {
				return ncast.PrintJSON(out)
			}
			fmt.Printf("ok: %d / %d\n", okCount.Load(), count)
			if f := failCount.Load(); f > 0 {
				fmt.Printf("failed: %d\n", f)
			}
			fmt.Printf("elapsed: %s\n", elapsed.Round(time.Millisecond))
			if elapsed > 0 {
				fmt.Printf("throughput: %.1f tx/s\n", float64(okCount.Load())/elapsed.Seconds())
			}
			if failCount.Load() > 0 {
				return fmt.Errorf("%d transfers failed", failCount.Load())
			}
			return nil
		},
	}
}

// rpcclientHolder wraps *rpcclient.Client for burst worker cleanup tracking.
type rpcclientHolder struct {
	*rpcclient.Client
}

func waitTxMined(c *rpcclient.Client, hash util.Uint256, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		pool, err := c.GetRawMemPool()
		if err == nil {
			inPool := false
			for _, h := range pool {
				if h == hash {
					inPool = true
					break
				}
			}
			if !inPool {
				tx, err := c.GetRawTransactionVerbose(hash)
				if err == nil {
					if tx.VMState != "" && tx.VMState != "HALT" {
						return fmt.Errorf("tx %s vm state %s", hash.StringLE(), tx.VMState)
					}
					return nil
				}
			}
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", hash.StringLE())
}
