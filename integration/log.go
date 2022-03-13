package integration

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Fantom-foundation/lachesis-base/kvdb"
)

type DBProducerWithSummary struct {
	kvdb.FlushableDBProducer

	written []uint64
	erased  []uint64
	start   time.Time
}

type StoreWithSummary struct {
	kvdb.Store

	written    []uint64
	erased     []uint64
	start      time.Time
	lastLogged time.Time
	log        io.Writer
}

type BatchWithSummary struct {
	kvdb.Batch

	written []uint64
	erased  []uint64
}

func WrapDatabaseWithSummary(db kvdb.FlushableDBProducer) kvdb.FlushableDBProducer {
	wrapper := &DBProducerWithSummary{
		FlushableDBProducer: db,
		written:             make([]uint64, 256),
		erased:              make([]uint64, 256),
		start:               time.Now(),
	}
	return wrapper
}

func (ds *StoreWithSummary) logTick() {
	if time.Since(ds.lastLogged) > time.Second*60 {
		ds.lastLogged = time.Now()
		fmt.Fprintf(ds.log, "\nLogging at %s\n", ds.lastLogged.String())
		fmt.Fprintf(ds.log, "Written:\n")
		total := uint64(0)
		for i := 0; i < 256; i++ {
			if ds.written[i] == 0 {
				continue
			}
			//fmt.Fprintf(ds.log, "0x%02x: %d\n", i, ds.written[i])
			total += ds.written[i]
			ds.written[i] = 0
		}
		fmt.Fprintf(ds.log, "Written total: %d\n", total)
		fmt.Fprintf(ds.log, "Erased:\n")
		total = uint64(0)
		for i := 0; i < 256; i++ {
			if ds.erased[i] == 0 {
				continue
			}
			//fmt.Fprintf(ds.log, "0x%02x: %d\n", i, ds.erased[i])
			total += ds.erased[i]
			ds.erased[i] = 0
		}
		fmt.Fprintf(ds.log, "Erased total: %d\n", total)
	}
}

func (ds *StoreWithSummary) Put(key, val []byte) error {
	ds.logTick()
	if len(key) == 0 {
		key = []byte("_")
	}
	ds.written[key[0]]++
	return ds.Store.Put(key, val)
}

func (ds *StoreWithSummary) Delete(key []byte) error {
	ds.logTick()
	ds.erased[key[0]]++
	return ds.Store.Delete(key)
}

func (ds *StoreWithSummary) NewBatch() kvdb.Batch {
	return &BatchWithSummary{
		Batch:   ds.Store.NewBatch(),
		written: ds.written,
		erased:  ds.erased,
	}
}

func (ds *BatchWithSummary) Put(key, val []byte) error {
	ds.written[key[0]]++
	return ds.Batch.Put(key, val)
}

func (ds *BatchWithSummary) Delete(key []byte) error {
	ds.erased[key[0]]++
	return ds.Batch.Delete(key)
}

func (db *DBProducerWithSummary) OpenDB(name string) (kvdb.Store, error) {
	ds, err := db.FlushableDBProducer.OpenDB(name)
	if err != nil {
		return nil, err
	}

	name = strings.ReplaceAll(name, "/", "_")
	_ = os.MkdirAll("/tmp/operadblogs/", 0700)
	f, _ := os.Create("/tmp/operadblogs/" + name)
	return &StoreWithSummary{
		Store:   ds,
		written: db.written,
		erased:  db.erased,
		start:   db.start,
		log:     f,
	}, nil
}
