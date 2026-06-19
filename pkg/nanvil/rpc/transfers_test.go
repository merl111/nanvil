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

func TestNEP17TransfersVisibleWithNeoLineTimestamp(t *testing.T) {
	log := zap.NewNop()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	opts.Explorer = false
	opts.NoMining = true
	opts.Accounts = 5

	n, err := node.NewDevNode(opts, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer n.Shutdown()

	mgr, err := accounts.NewManager(opts.Mnemonic, opts.Accounts)
	if err != nil {
		t.Fatal(err)
	}
	sender := mgr.Accounts[4]
	tx, err := mgr.SignedGASTransfer(n.Chain, sender, mgr.Accounts[0].Signer.ScriptHash(), 100_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.NetServer.RelayTxn(tx); err != nil {
		t.Fatal(err)
	}
	if err := n.MineBlock(1); err != nil {
		t.Fatal(err)
	}

	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"getnep17transfers","params":["%s", 0]}`, sender.Address)
	resp, err := http.Post("http://"+n.RPCAddr, "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)

	var out struct {
		Result struct {
			Sent     []json.RawMessage `json:"sent"`
			Received []json.RawMessage `json:"received"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, raw)
	}
	require.NotEmpty(t, out.Result.Sent, "expected sent transfers, got %s", raw)
}
