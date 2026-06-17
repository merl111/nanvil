package txregistry_test

import (
	"errors"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/txregistry"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestRegistry(t *testing.T) {
	txregistry.Reset()
	h := util.Uint256{1, 2, 3}
	txregistry.RecordRelayed(h)
	entry, ok := txregistry.Lookup(h)
	require.True(t, ok)
	require.Equal(t, txregistry.StatusRelayed, entry.Status)

	txregistry.Reset()
	txregistry.RecordRejected(h, errors.New("rejected"))
	entry, ok = txregistry.Lookup(h)
	require.True(t, ok)
	require.Equal(t, txregistry.StatusRejected, entry.Status)
	require.Error(t, entry.Err)
}
