package rpc_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	nanvilcfg "github.com/nspcc-dev/neo-go/pkg/nanvil/config"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/node"
	"go.uber.org/zap"
)

func TestNEP17TransfersVisibleWithNeoLineTimestamp(t *testing.T) {
	log := zap.NewNop()
	opts := nanvilcfg.DefaultStartOptions()
	opts.Port = 0
	n, err := node.NewDevNode(opts, log)
	if err != nil {
		t.Fatal(err)
	}
	if err := n.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer n.Shutdown()
	time.Sleep(300 * time.Millisecond)

	b64 := "AMLxFuSWP5gAAAAAAKDKEgAAAAAA0wAAAAGoya2kigsN+t2e5qJyqQAmpDkp2AEAWgsCAOH1BQwUXHTYphWH76cEIgA2r+OtW5tRJXUMFKjJraSKCw363Z7monKpACakOSnYFMAfDAh0cmFuc2ZlcgwUz3bii9AGLEpHjuNVYQETGfPPpNJBYn1bUgFCDEDDtz04zNItPLYrDISu+Vox1XmMCEAfXtoV3b3x/g9fvZaqjPJsCdWI0VXPxphDRRC33xILxgdSoidta5dFxIcgKAwhAwg3RX0I5eKijd86gA/K3GjQE/plZTlvw2C2SDzbr1uwQVbnsyc="
	data, _ := base64.StdEncoding.DecodeString(b64)
	tx, _ := transaction.NewTransactionFromBytes(data)
	if err := n.NetServer.RelayTxn(tx); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	since := time.Now().UnixMilli()
	body := fmt.Sprintf(`{"jsonrpc":"2.0","id":1,"method":"getnep17transfers","params":["NbJSJqVQCAtnGv85YqUNzcezsTgzooEPQD", %d]}`, since)
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
	if len(out.Result.Sent)+len(out.Result.Received) == 0 {
		t.Fatalf("expected transfers, got %s", raw)
	}
}
