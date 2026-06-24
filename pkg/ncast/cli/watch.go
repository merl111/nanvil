package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/mempoolevent"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/ncast"
	"github.com/nspcc-dev/neo-go/pkg/neorpc/result"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/urfave/cli/v2"
)

const (
	colorReset   = "\033[0m"
	colorDim     = "\033[2m"
	colorGreen   = "\033[32m"
	colorCyan    = "\033[36m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorRed     = "\033[31m"
	colorBold    = "\033[1m"
)

func watchCmd() *cli.Command {
	return &cli.Command{
		Name:    "watch",
		Aliases: []string{"live", "tail"},
		Usage:   "Live chain status — blocks, transactions, mempool in real time",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "lines", Aliases: []string{"n"}, Value: 20, Usage: "Number of recent events to show"},
			&cli.StringFlag{Name: "contract", Aliases: []string{"c"}, Usage: "Filter notifications by contract (native name, N-address, or script hash)"},
			&cli.StringFlag{Name: "event", Aliases: []string{"e"}, Usage: "Filter notifications by event name (e.g. Transfer)"},
			&cli.DurationFlag{Name: "poll", Aliases: []string{"p"}, Value: 2 * time.Second, Usage: "Polling interval in event mode (e.g. 500ms, 2s)"},
		},
		Action: func(ctx *cli.Context) error {
			if ctx.String("contract") != "" || ctx.String("event") != "" {
				return runWatchEvents(ctx)
			}
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


func runWatchEvents(ctx *cli.Context) error {
	endpoint := rpcEndpoint(ctx)
	poll := ctx.Duration("poll")
	if poll <= 0 {
		return fmt.Errorf("--poll must be positive")
	}
	eventFilter := strings.ToLower(strings.TrimSpace(ctx.String("event")))
	jsonMode := jsonOut(ctx)

	cctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	c, err := ncast.RPCClient(cctx, endpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	var (
		contractFilter    util.Uint160
		hasContractFilter bool
	)
	if s := strings.TrimSpace(ctx.String("contract")); s != "" {
		u, err := ncast.ResolveContract(c, s)
		if err != nil {
			return fmt.Errorf("contract: %w", err)
		}
		contractFilter = u
		hasContractFilter = true
	}

	count, err := c.GetBlockCount()
	if err != nil {
		return fmt.Errorf("getblockcount: %w", err)
	}
	var lastBlock uint32
	if count > 0 {
		lastBlock = count - 1
	}

	if !jsonMode {
		printEventsHeader(endpoint, hasContractFilter, contractFilter, eventFilter, poll, lastBlock+1)
	}

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	for {
		select {
		case <-cctx.Done():
			return nil
		case <-ticker.C:
			current, err := c.GetBlockCount()
			if err != nil {
				if !jsonMode {
					fmt.Fprintf(os.Stderr, "%spoll error: %s%s\n", colorRed, err, colorReset)
				}
				continue
			}
			if current == 0 {
				continue
			}
			for i := lastBlock + 1; i <= current-1; i++ {
				processEventBlock(c, i, hasContractFilter, contractFilter, eventFilter, jsonMode)
				lastBlock = i
			}
		}
	}
}

func processEventBlock(c *rpcclient.Client, idx uint32, hasContractFilter bool, contractFilter util.Uint160, eventFilter string, jsonMode bool) {
	hash, err := c.GetBlockHash(idx)
	if err != nil {
		if !jsonMode {
			fmt.Fprintf(os.Stderr, "%sblock %d: %s%s\n", colorRed, idx, err, colorReset)
		}
		return
	}
	blk, err := c.GetBlockByHash(hash)
	if err != nil {
		if !jsonMode {
			fmt.Fprintf(os.Stderr, "%sblock %d: %s%s\n", colorRed, idx, err, colorReset)
		}
		return
	}
	for _, tx := range blk.Transactions {
		appLog, err := c.GetApplicationLog(tx.Hash(), nil)
		if err != nil {
			continue
		}
		for _, exec := range appLog.Executions {
			for _, ne := range exec.Events {
				if hasContractFilter && ne.ScriptHash != contractFilter {
					continue
				}
				if eventFilter != "" && strings.ToLower(ne.Name) != eventFilter {
					continue
				}
				if jsonMode {
					printEventJSON(idx, tx.Hash(), ne)
				} else {
					printEventHuman(idx, tx.Hash(), ne)
				}
			}
		}
	}
}

func printEventsHeader(endpoint string, hasContract bool, contract util.Uint160, eventFilter string, poll time.Duration, startBlock uint32) {
	fmt.Println()
	fmt.Printf("%s%s ncast watch%s\n", colorBold, colorGreen, colorReset)
	fmt.Printf("%s  RPC      : %s%s\n", colorDim, endpoint, colorReset)
	cf := "all contracts"
	if hasContract {
		cf = "0x" + contract.StringLE()
	}
	fmt.Printf("%s  Contract : %s%s\n", colorDim, cf, colorReset)
	ef := "all events"
	if eventFilter != "" {
		ef = eventFilter
	}
	fmt.Printf("%s  Event    : %s%s\n", colorDim, ef, colorReset)
	fmt.Printf("%s  Poll     : %s%s\n", colorDim, poll, colorReset)
	fmt.Printf("%s  Starting at block %d. Press Ctrl+C to stop.%s\n\n", colorDim, startBlock, colorReset)
}

func printEventHuman(blockIdx uint32, txHash util.Uint256, ne state.NotificationEvent) {
	fmt.Printf("\n%s── Block %d ──────────────────────────────%s\n", colorCyan, blockIdx, colorReset)
	fmt.Printf("  TX       %s%s%s\n", colorDim, txHash.StringLE(), colorReset)
	fmt.Printf("  Contract %s0x%s%s\n", colorYellow, ne.ScriptHash.StringLE(), colorReset)
	fmt.Printf("  Event    %s%s%s\n", colorGreen, ne.Name, colorReset)
	var items []stackitem.Item
	if ne.Item != nil {
		if v, ok := ne.Item.Value().([]stackitem.Item); ok {
			items = v
		}
	}
	if len(items) > 0 {
		fmt.Println("  Args")
		for i, it := range items {
			t := "Any"
			if it != nil {
				t = it.Type().String()
			}
			fmt.Printf("    %s[%d]%s %s%s%s = %s\n",
				colorDim, i, colorReset,
				colorMagenta, t, colorReset,
				formatStackItem(it))
		}
	}
}

func printEventJSON(blockIdx uint32, txHash util.Uint256, ne state.NotificationEvent) {
	out := struct {
		Block  uint32       `json:"block"`
		TxHash util.Uint256 `json:"txhash"`
		state.NotificationEvent
	}{
		Block:             blockIdx,
		TxHash:            txHash,
		NotificationEvent: ne,
	}
	data, err := json.Marshal(out)
	if err != nil {
		return
	}
	fmt.Println(string(data))
}

func formatStackItem(it stackitem.Item) string {
	if it == nil {
		return "null"
	}
	switch v := it.Value().(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case *big.Int:
		return v.String()
	case []byte:
		if utf8.Valid(v) && isPrintableBytes(v) {
			return strconv.Quote(string(v))
		}
		return "0x" + hex.EncodeToString(v)
	case []stackitem.Item:
		return fmt.Sprintf("[%d items]", len(v))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func isPrintableBytes(b []byte) bool {
	for _, c := range b {
		if c < 0x20 && c != '\n' && c != '\t' && c != '\r' {
			return false
		}
	}
	return true
}