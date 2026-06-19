package integration_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

const forkTestMagic = uint32(860833102)

func forkTestBlock(t *testing.T, index uint32) (util.Uint256, *block.Block) {
	t.Helper()
	val, err := accounts.NewValidatorSigner()
	require.NoError(t, err)
	blk := &block.Block{
		Header: block.Header{
			PrevHash:      util.Uint256{byte(index)},
			Index:         index,
			Timestamp:     uint64(index) * 1000,
			NextConsensus: val.ScriptHash(),
			Script: transaction.Witness{
				VerificationScript: val.Script(),
			},
		},
	}
	blk.RebuildMerkleRoot()
	blk.Script.InvocationScript = val.SignHashable(forkTestMagic, blk)
	return util.Uint256{2}, blk
}

func forkTestRPCServer(t *testing.T, forkBlk *block.Block, root util.Uint256) *httptest.Server {
	t.Helper()
	blockRaw, err := testserdes.EncodeBinary(forkBlk)
	require.NoError(t, err)
	blockB64 := base64.StdEncoding.EncodeToString(blockRaw)
	index := forkBlk.Index
	blockHash := forkBlk.Hash()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getstateheight":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"localrootindex":%d,"validatedrootindex":%d}}`, index, index)))
		case "getversion":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":{"protocol":{"network":%d,"addressversion":53}}}`, forkTestMagic)))
		case "getblockhash":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x` + blockHash.StringBE() + `"}`))
		case "getstateroot":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"version":0,"index":` + fmt.Sprint(index) + `,"roothash":"0x` + root.StringBE() + `","witnesses":[]}}`))
		case "getnativecontracts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[{"id":-1,"hash":"0x` + util.Uint160{8}.StringBE() + `","manifest":{"name":"ContractManagement"}}]}`))
		case "findstates":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"results":[],"truncated":false}}`))
		case "getblock":
			_, _ = w.Write([]byte(fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"result":"%s"}`, blockB64)))
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
}

func TestDevNodeForkMode(t *testing.T) {
	const index uint32 = 10
	root, forkBlk := forkTestBlock(t, index)
	srv := forkTestRPCServer(t, forkBlk, root)
	defer srv.Close()

	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.Explorer = false
	opts.NoMining = true
	opts.Accounts = 2
	opts.ForkURL = srv.URL
	opts.ForkBlock = index
	opts.ForkCachePath = t.TempDir()

	dev, err := node.NewDevNode(opts, nil)
	require.NoError(t, err)
	require.NotNil(t, dev.Fork)
	require.Equal(t, index, dev.Fork.Index)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	require.NoError(t, dev.Start(ctx))
	defer dev.Shutdown()

	require.Equal(t, index, dev.Chain.BlockHeight())

	body := `{"jsonrpc":"2.0","id":1,"method":"nanvil_nodeInfo","params":[]}`
	resp, err := http.Post("http://"+dev.RPCAddr, "application/json", strings.NewReader(body))
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	resp.Body.Close()

	var out struct {
		Result struct {
			Fork map[string]any `json:"fork"`
		} `json:"result"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, float64(index), out.Result.Fork["index"])
	require.Equal(t, srv.URL, out.Result.Fork["rpcUrl"])
}
