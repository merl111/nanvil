package fork

import (
	"context"
	"fmt"
	"math/big"

	"github.com/nspcc-dev/neo-go/pkg/core"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativeids"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/accounts"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
)

// BootstrapOptions configures fork chain bootstrap.
type BootstrapOptions struct {
	Manifest *Manifest
	Remote   *RemoteStateStore
	Overlay  *TrackingOverlay
	Chain    *core.Blockchain
	Accounts *accounts.Manager
	Balance  int64
}

// Bootstrap positions the local chain at the fork branch and prepares writable state.
func Bootstrap(ctx context.Context, cfg BootstrapOptions) error {
	if cfg.Manifest == nil || cfg.Remote == nil || cfg.Chain == nil {
		return fmt.Errorf("fork bootstrap: missing manifest, remote, or chain")
	}
	m := cfg.Manifest
	validator, err := accounts.NewValidatorSigner()
	if err != nil {
		return err
	}
	_ = validator
	valAcc, err := accounts.ValidatorPublicKeyHex()
	if err != nil {
		return err
	}
	pub, err := keys.NewPublicKeyFromString(valAcc)
	if err != nil {
		return err
	}

	if err := patchCommittee(cfg.Overlay, pub); err != nil {
		return fmt.Errorf("patch committee: %w", err)
	}
	if _, ok := cfg.Remote.ContractHash(nativeids.ContractManagement); !ok {
		return fmt.Errorf("management contract hash not found")
	}
	if cfg.Accounts != nil && cfg.Overlay != nil {
		if err := fundForkAccounts(cfg.Overlay, cfg.Accounts, cfg.Balance); err != nil {
			return fmt.Errorf("fund accounts: %w", err)
		}
	}

	blk, err := cfg.Remote.GetBlockByHash(m.IndexHash)
	if err != nil {
		return fmt.Errorf("get fork block: %w", err)
	}
	if blk.Index != m.Index {
		return fmt.Errorf("fork block index mismatch: expected %d, got %d", m.Index, blk.Index)
	}

	if err := cfg.Chain.BootstrapFork(&blk.Header, blk, m.RootHash); err != nil {
		return fmt.Errorf("bootstrap fork: %w", err)
	}
	return nil
}

func patchCommittee(overlay *TrackingOverlay, validator *keys.PublicKey) error {
	if overlay == nil {
		return nil
	}
	const neoID int32 = nativeids.NeoToken
	prefixCommittee := []byte{14}
	item := stackitem.NewArray([]stackitem.Item{
		stackitem.NewStruct([]stackitem.Item{
			stackitem.NewByteArray(validator.Bytes()),
			stackitem.NewBigInteger(big.NewInt(0)),
		}),
	})
	ctx := stackitem.NewSerializationContext()
	data, err := ctx.Serialize(item, false)
	if err != nil {
		return err
	}
	PutStorageOverlay(overlay, neoID, prefixCommittee, data)
	return nil
}

func fundForkAccounts(overlay *TrackingOverlay, mgr *accounts.Manager, balance int64) error {
	valHash := mgr.Validator.ScriptHash()
	if err := PutGasBalanceOverlay(overlay, valHash, balance*int64(len(mgr.Accounts)+1)); err != nil {
		return err
	}
	for _, acc := range mgr.Accounts {
		if err := PutGasBalanceOverlay(overlay, acc.Signer.ScriptHash(), balance); err != nil {
			return err
		}
	}
	return nil
}
