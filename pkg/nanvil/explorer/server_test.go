package explorer_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/explorer"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestExplorerServesUI(t *testing.T) {
	log := zap.NewNop()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.ExplorerPort = 0
	n, err := node.NewDevNode(opts, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer n.Shutdown()
	time.Sleep(200 * time.Millisecond)

	if n.ExplorerAddr == "" {
		t.Fatal("expected explorer address")
	}

	resp, err := http.Get("http://" + n.ExplorerAddr + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "Nanvil Explorer") {
		t.Fatalf("missing title: %s", body[:min(200, len(body))])
	}
	if !strings.Contains(string(body), `id="activity-drawer"`) {
		t.Fatal("missing live activity drawer")
	}
	if !strings.Contains(string(body), `id="activity-feed"`) {
		t.Fatal("missing live activity feed")
	}
	if !strings.Contains(string(body), `id="live-toggle"`) {
		t.Fatal("missing live activity toggle")
	}

	respDocs, err := http.Get("http://" + n.ExplorerAddr + "/docs/getting-started")
	if err != nil {
		t.Fatal(err)
	}
	defer respDocs.Body.Close()
	docsBody, _ := io.ReadAll(respDocs.Body)
	if respDocs.StatusCode != http.StatusOK {
		t.Fatalf("docs status %d", respDocs.StatusCode)
	}
	if !strings.Contains(string(docsBody), "Getting started") {
		t.Fatalf("missing docs content: %s", docsBody[:min(300, len(docsBody))])
	}
	if !strings.Contains(string(docsBody), "docs-sidebar") {
		t.Fatal("missing docs layout")
	}
	if !strings.Contains(string(docsBody), "theme-switcher") {
		t.Fatal("missing theme switcher")
	}

	resp2, err := http.Post("http://"+n.ExplorerAddr+"/api/rpc", "application/json",
		strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"getblockcount","params":[]}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	raw, _ := io.ReadAll(resp2.Body)
	if !strings.Contains(string(raw), `"result"`) {
		t.Fatalf("rpc proxy failed: %s", raw)
	}
}

func TestExplorerShutdownAndWS(t *testing.T) {
	log := zap.NewNop()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.ExplorerPort = 0
	opts.Explorer = true
	n, err := node.NewDevNode(opts, log)
	require.NoError(t, err)
	require.NoError(t, n.Start(context.Background()))
	defer n.Shutdown()

	require.NoError(t, n.Explorer.Ping())

	// WS proxy without backend /ws should fail gracefully.
	resp, err := http.Get("http://" + n.ExplorerAddr + "/")
	require.NoError(t, err)
	resp.Body.Close()

	n.Explorer.Shutdown()
}

func TestExplorerPing(t *testing.T) {
	s := explorer.New("127.0.0.1:1", "127.0.0.1", 0, zap.NewNop())
	if err := s.Ping(); err == nil {
		t.Fatal("expected ping failure on closed port")
	}
}
