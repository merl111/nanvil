package rpc_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	nanvilrpc "github.com/nspcc-dev/neo-go/pkg/nanvil/rpc"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/nspcc-dev/neo-go/pkg/services/rpcsrv"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestResolveUnknownTransactionFormat(t *testing.T) {
	h, err := util.Uint256DecodeStringLE("b11792ce56557b18620b96223c1d41d0cb50c3a5363b6bb2ea531a826d5d7cd5")
	require.NoError(t, err)
	rerr := nanvilrpc.ResolveUnknownTransaction(h)
	require.Contains(t, rerr.Message, h.StringLE())
}

func TestUnknownTransactionReturnsNullOnNanvil(t *testing.T) {
	log := zap.NewNop()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	n, err := node.NewDevNode(opts, log)
	require.NoError(t, err)
	require.NoError(t, n.Start(context.Background()))
	defer n.Shutdown()
	require.True(t, rpcsrv.HasUnknownTransactionResolver())
	time.Sleep(200 * time.Millisecond)

	body := `{"jsonrpc":"2.0","id":1,"method":"getrawtransaction","params":["0xb11792ce56557b18620b96223c1d41d0cb50c3a5363b6bb2ea531a826d5d7cd5", true]}`
	resp, err := http.Post("http://"+n.RPCAddr, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var out struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, json.RawMessage("null"), out.Result)
	require.Nil(t, out.Error)
}
