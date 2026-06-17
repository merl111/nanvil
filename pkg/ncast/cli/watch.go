package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/urfave/cli/v2"
)

const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorCyan   = "\033[36m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorBold   = "\033[1m"
)

func watchCmd() *cli.Command {
	return &cli.Command{
		Name:    "watch",
		Aliases: []string{"live", "tail"},
		Usage:   "Live chain status — blocks, transactions, mempool in real time",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "lines", Aliases: []string{"n"}, Value: 20, Usage: "Number of recent events to show"},
		},
		Action: func(ctx *cli.Context) error {
			return runWatch(ctx)
		},
	}
}

type watchEvent struct {
	time    time.Time
	kind    string
	message string
}

func runWatch(ctx *cli.Context) error {
	endpoint := rpcEndpoint(ctx)
	wsURL, err := ncast.WSURL(endpoint)
	if err != nil {
		return err
	}

	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	wsc, err := rpcclient.NewWS(cctx, wsURL, rpcclient.WSOptions{})
	if err != nil {
		return fmt.Errorf("websocket: %w", err)
	}
	defer wsc.Close()
	if err := wsc.Init(); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	var (
		mu       sync.Mutex
		events   []watchEvent
		maxLines = ctx.Int("lines")
		height   uint32
		bestHash string
		mempoolN int
		network  string
		lastBlk  string
	)

	addEvent := func(kind, msg string) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, watchEvent{time: time.Now(), kind: kind, message: msg})
		if len(events) > maxLines {
			events = events[len(events)-maxLines:]
		}
	}

	refreshStats := func() {
		httpClient, err := ncast.RPCClient(cctx, endpoint)
		if err != nil {
			return
		}
		defer httpClient.Close()
		count, _ := httpClient.GetBlockCount()
		if count > 0 {
			height = count - 1
		}
		if h, err := httpClient.GetBestBlockHash(); err == nil {
			bestHash = ncast.ShortHash(h.StringLE(), 6)
		}
		if mp, err := httpClient.GetRawMemPool(); err == nil {
			mempoolN = len(mp)
		}
		if v, err := httpClient.GetVersion(); err == nil {
			network = fmt.Sprintf("%d", v.Protocol.Network)
		}
	}

	refreshStats()

	blockCh := make(chan *block.Block, 8)
	txCh := make(chan *transaction.Transaction, 16)
	memCh := make(chan *result.MempoolEvent, 8)

	if _, err := wsc.ReceiveBlocks(nil, blockCh); err != nil {
		return fmt.Errorf("subscribe blocks: %w", err)
	}
	if _, err := wsc.ReceiveTransactions(nil, txCh); err != nil {
		return fmt.Errorf("subscribe txs: %w", err)
	}
	if _, err := wsc.ReceiveMempoolEvents(nil, memCh); err != nil {
		return fmt.Errorf("subscribe mempool: %w", err)
	}

	enterAltScreen()
	defer leaveAltScreen()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	redraw := func() {
		mu.Lock()
		defer mu.Unlock()
		var b strings.Builder
		b.WriteString("\033[H\033[J")
		b.WriteString(colorBold + "ncast watch" + colorReset + colorDim + "  " + endpoint + colorReset + "\n")
		b.WriteString(strings.Repeat("─", 72) + "\n")
		b.WriteString(fmt.Sprintf("  Height %-8d  Best %s  Mempool %-4d  Network %s\n",
			height, bestHash, mempoolN, network))
		if lastBlk != "" {
			b.WriteString(colorDim + "  " + lastBlk + colorReset + "\n")
		}
		b.WriteString(strings.Repeat("─", 72) + "\n")
		for _, e := range events {
			kindColor := colorReset
			switch e.kind {
			case "BLOCK":
				kindColor = colorGreen
			case "TX":
				kindColor = colorCyan
			case "MEMPOOL":
				kindColor = colorYellow
			case "EXEC":
				kindColor = colorBlue
			}
			b.WriteString(fmt.Sprintf("%s%s %-7s %s%s%s\n",
				colorDim, e.time.Format("15:04:05"), e.kind, kindColor, e.message, colorReset))
		}
		if len(events) == 0 {
			b.WriteString(colorDim + "  Waiting for events… (send a tx to see activity)\n" + colorReset)
		}
		b.WriteString(colorDim + "\nCtrl+C to exit" + colorReset)
		fmt.Print(b.String())
	}

	redraw()

	for {
		select {
		case <-cctx.Done():
			return nil
		case <-ticker.C:
			refreshStats()
			redraw()
		case blk := <-blockCh:
			height = blk.Index
			bestHash = ncast.ShortHash(blk.Hash().StringLE(), 6)
			lastBlk = fmt.Sprintf("Block #%d · %d tx · %s",
				blk.Index, len(blk.Transactions), ncast.FormatTimeMs(blk.Timestamp))
			addEvent("BLOCK", fmt.Sprintf("#%-5d %s  %d tx",
				blk.Index, ncast.ShortHash(blk.Hash().StringLE(), 6), len(blk.Transactions)))
			redraw()
		case tx := <-txCh:
			sender := ""
			if len(tx.Signers) > 0 {
				sender = address.Uint160ToString(tx.Signers[0].Account)
			}
			addEvent("TX", fmt.Sprintf("%s  from %s",
				ncast.ShortHash(tx.Hash().StringLE(), 6), ncast.ShortHash(sender, 4)))
			redraw()
		case ev := <-memCh:
			if ev == nil || ev.Tx == nil {
				continue
			}
			sign := "+"
			if ev.Type == mempoolevent.TransactionRemoved {
				sign = "-"
			}
			addEvent("MEMPOOL", fmt.Sprintf("%s %s", sign, ncast.ShortHash(ev.Tx.Hash().StringLE(), 6)))
			if ev.Type == mempoolevent.TransactionAdded {
				mempoolN++
			} else {
				mempoolN--
			}
			if mempoolN < 0 {
				mempoolN = 0
			}
			redraw()
		}
	}
}

func enterAltScreen() {
	fmt.Print("\033[?1049h\033[?25l")
}

func leaveAltScreen() {
	fmt.Print("\033[?1049l\033[?25h")
}
