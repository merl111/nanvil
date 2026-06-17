package ncast

import (
	"context"

	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
)

// RPCClientHolder wraps a client with its context cancel func.
type RPCClientHolder struct {
	Client *rpcclient.Client
	Cancel context.CancelFunc
}
