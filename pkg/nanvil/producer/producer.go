package producer

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/zap"
)

// BlockBuilder creates and signs blocks for the dev chain.
type BlockBuilder struct {
	Chain     *core.Blockchain
	Validator neotest.Signer
	log       *zap.Logger
	textLog   bool
	mu        sync.Mutex
	// NextTimestampOffset is added to the next block timestamp (time travel).
	NextTimestampOffset uint64
	// FixedNextTimestamp if non-zero overrides computed timestamp for one block.
	FixedNextTimestamp uint64
}

// NewBlockBuilder creates a block builder.
func NewBlockBuilder(chain *core.Blockchain, validator neotest.Signer, textLog bool, logger ...*zap.Logger) *BlockBuilder {
	log := zap.NewNop()
	if len(logger) != 0 && logger[0] != nil {
		log = logger[0]
	}
	return &BlockBuilder{Chain: chain, Validator: validator, log: log, textLog: textLog}
}

// Mine drains the mempool (or uses provided txs) and adds one block.
// When no transactions are provided, all verified mempool transactions are
// included (subject to chain block policy limits).
func (b *BlockBuilder) Mine(txs ...*transaction.Transaction) (*block.Block, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.mineLocked(txs...)
}

func (b *BlockBuilder) mineLocked(txs ...*transaction.Transaction) (*block.Block, error) {
	b.purgeStaleMempool()
	if len(txs) == 0 {
		txs = b.collectMempoolTxsForBlock()
	}
	blk, err := b.buildBlock(txs)
	if err != nil {
		return nil, err
	}
	if err := b.Chain.AddBlock(blk); err != nil {
		return nil, err
	}
	txHashes := make([]string, 0, len(blk.Transactions))
	for _, tx := range blk.Transactions {
		h := tx.Hash().StringLE()
		txHashes = append(txHashes, h)
		if !b.textLog {
			b.log.Info("mined transaction",
				zap.Uint32("block", blk.Index),
				zap.String("tx", h),
				zap.Int64("network_fee", tx.NetworkFee),
				zap.Int64("system_fee", tx.SystemFee),
			)
		}
	}
	if b.textLog {
		txWord := "transactions"
		if len(blk.Transactions) == 1 {
			txWord = "transaction"
		}
		fmt.Fprintf(os.Stderr, "    Block %d mined (%d %s)\n", blk.Index, len(blk.Transactions), txWord)
		for _, h := range txHashes {
			fmt.Fprintf(os.Stderr, "      %s\n", h)
		}
	} else {
		b.log.Info("new block mined",
			zap.Uint32("index", blk.Index),
			zap.String("hash", blk.Hash().StringLE()),
			zap.Int("txs", len(blk.Transactions)),
			zap.Uint64("timestamp", blk.Timestamp),
			zap.Strings("tx_hashes", txHashes),
		)
	}
	return blk, nil
}

// MineEmpty adds empty blocks.
func (b *BlockBuilder) MineEmpty(count int) ([]*block.Block, error) {
	out := make([]*block.Block, 0, count)
	for range count {
		blk, err := b.Mine()
		if err != nil {
			return out, err
		}
		out = append(out, blk)
	}
	return out, nil
}

func (b *BlockBuilder) collectMempoolTxs() []*transaction.Transaction {
	return b.Chain.GetMemPool().GetVerifiedTransactions()
}

func (b *BlockBuilder) collectMempoolTxsForBlock() []*transaction.Transaction {
	return b.Chain.ApplyPolicyToTxSet(b.collectMempoolTxs())
}

func (b *BlockBuilder) purgeStaleMempool() {
	mp := b.Chain.GetMemPool()
	mp.RemoveStale(func(tx *transaction.Transaction) bool {
		// Do not pass mp here: RemoveStale already holds the mempool lock and
		// IsTxStillRelevant would call HasConflicts which tries to RLock it.
		return b.Chain.IsTxStillRelevant(tx, nil, false)
	}, b.Chain)
}

func (b *BlockBuilder) buildBlock(txs []*transaction.Transaction) (*block.Block, error) {
	lastHash := b.Chain.GetHeaderHash(b.Chain.BlockHeight())
	lastHeader, err := b.Chain.GetHeader(lastHash)
	if err != nil {
		return nil, err
	}
	ts := lastHeader.Timestamp + 1 + b.NextTimestampOffset
	if b.FixedNextTimestamp != 0 {
		ts = b.FixedNextTimestamp
		b.FixedNextTimestamp = 0
	} else if b.Chain.GetConfig().NanvilForkMode || b.Chain.GetConfig().Magic == netmode.NanvilNet {
		now := uint64(time.Now().UnixMilli())
		minTs := lastHeader.Timestamp + 1
		if now > minTs {
			ts = now
		} else {
			ts = minTs
		}
	}
	b.NextTimestampOffset = 0

	blk := &block.Block{
		Header: block.Header{
			NextConsensus: b.Validator.ScriptHash(),
			Script: transaction.Witness{
				VerificationScript: b.Validator.Script(),
			},
			Timestamp: ts,
		},
		Transactions: txs,
	}
	if b.Chain.GetConfig().StateRootInHeader {
		blk.StateRootEnabled = true
		blk.PrevStateRoot = b.Chain.GetStateModule().CurrentLocalStateRoot()
	}
	blk.PrevHash = lastHash
	blk.Index = b.Chain.BlockHeight() + 1
	blk.RebuildMerkleRoot()
	invoc := b.Validator.SignHashable(uint32(b.Chain.GetConfig().Magic), blk)
	blk.Script.InvocationScript = invoc
	return blk, nil
}

