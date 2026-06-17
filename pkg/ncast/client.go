package ncast

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/encoding/address"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

const DefaultRPC = "http://127.0.0.1:8545"

var nativeAliases = map[string]string{
	"gas":              nativenames.Gas,
	"neo":              nativenames.Neo,
	"policy":           nativenames.Policy,
	"oracle":           nativenames.Oracle,
	"notary":           nativenames.Notary,
	"cryptolib":        nativenames.CryptoLib,
	"stdlib":           nativenames.StdLib,
	"ledger":           nativenames.Ledger,
	"management":       nativenames.Management,
	"contractmanagement": nativenames.Management,
	"rolemanagement":   nativenames.Designation,
	"treasury":         nativenames.Treasury,
}

// RPCClient opens and initializes an RPC client.
func RPCClient(ctx context.Context, endpoint string) (*rpcclient.Client, error) {
	if endpoint == "" {
		endpoint = DefaultRPC
	}
	c, err := rpcclient.New(ctx, endpoint, rpcclient.Options{
		RequestTimeout: 30 * time.Second,
		DialTimeout:    10 * time.Second,
	})
	if err != nil {
		return nil, err
	}
	if err := c.Init(); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

// WSURL converts an HTTP RPC endpoint to a websocket URL.
func WSURL(httpEndpoint string) (string, error) {
	u, err := url.Parse(httpEndpoint)
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "http":
		u.Scheme = "ws"
	case "https":
		u.Scheme = "wss"
	default:
		return "", fmt.Errorf("unsupported RPC scheme %q", u.Scheme)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/ws"
	return u.String(), nil
}

// ResolveContract resolves a contract by native name, address, or script hash.
func ResolveContract(c *rpcclient.Client, s string) (util.Uint160, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return util.Uint160{}, fmt.Errorf("empty contract")
	}
	if name, ok := nativeAliases[strings.ToLower(s)]; ok {
		return lookupNative(c, name)
	}
	if nativenames.IsValid(s) {
		return lookupNative(c, s)
	}
	if strings.HasPrefix(s, "N") {
		return address.StringToUint160(s)
	}
	return util.Uint160DecodeStringLE(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X"))
}

func lookupNative(c *rpcclient.Client, name string) (util.Uint160, error) {
	natives, err := c.GetNativeContracts()
	if err != nil {
		return util.Uint160{}, err
	}
	for _, ctr := range natives {
		if ctr.Manifest.Name == name {
			return ctr.Hash, nil
		}
	}
	return util.Uint160{}, fmt.Errorf("native contract %q not found", name)
}

// ResolveHash160 resolves an address or script hash.
func ResolveHash160(s string) (util.Uint160, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "N") {
		return address.StringToUint160(s)
	}
	return util.Uint160DecodeStringLE(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X"))
}

// ResolveHash256 resolves a uint256 hash.
func ResolveHash256(s string) (util.Uint256, error) {
	s = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s), "0x"), "0X")
	return util.Uint256DecodeStringLE(s)
}
