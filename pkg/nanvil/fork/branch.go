package fork

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/rpcclient"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// ContractInfo describes a deployed contract at the branch point.
type ContractInfo struct {
	ID   int32         `json:"id"`
	Hash util.Uint160  `json:"hash"`
	Name string        `json:"name"`
}

// Manifest holds fork branch metadata (WorkNet-compatible model).
type Manifest struct {
	RPCURL         string         `json:"rpcUrl"`
	NetworkMagic   uint32         `json:"networkMagic"`
	AddressVersion byte           `json:"addressVersion"`
	Index          uint32         `json:"index"`
	IndexHash      util.Uint256   `json:"indexHash"`
	RootHash       util.Uint256   `json:"rootHash"`
	Contracts      []ContractInfo `json:"contracts"`
}

// Save writes manifest to disk.
func (m *Manifest) Save(path string) error {
	raw, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// LoadManifest reads manifest from disk.
func LoadManifest(path string) (*Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// CreateBranch captures branch metadata from a remote StateService-enabled RPC node.
func CreateBranch(ctx context.Context, rpcURL string, index uint32) (*Manifest, error) {
	c, err := rpcclient.New(ctx, rpcURL, rpcclient.Options{})
	if err != nil {
		return nil, err
	}
	defer c.Close()

	ver, err := c.GetVersion()
	if err != nil {
		return nil, fmt.Errorf("getversion: %w", err)
	}
	if index == 0 {
		sh, err := c.GetStateHeight()
		if err != nil {
			return nil, fmt.Errorf("getstateheight: %w", err)
		}
		if sh.Validated != 0 {
			index = sh.Validated
		} else {
			index = sh.Local
		}
	}
	blockHash, err := c.GetBlockHash(index)
	if err != nil {
		return nil, fmt.Errorf("getblockhash: %w", err)
	}
	stateRoot, err := c.GetStateRootByHeight(index)
	if err != nil {
		return nil, fmt.Errorf("getstateroot: %w", err)
	}
	mgmtHash, err := managementHash(c)
	if err != nil {
		return nil, err
	}
	contracts, err := enumerateContracts(c, stateRoot.Root, mgmtHash)
	if err != nil {
		return nil, err
	}
	return &Manifest{
		RPCURL:         rpcURL,
		NetworkMagic:   uint32(ver.Protocol.Network),
		AddressVersion: ver.Protocol.AddressVersion,
		Index:          index,
		IndexHash:      blockHash,
		RootHash:       stateRoot.Root,
		Contracts:      contracts,
	}, nil
}

func managementHash(c *rpcclient.Client) (util.Uint160, error) {
	natives, err := c.GetNativeContracts()
	if err != nil {
		return util.Uint160{}, err
	}
	for i := range natives {
		if natives[i].Manifest.Name == nativenames.Management {
			return natives[i].Hash, nil
		}
	}
	return util.Uint160{}, fmt.Errorf("ContractManagement native contract not found")
}

func enumerateContracts(c *rpcclient.Client, root util.Uint256, mgmt util.Uint160) ([]ContractInfo, error) {
	var (
		out    []ContractInfo
		prefix = []byte{0x08}
		from   []byte
	)
	for {
		res, err := c.FindStates(root, mgmt, prefix, from, nil)
		if err != nil {
			return nil, err
		}
		for _, kv := range res.Results {
			if len(kv.Key) >= 5 && len(kv.Value) >= util.Uint160Size {
				id := int32(binary.BigEndian.Uint32(kv.Key[1:5]))
				hash, err := util.Uint160DecodeBytesBE(kv.Value[:util.Uint160Size])
				if err != nil {
					continue
				}
				out = append(out, ContractInfo{ID: id, Hash: hash})
			}
		}
		if !res.Truncated {
			break
		}
		if len(res.Results) == 0 {
			break
		}
		from = res.Results[len(res.Results)-1].Key
	}
	return out, nil
}

// BranchState tracks active fork for a running node.
type BranchState struct {
	Manifest *Manifest
	mu       sync.RWMutex
}

// NewBranchState wraps a manifest.
func NewBranchState(m *Manifest) *BranchState {
	return &BranchState{Manifest: m}
}

// Height returns branch index.
func (b *BranchState) Height() uint32 {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if b.Manifest == nil {
		return 0
	}
	return b.Manifest.Index
}