// IncreaseTime adds seconds to the next mined block timestamp.
func (b *BlockBuilder) IncreaseTime(seconds uint64) {
	b.mu.Lock()
	b.NextTimestampOffset += seconds
	b.mu.Unlock()
}

// SetNextBlockTimestamp sets absolute timestamp for the next block.
func (b *BlockBuilder) SetNextBlockTimestamp(ts uint64) {
	b.mu.Lock()
	b.FixedNextTimestamp = ts
	b.mu.Unlock()
}

// Producer manages auto-mining behavior.
type Producer struct {
	Builder            *BlockBuilder
	AutoMine           bool
	BlockTime          time.Duration
	MineEmptyBlocks    bool
	EmptyBlockInterval time.Duration
	log                *zap.Logger
	cancel             context.CancelFunc
	mineCh             chan *transaction.Transaction
	mu                 sync.Mutex
}

const mineQueueSize = 8192

// NewProducer creates a block producer.
func NewProducer(builder *BlockBuilder, autoMine bool, blockTime time.Duration, mineEmpty bool, emptyInterval time.Duration, log *zap.Logger) *Producer {
	return &Producer{
		Builder:            builder,
		AutoMine:           autoMine,
		BlockTime:          blockTime,
		MineEmptyBlocks:    mineEmpty,
		EmptyBlockInterval: emptyInterval,
		log:                log,
		mineCh:             make(chan *transaction.Transaction, mineQueueSize),
	}
}

// Start begins background mining workers.
func (p *Producer) Start(ctx context.Context) {
	ctx, p.cancel = context.WithCancel(ctx)
	go p.mineWorker(ctx)
	if p.AutoMine && p.BlockTime > 0 {
		go p.intervalLoop(ctx)
	}
	if p.AutoMine && p.EmptyBlockInterval > 0 && p.BlockTime == 0 {
		go p.emptyBlockLoop(ctx)
	}
}

func (p *Producer) mineWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case tx := <-p.mineCh:
			p.mineOne(tx)
		}
	}
}

func (p *Producer) intervalLoop(ctx context.Context) {
	ticker := time.NewTicker(p.BlockTime)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !p.MineEmptyBlocks && p.Builder.Chain.GetMemPool().Count() == 0 {
				continue
			}
			p.enqueueMine(nil)
		}
	}
}

func (p *Producer) emptyBlockLoop(ctx context.Context) {
	ticker := time.NewTicker(p.EmptyBlockInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if p.Builder.Chain.GetMemPool().Count() == 0 {
				p.enqueueMine(nil)
			}
		}
	}
}

// Stop stops background mining.
func (p *Producer) Stop() {
	if p.cancel != nil {
		p.cancel()
	}
}

// SetAutomine toggles auto-mining on transaction relay.
func (p *Producer) SetAutomine(v bool) {
	p.mu.Lock()
	p.AutoMine = v
	p.mu.Unlock()
}

// GetAutomine returns auto-mine state.
func (p *Producer) GetAutomine() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.AutoMine
}

// OnTransactionRelayed is called after a tx enters the mempool.
func (p *Producer) OnTransactionRelayed(tx *transaction.Transaction) {
	if !p.GetAutomine() || p.BlockTime > 0 {
		return
	}
	p.enqueueMine(tx)
}

func (p *Producer) enqueueMine(tx *transaction.Transaction) {
	select {
	case p.mineCh <- tx:
	default:
		if tx != nil {
			p.log.Warn("mine queue full, mining synchronously", zap.Stringer("hash", tx.Hash()))
		} else {
			p.log.Warn("mine queue full, mining synchronously")
		}
		p.mineOne(tx)
	}
}

func (p *Producer) mineOne(tx *transaction.Transaction) {
	if tx != nil && p.Builder.Chain.GetMemPool().Count() == 0 {
		return
	}
	blk, err := p.Builder.Mine()
	if err != nil {
		fields := []zap.Field{zap.Error(err)}
		if tx != nil {
			fields = append(fields, zap.Stringer("hash", tx.Hash()))
		}
		if strings.Contains(err.Error(), "expired") {
			p.Builder.purgeStaleMempool()
			if _, retryErr := p.Builder.Mine(); retryErr == nil {
				return
			}
		}
		if strings.Contains(err.Error(), "hash mismatch") {
			p.log.Error("auto-mine failed: chain header desync — restart nanvil", fields...)
		} else {
			p.log.Warn("auto-mine failed", fields...)
		}
		return
	}
	if tx != nil && blk != nil && len(blk.Transactions) == 0 {
		p.log.Warn("mined block did not include relayed transaction", zap.Stringer("hash", tx.Hash()))
	}
}

// DropTransaction removes a transaction from the mempool by hash.
func (p *Producer) DropTransaction(h util.Uint256) bool {
	mp := p.Builder.Chain.GetMemPool()
	if !mp.ContainsKey(h) {
		return false
	}
	mp.Remove(h)
	return true
}

// PendingMempool returns the number of verified mempool transactions.
func (p *Producer) PendingMempool() int {
	return p.Builder.Chain.GetMemPool().Count()
}
