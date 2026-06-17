package core

import (
	"encoding/binary"
	"errors"
	"fmt"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// BootstrapFork positions the chain at a fork branch block without replaying history.
// It must be called before Blockchain.Run.
func (bc *Blockchain) BootstrapFork(hdr *block.Header, blk *block.Block, root util.Uint256) error {
	if bc.isRunning.Load().(bool) {
		return errors.New("can't bootstrap fork on running blockchain")
	}
	if hdr.Index != blk.Index || hdr.Hash() != blk.Hash() {
		return errors.New("fork header/block mismatch")
	}
	if bc.BlockHeight() != 0 {
		return fmt.Errorf("fork bootstrap expects genesis height 0, got %d", bc.BlockHeight())
	}

	cache := bc.dao.GetPrivate()
	if err := cache.StoreAsBlock(blk, nil, nil); err != nil {
		return fmt.Errorf("store fork block: %w", err)
	}
	if err := cache.StoreHeader(hdr); err != nil {
		return fmt.Errorf("store fork header: %w", err)
	}
	cache.StoreAsCurrentBlock(blk)
	cache.PutCurrentHeader(hdr.Hash(), hdr.Index)

	putForkStateRoot(cache.Store, &state.MPTRoot{Index: hdr.Index, Root: root})
	if _, err := cache.Persist(); err != nil {
		return fmt.Errorf("persist fork state root: %w", err)
	}
	if err := bc.stateRoot.Init(hdr.Index); err != nil {
		return fmt.Errorf("init state root: %w", err)
	}
	if _, err := bc.dao.GetPrivate().Persist(); err != nil {
		return fmt.Errorf("persist fork bootstrap: %w", err)
	}

	bc.bootstrapForkHeaderIndex(hdr)

	if err := bc.resetRAMState(hdr.Index, false); err != nil {
		return fmt.Errorf("reset ram state: %w", err)
	}
	return nil
}

func putForkStateRoot(cache *storage.MemCachedStore, sr *state.MPTRoot) {
	key := make([]byte, 5)
	key[0] = byte(storage.DataMPTAux)
	binary.BigEndian.PutUint32(key[1:], sr.Index)
	w := io.NewBufBinWriter()
	sr.EncodeBinary(w.BinWriter)
	cache.Put(key, w.Bytes())
	data := make([]byte, 4)
	binary.LittleEndian.PutUint32(data, sr.Index)
	cache.Put([]byte{byte(storage.DataMPTAux), 0x02}, data)
}

func (bc *Blockchain) bootstrapForkHeaderIndex(hdr *block.Header) {
	bc.HeaderHashes.dao = bc.dao
	if bc.HeaderHashes.cache == nil {
		bc.HeaderHashes.cache, _ = lru.New[uint32, []util.Uint256](pagesCache)
	}
	bc.HeaderHashes.lock.Lock()
	defer bc.HeaderHashes.lock.Unlock()

	page := (hdr.Index / headerBatchCount) * headerBatchCount
	idxInBatch := hdr.Index % headerBatchCount
	bc.HeaderHashes.storedHeaderCount = page
	bc.HeaderHashes.previous = make([]util.Uint256, headerBatchCount)
	bc.HeaderHashes.latest = make([]util.Uint256, idxInBatch+1)
	bc.HeaderHashes.latest[idxInBatch] = hdr.Hash()
	updateHeaderHeightMetric(hdr.Index)
}
