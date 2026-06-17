package node

import (
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	nanvilrpc "github.com/nspcc-dev/neo-go/pkg/nanvil/rpc"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/producer"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Producer returns producer view for RPC.
func (n *DevNode) Producer() nanvilrpc.ProducerView {
	return producerView{n.prod}
}

// Builder returns block builder view for RPC.
func (n *DevNode) Builder() nanvilrpc.BuilderView {
	return builderView{n.builder}
}

// Accounts returns accounts view.
func (n *DevNode) Accounts() nanvilrpc.AccountsView {
	return accountsView{n}
}

// Snapshots returns snapshot manager.
func (n *DevNode) Snapshots() *snapshot.Manager { return n.SnapMgr }

// ForkInfo returns fork metadata for RPC.
func (n *DevNode) ForkInfo() any {
	if n.Fork == nil {
		return nil
	}
	return map[string]any{
		"rpcUrl":    n.Fork.RPCURL,
		"index":     n.Fork.Index,
		"indexHash": n.Fork.IndexHash.StringLE(),
		"rootHash":  n.Fork.RootHash.StringLE(),
		"contracts": len(n.Fork.Contracts),
		"cached":    remoteCached(n),
	}
}

func remoteCached(n *DevNode) int {
	if n.Remote == nil {
		return 0
	}
	return n.Remote.CachedCount()
}

type producerView struct{ *producer.Producer }

func (p producerView) DropTransaction(h util.Uint256) bool {
	if p.Producer == nil {
		return false
	}
	return p.Producer.DropTransaction(h)
}

type builderView struct{ *producer.BlockBuilder }

func (b builderView) IncreaseTime(seconds uint64) {
	if b.BlockBuilder != nil {
		b.BlockBuilder.IncreaseTime(seconds)
	}
}

func (b builderView) SetNextBlockTimestamp(ts uint64) {
	if b.BlockBuilder != nil {
		b.BlockBuilder.SetNextBlockTimestamp(ts)
	}
}

type accountsView struct{ *DevNode }

func (a accountsView) DevAccountsJSON() []map[string]any {
	out := make([]map[string]any, len(a.accMgr.Accounts))
	for i, acc := range a.accMgr.Accounts {
		out[i] = map[string]any{
			"index":   acc.Index,
			"address": acc.Address,
			"wif":     acc.WIF,
		}
	}
	return out
}

func (a accountsView) FundAddress(hash util.Uint160, amount int64, _ func(...interface{}) error) error {
	return a.accMgr.FundAddress(a.Chain, hash, amount, func(txs ...*transaction.Transaction) error {
		_, err := a.builder.Mine(txs...)
		return err
	})
}
