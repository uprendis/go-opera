package evmstore

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"testing"

	"github.com/Fantom-foundation/lachesis-base/common/bigendian"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/kvdb/pebble"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/syndtr/goleveldb/leveldb/opt"

	"github.com/Fantom-foundation/go-opera/logger"
)

func equalStorageReceipts(t *testing.T, expect, got []*types.ReceiptForStorage) {
	assert.EqualValues(t, len(expect), len(got))
	for i := range expect {
		assert.EqualValues(t, expect[i].CumulativeGasUsed, got[i].CumulativeGasUsed)
		assert.EqualValues(t, expect[i].Logs, got[i].Logs)
		assert.EqualValues(t, expect[i].Status, got[i].Status)
	}
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func TestStoreGetCachedReceipts(t *testing.T) {
	os.RemoveAll("/tmp/123")
	os.Mkdir("/tmp/123", 0700)
	db, _ := pebble.New("/tmp/123", opt.MiB * 650, 1000, nil, nil)

	buf := make([]byte, 32)

	for i := 0; i < 1000000000; i++ {
		rand.Read(buf)
		db.Put(bigendian.Uint64ToBytes(uint64(i)), buf)
		db.Get(bigendian.Uint64ToBytes(uint64(rand.Intn(i+1))))
		it := db.NewIterator(nil, bigendian.Uint64ToBytes(uint64(rand.Intn(i+1))))
		it.Next()
		it.Release()
		if i % 100000 == 0 {
			PrintMemUsage()
		}
	}

	db.Close()
	os.RemoveAll("\"/tmp/123\"")
	return

	logger.SetTestMode(t)

	block, expect := fakeReceipts()
	store := cachedStore()
	store.SetRawReceipts(block, expect)

	got, _ := store.GetRawReceipts(block)
	assert.EqualValues(t, expect, got)
}

func TestStoreGetNonCachedReceipts(t *testing.T) {
	logger.SetTestMode(t)

	block, expect := fakeReceipts()
	store := nonCachedStore()
	store.SetRawReceipts(block, expect)

	got, _ := store.GetRawReceipts(block)
	equalStorageReceipts(t, expect, got)
}

func BenchmarkStoreGetRawReceipts(b *testing.B) {
	logger.SetTestMode(b)

	b.Run("cache on", func(b *testing.B) {
		benchStoreGetRawReceipts(b, cachedStore())
	})
	b.Run("cache off", func(b *testing.B) {
		benchStoreGetRawReceipts(b, nonCachedStore())
	})
}

func benchStoreGetRawReceipts(b *testing.B, store *Store) {
	block, receipt := fakeReceipts()

	store.SetRawReceipts(block, receipt)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if v, _ := store.GetRawReceipts(block); v == nil {
			b.Fatal("invalid result")
		}
	}
}

func BenchmarkStoreSetRawReceipts(b *testing.B) {
	logger.SetTestMode(b)

	b.Run("cache on", func(b *testing.B) {
		benchStoreSetRawReceipts(b, cachedStore())
	})
	b.Run("cache off", func(b *testing.B) {
		benchStoreSetRawReceipts(b, nonCachedStore())
	})
}

func benchStoreSetRawReceipts(b *testing.B, store *Store) {
	block, receipt := fakeReceipts()

	for i := 0; i < b.N; i++ {
		store.SetRawReceipts(block, receipt)
	}
}

func fakeReceipts() (idx.Block, []*types.ReceiptForStorage) {
	return idx.Block(1),
		[]*types.ReceiptForStorage{
			{
				PostState:         nil,
				Status:            0,
				CumulativeGasUsed: 0,
				Bloom:             types.Bloom{},
				Logs:              []*types.Log{},
				TxHash:            common.Hash{},
				ContractAddress:   common.Address{},
				GasUsed:           0,
				BlockHash:         common.Hash{},
				BlockNumber:       nil,
				TransactionIndex:  0,
			},
		}
}
