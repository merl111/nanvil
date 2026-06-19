package producer_test

import (
	"context"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/producer"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testSetup(t *testing.T) (*core.Blockchain, *producer.BlockBuilder) {
	pub, err := accounts.ValidatorPublicKeyHex()
	require.NoError(t, err)
	cfg := config.Blockchain{
		ProtocolConfiguration: config.ProtocolConfiguration{
			Magic:              netmode.NanvilNet,
			MaxTraceableBlocks: 1000,
			ValidatorsCount:    1,
			StandbyCommittee:   []string{pub},
		},
	}
	bc, err := core.NewBlockchain(storage.NewMemoryStore(), cfg, zaptest.NewLogger(t))
	require.NoError(t, err)
	go bc.Run()
	t.Cleanup(bc.Close)
	val, err := accounts.NewValidatorSigner()
	require.NoError(t, err)
	return bc, producer.NewBlockBuilder(bc, val, false)
}

func TestMineEmptyBlock(t *testing.T) {
	bc, b := testSetup(t)
	h0 := bc.BlockHeight()
	_, err := b.Mine()
	require.NoError(t, err)
	require.Equal(t, h0+1, bc.BlockHeight())
}

func TestProducerAutomine(t *testing.T) {
	bc, b := testSetup(t)
	p := producer.NewProducer(b, true, 0, false, 0, zaptest.NewLogger(t))
	require.True(t, p.GetAutomine())
	p.SetAutomine(false)
	require.False(t, p.GetAutomine())
	_ = bc
}

func TestIncreaseTime(t *testing.T) {
	bc, b := testSetup(t)
	last, err := bc.GetHeader(bc.GetHeaderHash(bc.BlockHeight()))
	require.NoError(t, err)
	target := last.Timestamp + 5000
	b.SetNextBlockTimestamp(target)
	blk, err := b.Mine()
	require.NoError(t, err)
	require.Equal(t, target, blk.Timestamp)
}

func TestProducerInterval(t *testing.T) {
	_, b := testSetup(t)
	p := producer.NewProducer(b, true, 10*time.Millisecond, true, 0, zaptest.NewLogger(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	time.Sleep(25 * time.Millisecond)
	p.Stop()
}

func TestMineIncludesAllPendingMempoolTxs(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	opts.Accounts = 2
	pubHex, err := accounts.ValidatorPublicKeyHex()
	require.NoError(t, err)

	bcCfg := nanvilcfg.BlockchainConfig(opts)
	bcCfg.StandbyCommittee = []string{pubHex}
	log := zaptest.NewLogger(t)
	bc, err := core.NewBlockchain(storage.NewMemoryStore(), bcCfg, log)
	require.NoError(t, err)
	t.Cleanup(bc.Close)
	go bc.Run()

	mgr, err := accounts.NewManager(opts.Mnemonic, opts.Accounts)
	require.NoError(t, err)
	builder := producer.NewBlockBuilder(bc, mgr.Validator, false)

	require.NoError(t, mgr.FundAll(bc, opts.Balance, func(txs ...*transaction.Transaction) error {
		_, err := builder.Mine(txs...)
		return err
	}))

	pool := func(txs ...*transaction.Transaction) error {
		for _, tx := range txs {
			if err := bc.PoolTx(tx); err != nil {
				return err
			}
		}
		return nil
	}
	for range 3 {
		require.NoError(t, mgr.FundAddress(bc, mgr.Accounts[1].Signer.ScriptHash(), 1, pool))
	}
	require.Equal(t, 3, bc.GetMemPool().Count())

	blk, err := builder.Mine()
	require.NoError(t, err)
	require.Len(t, blk.Transactions, 3)
	require.Equal(t, 0, bc.GetMemPool().Count())
}

func TestDropTransactionAndPendingMempool(t *testing.T) {
	bc, b := testSetup(t)
	p := producer.NewProducer(b, false, 0, false, 0, zaptest.NewLogger(t))

	mgr, err := accounts.NewManager("test test test test test test test test test test test junk", 2)
	require.NoError(t, err)
	require.NoError(t, mgr.FundAll(bc, 10_000_0000_0000, func(txs ...*transaction.Transaction) error {
		_, err := b.Mine(txs...)
		return err
	}))
	tx, err := mgr.SignedGASTransfer(bc, mgr.Accounts[0], mgr.Accounts[1].Signer.ScriptHash(), 1_0000_0000)
	require.NoError(t, err)
	require.NoError(t, bc.GetMemPool().Add(tx, bc))
	require.Equal(t, 1, p.PendingMempool())

	h := tx.Hash()
	require.True(t, p.DropTransaction(h))
	require.False(t, p.DropTransaction(h))
	require.Equal(t, 0, p.PendingMempool())
}

func TestMineEmpty(t *testing.T) {
	bc, b := testSetup(t)
	h0 := bc.BlockHeight()
	blks, err := b.MineEmpty(2)
	require.NoError(t, err)
	require.Len(t, blks, 2)
	require.Equal(t, h0+2, bc.BlockHeight())
}

func TestEmptyBlockInterval(t *testing.T) {
	bc, b := testSetup(t)
	h0 := bc.BlockHeight()
	p := producer.NewProducer(b, true, 0, false, 15*time.Millisecond, zaptest.NewLogger(t))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p.Start(ctx)
	time.Sleep(50 * time.Millisecond)
	p.Stop()
	require.Greater(t, bc.BlockHeight(), h0)
}
