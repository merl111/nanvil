package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w
	fn()
	require.NoError(t, w.Close())
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestNewAppMetadata(t *testing.T) {
	app := NewApp()
	require.Equal(t, "nanvil", app.Name)
	require.NotEmpty(t, app.Commands)
}

func TestForkInfoCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fork.json")
	m := &fork.Manifest{
		RPCURL:       "http://example.com",
		NetworkMagic: 1,
		Index:        99,
		RootHash:     util.Uint256{7},
	}
	require.NoError(t, m.Save(path))

	app := NewApp()
	out := captureStdout(t, func() {
		require.NoError(t, app.Run([]string{"nanvil", "fork", "info", "--manifest", path}))
	})
	require.Contains(t, out, "http://example.com")
	require.Contains(t, out, "99")
}

func TestForkCreateCommand(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Method string `json:"method"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		w.Header().Set("Content-Type", "application/json")
		switch req.Method {
		case "getstateheight":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"localrootindex":1,"validatedrootindex":1}}`))
		case "getversion":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocol":{"network":860833102,"addressversion":53}}}`))
		case "getblockhash":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0x` + util.Uint256{1}.StringBE() + `"}`))
		case "getstateroot":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"version":0,"index":1,"roothash":"0x` + util.Uint256{2}.StringBE() + `","witnesses":[]}}`))
		case "getnativecontracts":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[{"id":-1,"hash":"0x` + util.Uint160{8}.StringBE() + `","manifest":{"name":"ContractManagement"}}]}`))
		case "findstates":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"results":[],"truncated":false}}`))
		default:
			fmt.Fprintf(w, `{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"%s"}}`, req.Method)
		}
	}))
	defer srv.Close()

	outPath := filepath.Join(t.TempDir(), "fork.json")
	app := NewApp()
	out := captureStdout(t, func() {
		require.NoError(t, app.Run([]string{"nanvil", "fork", "create", "--rpc-url", srv.URL, "--block", "1", "--out", outPath}))
	})
	require.Contains(t, out, "Fork manifest saved")
	_, err := fork.LoadManifest(outPath)
	require.NoError(t, err)
}

func TestPolicySyncCommand(t *testing.T) {
	app := NewApp()
	out := captureStdout(t, func() {
		require.NoError(t, app.Run([]string{"nanvil", "policy", "sync", "--rpc-url", "http://127.0.0.1:1"}))
	})
	require.Contains(t, out, "Policy sync")
}

func TestStartFlagsPresent(t *testing.T) {
	app := NewApp()
	var start *cli.Command
	for _, c := range app.Commands {
		if c.Name == "start" {
			start = c
			break
		}
	}
	require.NotNil(t, start)
	var hasHost, hasFork bool
	for _, f := range start.Flags {
		name := f.String()
		if strings.Contains(name, "--host") {
			hasHost = true
		}
		if strings.Contains(name, "--fork-url") {
			hasFork = true
		}
	}
	require.True(t, hasHost)
	require.True(t, hasFork)
}

func TestParseContractHashViaPrefetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":[]}`))
	}))
	defer srv.Close()

	dir := t.TempDir()
	m := &fork.Manifest{
		RPCURL:       srv.URL,
		NetworkMagic: 1,
		Index:        1,
		RootHash:     util.Uint256{1},
		Contracts:    []fork.ContractInfo{{ID: 1, Hash: util.Uint160{0xaa}}},
	}
	require.NoError(t, m.Save(filepath.Join(dir, "m.json")))

	app := NewApp()
	err := app.Run([]string{
		"nanvil", "fork", "prefetch",
		"--manifest", filepath.Join(dir, "m.json"),
		"--contract", "0x" + util.Uint160{0xaa}.StringLE(),
		"--cache-path", dir,
	})
	require.Error(t, err)
}
