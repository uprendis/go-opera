package compactdb

import (
	"bytes"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Fantom-foundation/lachesis-base/kvdb"
	"github.com/Fantom-foundation/lachesis-base/kvdb/table"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/status-im/keycard-go/hexutils"
)

func isEmptyDB(db kvdb.Iteratee) bool {
	it := db.NewIterator(nil, nil)
	defer it.Release()
	return !it.Next()
}

func firstKey(db kvdb.Store) []byte {
	it := db.NewIterator(nil, nil)
	defer it.Release()
	if !it.Next() {
		return nil
	}
	return it.Key()
}

func lastKey(db kvdb.Store) []byte {
	var start []byte
	for {
		for b := 0xff; b >= 0; b-- {
			if !isEmptyDB(table.New(db, append(start, byte(b)))) {
				start = append(start, byte(b))
				break
			}
			if b == 0 {
				return start
			}
		}
	}
}

func addToPrefix(prefix *big.Int, diff *big.Int, size int) []byte {
	endBn := new(big.Int).Set(prefix)
	endBn.Add(endBn, diff)
	if len(endBn.Bytes()) > size {
		// overflow
		return bytes.Repeat([]byte{0xff}, size)
	}
	end := endBn.Bytes()
	res := make([]byte, size-len(end), size)
	return append(res, end...)
}

type loggedStore struct {
	kvdb.Store
	lastLog time.Time
	name    string

	currentOp atomic.Value

	wg   sync.WaitGroup
	quit chan struct{}
}

func (s *loggedStore) Compact(start []byte, limit []byte) error {
	s.currentOp.Store(limit)
	err := s.Store.Compact(start, limit)
	if err != nil {
		log.Error("Compaction error", "name", s.name, "err", err)
		return err
	}
	return nil
}

func (s *loggedStore) StartLogging() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(16 * time.Second)
		for {
			select {
			case <-ticker.C:
				untilI := s.currentOp.Load()
				if untilI != nil {
					until := untilI.([]byte)
					untilStr := hexutils.BytesToHex(until)
					if until == nil {
						untilStr = "end"
					}
					log.Info("Compacting DB", "name", s.name, "until", untilStr)
				}
			case <-s.quit:
				return
			}
		}
	}()
}

func (s *loggedStore) StopLogging() {
	close(s.quit)
	s.wg.Wait()
}

func Compact(unprefixedDB kvdb.Store, loggingName string) error {
	loggedDB := &loggedStore{
		Store: unprefixedDB,
		name:  loggingName,
		quit:  make(chan struct{}),
	}
	loggedDB.StartLogging()
	defer loggedDB.StopLogging()

	for b := 0; b < 256; b++ {
		prefixed := table.New(loggedDB, []byte{byte(b)})
		first := firstKey(prefixed)
		if first == nil {
			continue
		}
		last := lastKey(prefixed)
		if last == nil {
			continue
		}
		println(b, hexutils.BytesToHex(first), hexutils.BytesToHex(last))
		keySize := len(last)
		if keySize < len(first) {
			keySize = len(first)
		}
		first = common.RightPadBytes(first, keySize-len(first))
		last = common.RightPadBytes(last, keySize-len(last))
		println(b, hexutils.BytesToHex(first), hexutils.BytesToHex(last))
		firstBn := new(big.Int).SetBytes(first)
		lastBn := new(big.Int).SetBytes(last)
		diff := new(big.Int).Sub(lastBn, firstBn)
		println(b, firstBn.String(), lastBn.String(), diff.String())
		if diff.Cmp(big.NewInt(10000)) < 0 {
			// short circuit if too few keys
			err := prefixed.Compact(nil, nil)
			if err != nil {
				return err
			}
			continue
		}
		var prev []byte
		for i := 32; i >= 1; i-- {
			until := addToPrefix(firstBn, new(big.Int).Div(diff, big.NewInt(int64(i))), keySize)
			err := prefixed.Compact(prev, until)
			if err != nil {
				return err
			}
			prev = common.CopyBytes(until)
		}
	}
	return nil
}
