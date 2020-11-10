package evmstore

/*
	In LRU cache data stored like value
*/

import (
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/core/types"
)

// SetReceipts stores transaction receipts.
func (s *Store) SetReceipts(n idx.Block, receipts types.Receipts) {
	receiptsStorage := make([]*types.ReceiptForStorage, len(receipts))
	for i, r := range receipts {
		receiptsStorage[i] = (*types.ReceiptForStorage)(r)
	}
	s.SetRawReceipts(n, receiptsStorage)
}

// SetStorageReceipts stores raw transaction receipts.
func (s *Store) SetRawReceipts(n idx.Block, receipts []*types.ReceiptForStorage) {
	s.rlp.Set(s.table.Receipts, n.Bytes(), receipts)

	// Add to LRU cache.
	if s.cache.Receipts != nil {
		s.cache.Receipts.Add(n, receipts)
	}
}

// GetReceipts returns stored transaction receipts.
func (s *Store) GetReceipts(n idx.Block) types.Receipts {
	var receiptsStorage *[]*types.ReceiptForStorage

	// Get data from LRU cache first.
	if s.cache.Receipts != nil {
		if c, ok := s.cache.Receipts.Get(n); ok {
			if receiptsStorage, ok = c.(*[]*types.ReceiptForStorage); !ok {
				if cv, ok := c.([]*types.ReceiptForStorage); ok {
					receiptsStorage = &cv
				}
			}
		}
	}

	if receiptsStorage == nil {
		receiptsStorage, _ = s.rlp.Get(s.table.Receipts, n.Bytes(), &[]*types.ReceiptForStorage{}).(*[]*types.ReceiptForStorage)
		if receiptsStorage == nil {
			return nil
		}

		// Add to LRU cache.
		if s.cache.Receipts != nil {
			s.cache.Receipts.Add(n, *receiptsStorage)
		}
	}

	receipts := make(types.Receipts, len(*receiptsStorage))
	for i, r := range *receiptsStorage {
		receipts[i] = (*types.Receipt)(r)
		// TODO
		//receipts[i].ContractAddress = r.ContractAddress
		//receipts[i].GasUsed = r.GasUsed
	}
	return receipts
}
