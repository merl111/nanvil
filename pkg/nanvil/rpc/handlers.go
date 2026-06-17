package rpc

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/impersonate"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// NodeContext is the dev node surface used by RPC handlers.
type NodeContext interface {
	MineBlock(count int) error
	ResetChain() error
	Producer() ProducerView
	Builder() BuilderView
	Snapshots() *snapshot.Manager
	Accounts() AccountsView
	ForkInfo() any
}

type ProducerView interface {
	SetAutomine(bool)
	GetAutomine() bool
	DropTransaction(util.Uint256) bool
}

type BuilderView interface {
	IncreaseTime(seconds uint64)
	SetNextBlockTimestamp(ts uint64)
}

type AccountsView interface {
	DevAccountsJSON() []map[string]any
	FundAddress(hash util.Uint160, amount int64, mine func(...interface{}) error) error
}

var (
	ctxMu sync.RWMutex
	ctx   NodeContext
)

// SetServerContext sets the active dev node for RPC handlers.
func SetServerContext(n NodeContext) {
	ctxMu.Lock()
	ctx = n
	ctxMu.Unlock()
}

func getCtx() NodeContext {
	ctxMu.RLock()
	defer ctxMu.RUnlock()
	return ctx
}

// GetContextForTest exposes context for unit tests.
func GetContextForTest() NodeContext {
	return getCtx()
}

// RegisterHandlers registers all nanvil_* RPC methods.
func RegisterHandlers(n NodeContext) {
	SetServerContext(n)
	handlers := map[string]rpcsrv.RPCHandler{
		"nanvil_mine":                      handleMine,
		"nanvil_setAutomine":               handleSetAutomine,
		"nanvil_getAutomine":               handleGetAutomine,
		"nanvil_impersonateAccount":        handleImpersonate,
		"nanvil_stopImpersonatingAccount":  handleStopImpersonate,
		"nanvil_autoImpersonateAccount":    handleAutoImpersonate,
		"nanvil_increaseTime":              handleIncreaseTime,
		"nanvil_setNextBlockTimestamp":     handleSetNextTimestamp,
		"nanvil_setBalance":                handleSetBalance,
		"nanvil_snapshot":                  handleSnapshot,
		"nanvil_revert":                      handleRevert,
		"nanvil_reset":                     handleReset,
		"nanvil_dropTransaction":           handleDropTx,
		"nanvil_nodeInfo":                  handleNodeInfo,
		// Hardhat/Anvil aliases
		"evm_mine":         handleMine,
		"evm_increaseTime": handleIncreaseTime,
	}
	for name, h := range handlers {
		rpcsrv.RegisterRPCHandler(name, h)
	}
}

func handleMine(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	count := 1
	if len(p) > 0 {
		c, err := p[0].GetInt()
		if err == nil && c > 0 {
			count = c
		}
	}
	if err := n.MineBlock(count); err != nil {
		return nil, neorpc.NewInternalServerError(err.Error())
	}
	return "0x0", nil
}

func handleSetAutomine(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Producer() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	v, err := p[0].GetBoolean()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	n.Producer().SetAutomine(v)
	return true, nil
}

func handleGetAutomine(_ *rpcsrv.Server, _ params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Producer() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	return n.Producer().GetAutomine(), nil
}

func parseAddressParam(p params.Params, idx int) (util.Uint160, *neorpc.Error) {
	if len(p) <= idx {
		return util.Uint160{}, neorpc.ErrInvalidParams
	}
	s, err := p[idx].GetString()
	if err != nil {
		return util.Uint160{}, neorpc.ErrInvalidParams
	}
	u, err := address.StringToUint160(s)
	if err != nil {
		return util.Uint160{}, neorpc.WrapErrorWithData(neorpc.ErrInvalidParams, err.Error())
	}
	return u, nil
}

func handleImpersonate(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	h, rerr := parseAddressParam(p, 0)
	if rerr != nil {
		return nil, rerr
	}
	impersonate.Global().Impersonate(h)
	return true, nil
}

func handleStopImpersonate(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	h, rerr := parseAddressParam(p, 0)
	if rerr != nil {
		return nil, rerr
	}
	impersonate.Global().StopImpersonating(h)
	return true, nil
}

func handleAutoImpersonate(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	v, err := p[0].GetBoolean()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	impersonate.Global().SetAutoMode(v)
	return true, nil
}

func handleIncreaseTime(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Builder() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	sec, err := p[0].GetInt()
	if err != nil || sec < 0 {
		return nil, neorpc.ErrInvalidParams
	}
	n.Builder().IncreaseTime(uint64(sec))
	if err := n.MineBlock(1); err != nil {
		return nil, neorpc.NewInternalServerError(err.Error())
	}
	return sec, nil
}

func handleSetNextTimestamp(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Builder() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	ts, err := p[0].GetInt()
	if err != nil || ts < 0 {
		return nil, neorpc.ErrInvalidParams
	}
	n.Builder().SetNextBlockTimestamp(uint64(ts))
	return true, nil
}

func handleSetBalance(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	return nil, neorpc.NewInternalServerError("use native GAS transfer via nanvil prefunded validator accounts")
}

func handleSnapshot(_ *rpcsrv.Server, _ params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Snapshots() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	id, err := n.Snapshots().Snapshot()
	if err != nil {
		return nil, neorpc.NewInternalServerError(err.Error())
	}
	return string(id), nil
}

func handleRevert(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Snapshots() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	id, err := p[0].GetString()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	if err := n.Snapshots().Revert(snapshot.ID(id)); err != nil {
		return nil, neorpc.NewInternalServerError(err.Error())
	}
	return true, nil
}

func handleReset(_ *rpcsrv.Server, _ params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if err := n.ResetChain(); err != nil {
		return nil, neorpc.NewInternalServerError(err.Error())
	}
	return true, nil
}

func handleDropTx(_ *rpcsrv.Server, p params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil || n.Producer() == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	if len(p) < 1 {
		return nil, neorpc.ErrInvalidParams
	}
	s, err := p[0].GetString()
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	h, err := util.Uint256DecodeStringLE(stringsTrim0x(s))
	if err != nil {
		return nil, neorpc.ErrInvalidParams
	}
	return n.Producer().DropTransaction(h), nil
}

func stringsTrim0x(s string) string {
	if len(s) >= 2 && (s[0:2] == "0x" || s[0:2] == "0X") {
		return s[2:]
	}
	return s
}

func handleNodeInfo(_ *rpcsrv.Server, _ params.Params) (any, *neorpc.Error) {
	n := getCtx()
	if n == nil {
		return nil, neorpc.NewInternalServerError("nanvil not initialized")
	}
	info := map[string]any{
		"fork": n.ForkInfo(),
	}
	if n.Accounts() != nil {
		info["accounts"] = n.Accounts().DevAccountsJSON()
	}
	raw, _ := json.Marshal(info)
	return json.RawMessage(raw), nil
}

// Adapter methods on DevNode - implement in node_adapter.go

func errf(format string, args ...any) *neorpc.Error {
	return neorpc.NewInternalServerError(fmt.Sprintf(format, args...))
}
