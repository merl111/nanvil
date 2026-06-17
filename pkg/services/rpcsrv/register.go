package rpcsrv

import (
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// RPCHandler is a JSON-RPC method handler.
type RPCHandler func(*Server, params.Params) (any, *neorpc.Error)

var extraHandlers = make(map[string]RPCHandler)

// RegisterRPCHandler registers an additional JSON-RPC method handler.
func RegisterRPCHandler(name string, handler RPCHandler) {
	extraHandlers[name] = handler
}

// RelayOutcomeCallback is invoked after sendrawtransaction relay attempt.
type RelayOutcomeCallback func(relayErr error, hash util.Uint256)

var relayOutcomeCallback RelayOutcomeCallback

// SetRelayOutcomeCallback registers a nanvil relay outcome tracker.
func SetRelayOutcomeCallback(fn RelayOutcomeCallback) {
	relayOutcomeCallback = fn
}

// UnknownTransactionResolver produces wallet-facing errors for missing transactions.
type UnknownTransactionResolver func(hash util.Uint256) *neorpc.Error

var unknownTxResolver UnknownTransactionResolver

// SetUnknownTransactionResolver registers a custom unknown-transaction handler.
func SetUnknownTransactionResolver(fn UnknownTransactionResolver) {
	unknownTxResolver = fn
}

// HasUnknownTransactionResolver reports whether a custom resolver is registered.
func HasUnknownTransactionResolver() bool {
	return unknownTxResolver != nil
}

func lookupHandler(name string) (RPCHandler, bool) {
	if h, ok := rpcHandlers[name]; ok {
		return h, true
	}
	h, ok := extraHandlers[name]
	return h, ok
}
