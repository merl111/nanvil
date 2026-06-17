package rpc

import (
	"github.com/nspcc-dev/neo-go/pkg/nanvil/txregistry"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// RegisterTransactionTracking wires nanvil relay/lookup tracking into the RPC server
// so wallets receive C#-compatible unknown transaction errors and relay failures.
func RegisterTransactionTracking() {
	rpcsrv.SetRelayOutcomeCallback(func(relayErr error, hash util.Uint256) {
		if relayErr != nil {
			txregistry.RecordRejected(hash, relayErr)
			return
		}
		txregistry.RecordRelayed(hash)
	})
	rpcsrv.SetUnknownTransactionResolver(ResolveUnknownTransaction)
}

// ResolveUnknownTransaction returns wallet-facing errors for missing transactions.
func ResolveUnknownTransaction(h util.Uint256) *neorpc.Error {
	if entry, ok := txregistry.Lookup(h); ok {
		switch entry.Status {
		case txregistry.StatusRejected:
			return rpcsrv.RelayErrorToRPC(entry.Err, h)
		case txregistry.StatusRelayed:
			hstr := "0x" + h.StringLE()
			return neorpc.NewError(neorpc.ErrUnknownTransactionCode, "Unknown transaction - "+hstr,
				"Transaction was accepted by nanvil but is not on the current chain. Restarting nanvil resets the in-memory blockchain.")
		}
	}
	return neorpc.UnknownTransactionError(h.StringLE())
}
