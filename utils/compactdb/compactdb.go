package compactdb

import (
	"errors"
	"math/big"
	"strconv"

	"github.com/Fantom-foundation/lachesis-base/kvdb"
	"github.com/Fantom-foundation/lachesis-base/kvdb/table"

	"github.com/Fantom-foundation/go-opera/utils"
)

type contCompacter struct {
	kvdb.Store
	prev []byte
}

func (s *contCompacter) Compact(_ []byte, limit []byte) error {
	err := s.Store.Compact(s.prev, limit)
	if err != nil {
		return err
	}
	s.prev = limit
	return nil
}

func compact(db *contCompacter, prefix []byte, iters int) error {
	nonEmptyPrefixes := make([]byte, 0, 256)
	for b := 0; b < 256; b++ {
		if !isEmptyDB(table.New(db, append(prefix, byte(b)))) {
			nonEmptyPrefixes = append(nonEmptyPrefixes, byte(b))
		}
	}
	if len(nonEmptyPrefixes) == 0 {
		return nil
	}
	if len(nonEmptyPrefixes) != 1 && len(nonEmptyPrefixes) < 50 {
		// if data is split among tables, then compact each table individually
		for _, b := range nonEmptyPrefixes {
			if err := compact(db, append(prefix, b), iters); err != nil {
				return err
			}
		}
		return nil
	}

	prefixed := utils.NewTableOrSelf(db, append(prefix))
	first, _, diff := keysRange(prefixed)
	if diff.Cmp(big.NewInt(int64(iters+10000))) < 0 {
		// skip if too few keys and compact it along with next range
		return nil
	}
	firstBn := new(big.Int).SetBytes(first)
	for i := iters; i >= 1; i-- {
		until := addToKey(firstBn, new(big.Int).Div(diff, big.NewInt(int64(i))), len(first))
		if err := prefixed.Compact(nil, until); err != nil {
			return err
		}
	}
	return nil
}

func Compact(db kvdb.Store, loggingName string, sizePerIter uint64) error {
	loggedDB := &loggedCompacter{
		Store: db,
		name:  loggingName,
		quit:  make(chan struct{}),
	}
	loggedDB.StartLogging()
	defer loggedDB.StopLogging()

	// scale iterations number based on total DB size and sizePerIter
	diskSizeStr, err := db.Stat("disk.size")
	if err != nil {
		return err
	}
	var nDiskSize int64
	if nDiskSize, err = strconv.ParseInt(diskSizeStr, 10, 64); err != nil || nDiskSize < 0 {
		return errors.New("bad syntax of disk size entry")
	}

	iters := uint64(nDiskSize) / sizePerIter
	if iters <= 1 {
		// short circuit if too few iterations
		return loggedDB.Compact(nil, nil)
	}

	compacter := &contCompacter{
		Store: loggedDB,
	}
	err = compact(compacter, []byte{}, int(iters))
	if err != nil {
		return err
	}
	return compacter.Compact(nil, nil)
}
