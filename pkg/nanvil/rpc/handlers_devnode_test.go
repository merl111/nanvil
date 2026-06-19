package rpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func startHandlerDevNode(t *testing.T) *node.DevNode {
	t.Helper()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.Explorer = false
	opts.NoMining = true
	opts.Accounts = 3
	n, err := node.NewDevNode(opts, zap.NewNop())
	require.NoError(t, err)
	require.NoError(t, n.Start(context.Background()))
	t.Cleanup(n.Shutdown)
	return n
}

func rpcPost(t *testing.T, addr, body string) []byte {
	t.Helper()
	resp, err := http.Post("http://"+addr, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return raw
}

func TestNanvilHandlersOnDevNode(t *testing.T) {
	n := startHandlerDevNode(t)
	addr := n.RPCAddr

	t.Run("setAutomine", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_setAutomine","params":[false]}`)
		var out struct {
			Result bool `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)
	})

	t.Run("getAutomine", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_getAutomine","params":[]}`)
		var out struct {
			Result bool `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.False(t, out.Result)
	})

	t.Run("mine", func(t *testing.T) {
		h0 := n.Chain.BlockHeight()
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[2]}`)
		var out struct {
			Result string `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.Equal(t, "0x0", out.Result)
		require.Equal(t, h0+2, n.Chain.BlockHeight())
	})

	t.Run("increaseTime", func(t *testing.T) {
		h0 := n.Chain.BlockHeight()
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_increaseTime","params":[60]}`)
		var out struct {
			Result int `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.Equal(t, 60, out.Result)
		require.Greater(t, n.Chain.BlockHeight(), h0)
	})

	t.Run("setNextBlockTimestamp", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_setNextBlockTimestamp","params":[2000000000000]}`)
		var out struct {
			Result bool `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)
	})

	t.Run("impersonate", func(t *testing.T) {
		devAcc := "NRBs4Wc8vuFJY5sN7WvJNRzVduAeXQ8TQg"
		raw := rpcPost(t, addr, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"nanvil_impersonateAccount","params":["%s"]}`, devAcc))
		var out struct {
			Result bool `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)

		raw = rpcPost(t, addr, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"nanvil_stopImpersonatingAccount","params":["%s"]}`, devAcc))
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)

		raw = rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_autoImpersonateAccount","params":[true]}`)
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)
	})

	t.Run("snapshot", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_snapshot","params":[]}`)
		var snapOut struct {
			Result string `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &snapOut))
		require.NotEmpty(t, snapOut.Result)
	})

	t.Run("revertOnRunningChainReturnsError", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_snapshot","params":[]}`)
		var snapOut struct {
			Result string `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &snapOut))
		rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_mine","params":[1]}`)

		raw = rpcPost(t, addr, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"nanvil_revert","params":["%s"]}`, snapOut.Result))
		var out struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.NotEmpty(t, out.Error.Message)
	})

	t.Run("evmAliases", func(t *testing.T) {
		h0 := n.Chain.BlockHeight()
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"evm_mine","params":[]}`)
		var out struct {
			Result string `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.Equal(t, "0x0", out.Result)
		require.Equal(t, h0+1, n.Chain.BlockHeight())
	})

	t.Run("nodeInfo", func(t *testing.T) {
		raw := rpcPost(t, addr, `{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}`)
		var out struct {
			Result struct {
				Accounts []map[string]any `json:"accounts"`
				Fork     any              `json:"fork"`
			} `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.Len(t, out.Result.Accounts, 3)
		require.Nil(t, out.Result.Fork)
	})

	t.Run("dropTransaction", func(t *testing.T) {
		mgr, err := accounts.NewManager(n.Opts.Mnemonic, n.Opts.Accounts)
		require.NoError(t, err)
		tx, err := mgr.SignedGASTransfer(n.Chain, mgr.Accounts[0], mgr.Accounts[1].Signer.ScriptHash(), 1_0000_0000)
		require.NoError(t, err)
		require.NoError(t, n.NetServer.RelayTxn(tx))
		require.Equal(t, 1, n.Chain.GetMemPool().Count())

		raw := rpcPost(t, addr, fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"nanvil_dropTransaction","params":["0x%s"]}`, tx.Hash().StringLE()))
		var out struct {
			Result bool `json:"result"`
		}
		require.NoError(t, json.Unmarshal(raw, &out))
		require.True(t, out.Result)
		require.Equal(t, 0, n.Chain.GetMemPool().Count())
	})
}
