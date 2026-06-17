package fork_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestCreateBranchMockRPC(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getstateheight":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"localrootindex":10,"validatedrootindex":10}}`))
		case "getversion":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocol":{"network":860833102,"addressversion":53}}}`))
		case "getblockhash":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x` + util.Uint256{1}.StringBE() + `"}`))
		case "getstateroot":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"version":0,"index":10,"roothash":"0x` + util.Uint256{2}.StringBE() + `","witnesses":[]}}`))
		case "getnativecontracts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[{"id":-8,"hash":"0x` + util.Uint160{8}.StringBE() + `","manifest":{"name":"ContractManagement"}}]}`))
		case "findstates":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"results":[],"truncated":false}}`))
		default:
			t.Fatalf("unexpected method %s", req.Method)
		}
	}))
	defer srv.Close()

	m, err := fork.CreateBranch(context.Background(), srv.URL, 10)
	require.NoError(t, err)
	require.Equal(t, uint32(10), m.Index)
	require.NotEqual(t, util.Uint256{}, m.RootHash)
}
