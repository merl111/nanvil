package persist

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/fork"
	"github.com/nspcc-dev/neo-go/pkg/nanvil/snapshot"
)

const formatVersion = "nanvil-chain-v1"

// ChainSnapshot is a full restorable nanvil chain state.
type ChainSnapshot struct {
	Format    string           `json:"format"`
	Height    uint32           `json:"height"`
	Snapshots []snapshot.State `json:"snapshots,omitempty"`
	Fork      *fork.Manifest   `json:"fork,omitempty"`
	Overlay   OverlaySnapshot  `json:"overlay,omitempty"`
	Storage   StorageSnapshot  `json:"storage"`
}

// OverlaySnapshot captures fork overlay writes and deletes.
type OverlaySnapshot struct {
	Writes  map[string]string `json:"writes,omitempty"`
	Deletes []string          `json:"deletes,omitempty"`
}

// StorageSnapshot captures the in-memory backing store.
type StorageSnapshot struct {
	Mem  map[string]string `json:"mem,omitempty"`
	Stor map[string]string `json:"stor,omitempty"`
}

// Save writes a full chain snapshot to path.
func Save(path string, height uint32, snaps []snapshot.State, base *storage.MemoryStore, overlay *fork.TrackingOverlay, manifest *fork.Manifest) error {
	if base == nil {
		return fmt.Errorf("persist: nil memory store")
	}
	mem, stor := base.Export()
	data := ChainSnapshot{
		Format:    formatVersion,
		Height:    height,
		Snapshots: append([]snapshot.State(nil), snaps...),
		Fork:      manifest,
		Storage: StorageSnapshot{
			Mem:  encodeMap(mem),
			Stor: encodeMap(stor),
		},
	}
	if overlay != nil {
		writes, deletes := overlay.Export()
		if len(writes) > 0 || len(deletes) > 0 {
			data.Overlay = OverlaySnapshot{
				Writes:  encodeMap(writes),
				Deletes: deletes,
			}
		}
	}
	raw, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

// Load reads a full chain snapshot from path.
func Load(path string) (*ChainSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data ChainSnapshot
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if data.Format != formatVersion {
		return nil, fmt.Errorf("persist: unsupported snapshot format %q", data.Format)
	}
	return &data, nil
}

// TryLoad reads path and returns a chain snapshot when the file uses the full format.
// Legacy metadata-only state files return (nil, nil).
func TryLoad(path string) (*ChainSnapshot, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var peek struct {
		Format string `json:"format"`
	}
	if err := json.Unmarshal(raw, &peek); err != nil {
		return nil, err
	}
	if peek.Format != formatVersion {
		return nil, nil
	}
	var data ChainSnapshot
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	return &data, nil
}

// Apply restores storage and overlay from a chain snapshot.
func Apply(snap *ChainSnapshot, base *storage.MemoryStore, overlay *fork.TrackingOverlay) error {
	if snap == nil || base == nil {
		return fmt.Errorf("persist: nil snapshot or store")
	}
	mem, err := decodeMap(snap.Storage.Mem)
	if err != nil {
		return fmt.Errorf("persist: decode mem: %w", err)
	}
	stor, err := decodeMap(snap.Storage.Stor)
	if err != nil {
		return fmt.Errorf("persist: decode stor: %w", err)
	}
	base.Import(mem, stor)
	if overlay != nil && (len(snap.Overlay.Writes) > 0 || len(snap.Overlay.Deletes) > 0) {
		writes, err := decodeMap(snap.Overlay.Writes)
		if err != nil {
			return fmt.Errorf("persist: decode overlay writes: %w", err)
		}
		overlay.Import(writes, snap.Overlay.Deletes)
	}
	return nil
}

func encodeMap(m map[string][]byte) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		hk := hex.EncodeToString([]byte(k))
		if len(v) == 0 {
			out[hk] = ""
			continue
		}
		out[hk] = hex.EncodeToString(v)
	}
	return out
}

func decodeMap(m map[string]string) (map[string][]byte, error) {
	if len(m) == 0 {
		return nil, nil
	}
	out := make(map[string][]byte, len(m))
	for k, v := range m {
		key, err := hex.DecodeString(k)
		if err != nil {
			return nil, fmt.Errorf("key %q: %w", k, err)
		}
		var val []byte
		if v != "" {
			val, err = hex.DecodeString(v)
			if err != nil {
				return nil, fmt.Errorf("value for key %q: %w", k, err)
			}
		}
		out[string(key)] = val
	}
	return out, nil
}
