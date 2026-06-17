package rpc_test

import (
	"testing"

	nanvilrpc "github.com/nspcc-dev/neo-go/pkg/nanvil/rpc"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

type mockNode struct {
	autoMine bool
}

func (m *mockNode) MineBlock(int) error           { return nil }
func (m *mockNode) ResetChain() error             { return nil }
func (m *mockNode) Producer() nanvilrpc.ProducerView { return m }
func (m *mockNode) Builder() nanvilrpc.BuilderView   { return m }
func (m *mockNode) Snapshots() *snapshot.Manager     { return snapshot.NewManager(nil) }
func (m *mockNode) Accounts() nanvilrpc.AccountsView { return mockAccounts{} }
func (m *mockNode) ForkInfo() any                    { return nil }
func (m *mockNode) SetAutomine(v bool)               { m.autoMine = v }
func (m *mockNode) GetAutomine() bool                { return m.autoMine }
func (m *mockNode) DropTransaction(util.Uint256) bool { return false }
func (m *mockNode) IncreaseTime(uint64)              {}
func (m *mockNode) SetNextBlockTimestamp(uint64)     {}

type mockAccounts struct{}

func (mockAccounts) DevAccountsJSON() []map[string]any { return nil }
func (mockAccounts) FundAddress(util.Uint160, int64, func(...interface{}) error) error {
	return nil
}

func TestRegisterHandlers(t *testing.T) {
	n := &mockNode{}
	nanvilrpc.RegisterHandlers(n)
	nanvilrpc.SetServerContext(n)
	require.NotNil(t, nanvilrpc.GetContextForTest())
}
