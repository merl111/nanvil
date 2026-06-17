package fork

import (
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// TransitionInfo describes consensus takeover at the branch point.
type TransitionInfo struct {
	BranchIndex      uint32
	BranchHash       util.Uint256
	LocalValidator   util.Uint160
	TransitionHeight uint32
}

// BuildTransition computes metadata for local single-validator takeover after fork.
func BuildTransition(m *Manifest, validatorPub *keys.PublicKey) TransitionInfo {
	return TransitionInfo{
		BranchIndex:      m.Index,
		BranchHash:       m.IndexHash,
		LocalValidator:   validatorPub.GetScriptHash(),
		TransitionHeight: m.Index + 1,
	}
}

// ApplyTransition patches committee metadata and returns transition info for the local validator.
func ApplyTransition(m *Manifest, validatorPub *keys.PublicKey) TransitionInfo {
	return BuildTransition(m, validatorPub)
}
