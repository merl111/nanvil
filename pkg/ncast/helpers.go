package ncast

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/urfave/cli/v2"
)

// ResolveClientHolder runs fn with a connected RPC client.
func ResolveClientHolder(ctx *cli.Context, fn func(*RPCClientHolder) (any, error)) (any, error) {
	cctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	c, err := RPCClient(cctx, ctx.String("rpc"))
	if err != nil {
		return nil, err
	}
	defer c.Close()
	return fn(&RPCClientHolder{Client: c, Cancel: cancel})
}

// ParseHash256 parses a block/tx hash.
func ParseHash256(s string) (util.Uint256, error) {
	return ResolveHash256(s)
}

// RawRPC performs a raw JSON-RPC request.
func RawRPC(endpoint, method string, params []any) (json.RawMessage, error) {
	if params == nil {
		params = []any{}
	}
	body, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	resp, err := http.Post(endpoint, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var out struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", out.Error.Code, out.Error.Message)
	}
	return out.Result, nil
}
