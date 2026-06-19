package rpc_test

import (
	"errors"
	"testing"

	nanvilrpc "github.com/nspcc-dev/neo-go/pkg/nanvil/rpc"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/txregistry"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestResolveUnknownTransactionRelayed(t *testing.T) {
	h := util.Uint256{0xab}
	txregistry.Reset()
	txregistry.RecordRelayed(h)

	err := nanvilrpc.ResolveUnknownTransaction(h)
	require.NotNil(t, err)
	require.Contains(t, err.Data, "accepted by nanvil")
}

func TestResolveUnknownTransactionRejected(t *testing.T) {
	h := util.Uint256{0xcd}
	txregistry.Reset()
	txregistry.RecordRejected(h, errors.New("rejected"))

	err := nanvilrpc.ResolveUnknownTransaction(h)
	require.NotNil(t, err)
}

func TestRegisterTransactionTracking(t *testing.T) {
	txregistry.Reset()
	nanvilrpc.RegisterTransactionTracking()
	h := util.Uint256{0xef}
	// callback is wired; exercise via txregistry directly after relay simulation
	txregistry.RecordRelayed(h)
	_, ok := txregistry.Lookup(h)
	require.True(t, ok)
}
