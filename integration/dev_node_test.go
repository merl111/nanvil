package integration_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/stretchr/testify/require"
)

func TestDevNodeRPC(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 18545
	opts.Accounts = 10
	opts.BlockTime = 0

	dev, err := node.NewDevNode(opts, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, dev.Start(ctx))
	defer dev.Shutdown()

	require.Equal(t, uint32(10), dev.Chain.BlockHeight())

	body := `{"jsonrpc":"2.0","id":1,"method":"nanvil_getAutomine","params":[]}`
	resp, err := http.Post("http://127.0.0.1:18545", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var out struct {
		Result bool `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.True(t, out.Result)

	accs, err := accounts.Generate(opts.Mnemonic, opts.Accounts)
	require.NoError(t, err)
	expected := opts.Balance
	for _, acc := range accs {
		require.Equal(t, expected, dev.Chain.GetUtilityTokenBalance(acc.Signer.ScriptHash()).Int64(), acc.Address)
	}
}

func TestDevNodeMine(t *testing.T) {
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 18546
	opts.NoMining = true
	opts.Accounts = 2

	dev, err := node.NewDevNode(opts, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, dev.Start(ctx))
	defer dev.Shutdown()

	h0 := dev.Chain.BlockHeight()
	body := `{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[1]}`
	resp, err := http.Post("http://127.0.0.1:18546", "application/json", strings.NewReader(body))
	require.NoError(t, err)
	resp.Body.Close()
	require.Greater(t, dev.Chain.BlockHeight(), h0)
}
