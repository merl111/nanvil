package rpc

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/impersonate"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/nspcc-dev/neo-go/pkg/neorpc"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv/params"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type handlerMockNode struct {
	autoMine      bool
	mined         int
	resetCalled   bool
	dropped       bool
	increasedTime uint64
	nextTimestamp uint64
	snapMgr       *snapshot.Manager
	accounts      handlerMockAccounts
	forkInfo      any
	mineErr       error
	resetErr      error
}

type handlerMockAccounts struct {
	accounts []map[string]any
}

func (m handlerMockAccounts) DevAccountsJSON() []map[string]any { return m.accounts }
func (m handlerMockAccounts) FundAddress(util.Uint160, int64, func(...interface{}) error) error {
	return nil
}

func (m *handlerMockNode) MineBlock(count int) error {
	m.mined += count
	return m.mineErr
}
func (m *handlerMockNode) ResetChain() error {
	m.resetCalled = true
	return m.resetErr
}
func (m *handlerMockNode) Producer() ProducerView { return m }
func (m *handlerMockNode) Builder() BuilderView   { return m }
func (m *handlerMockNode) Snapshots() *snapshot.Manager {
	if m.snapMgr != nil {
		return m.snapMgr
	}
	return snapshot.NewManager(nil)
}
func (m *handlerMockNode) Accounts() AccountsView { return m.accounts }
func (m *handlerMockNode) ForkInfo() any          { return m.forkInfo }
func (m *handlerMockNode) SetAutomine(v bool)     { m.autoMine = v }
func (m *handlerMockNode) GetAutomine() bool      { return m.autoMine }
func (m *handlerMockNode) DropTransaction(util.Uint256) bool {
	m.dropped = true
	return true
}
func (m *handlerMockNode) IncreaseTime(seconds uint64) { m.increasedTime = seconds }
func (m *handlerMockNode) SetNextBlockTimestamp(ts uint64) {
	m.nextTimestamp = ts
}

func mustParams(t *testing.T, vals ...any) params.Params {
	t.Helper()
	p, err := params.FromAny(vals)
	require.NoError(t, err)
	return p
}

func setupHandlerNode(t *testing.T) *handlerMockNode {
	t.Helper()
	impersonate.Global().Reset()
	n := &handlerMockNode{
		accounts: handlerMockAccounts{
			accounts: []map[string]any{{"index": 0, "address": "NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg"}},
		},
		forkInfo: map[string]any{"index": uint32(10)},
	}
	SetServerContext(n)
	return n
}

func TestHandleMine(t *testing.T) {
	n := setupHandlerNode(t)
	res, err := handleMine(nil, mustParams(t))
	require.Nil(t, err)
	require.Equal(t, "0x0", res)
	require.Equal(t, 1, n.mined)

	res, err = handleMine(nil, mustParams(t, 3))
	require.Nil(t, err)
	require.Equal(t, 4, n.mined)
}

func TestHandleAutomine(t *testing.T) {
	n := setupHandlerNode(t)
	_, err := handleSetAutomine(nil, params.Params{})
	require.Equal(t, neorpc.ErrInvalidParams, err)

	res, err := handleSetAutomine(nil, mustParams(t, false))
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.False(t, n.autoMine)

	res, err = handleGetAutomine(nil, nil)
	require.Nil(t, err)
	require.Equal(t, false, res)
}

func TestHandleImpersonate(t *testing.T) {
	setupHandlerNode(t)
	acc := "NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg"
	res, err := handleImpersonate(nil, mustParams(t, acc))
	require.Nil(t, err)
	require.Equal(t, true, res)
	h, _ := address.StringToUint160(acc)
	require.True(t, impersonate.Global().IsImpersonated(h))

	res, err = handleStopImpersonate(nil, mustParams(t, acc))
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.False(t, impersonate.Global().IsImpersonated(h))

	res, err = handleAutoImpersonate(nil, mustParams(t, true))
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.True(t, impersonate.Global().AutoMode())
}

func TestHandleTimeControls(t *testing.T) {
	n := setupHandlerNode(t)
	res, err := handleIncreaseTime(nil, mustParams(t, 3600))
	require.Nil(t, err)
	require.Equal(t, 3600, res)
	require.Equal(t, uint64(3600), n.increasedTime)
	require.Equal(t, 1, n.mined)

	res, err = handleSetNextTimestamp(nil, mustParams(t, 1_700_000_000_000))
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.Equal(t, uint64(1_700_000_000_000), n.nextTimestamp)
}

func TestHandleSetBalance(t *testing.T) {
	res, err := handleSetBalance(nil, nil)
	require.Nil(t, res)
	require.NotNil(t, err)
	require.Contains(t, err.Data, "validator")
}

func TestHandleSnapshotRevertReset(t *testing.T) {
	n := setupHandlerNode(t)

	res, err := handleReset(nil, nil)
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.True(t, n.resetCalled)
}

func TestHandleDropTx(t *testing.T) {
	n := setupHandlerNode(t)
	h := util.Uint256{1, 2, 3}
	res, err := handleDropTx(nil, mustParams(t, "0x"+h.StringLE()))
	require.Nil(t, err)
	require.Equal(t, true, res)
	require.True(t, n.dropped)
}

func TestHandleNodeInfo(t *testing.T) {
	setupHandlerNode(t)
	res, err := handleNodeInfo(nil, nil)
	require.Nil(t, err)
	raw, ok := res.(json.RawMessage)
	require.True(t, ok)
	var info struct {
		Fork     map[string]any   `json:"fork"`
		Accounts []map[string]any `json:"accounts"`
	}
	require.NoError(t, json.Unmarshal(raw, &info))
	require.Equal(t, float64(10), info.Fork["index"])
	require.Len(t, info.Accounts, 1)
}

func TestParseAddressParam(t *testing.T) {
	_, err := parseAddressParam(params.Params{}, 0)
	require.Equal(t, neorpc.ErrInvalidParams, err)

	h, err := parseAddressParam(mustParams(t, "not-an-address"), 0)
	require.Error(t, err)
	require.Equal(t, util.Uint160{}, h)

	h, err = parseAddressParam(mustParams(t, "NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg"), 0)
	require.Nil(t, err)
	require.NotEqual(t, util.Uint160{}, h)
}

func TestStringsTrim0x(t *testing.T) {
	require.Equal(t, "abc", stringsTrim0x("0xabc"))
	require.Equal(t, "abc", stringsTrim0x("0Xabc"))
	require.Equal(t, "abc", stringsTrim0x("abc"))
}
