package main

import "github.com/nspcc-dev/neo-go/pkg/interop/runtime"

// Minimal contract used by scripts/test-explorer-contracts.sh.
func _deploy(_ any, isUpdate bool) {
	if !isUpdate {
		runtime.Log("explorer deploy test")
	}
}

// GetValue is called by the explorer contracts test script.
func GetValue() string {
	return "explorer-test-ok"
}
