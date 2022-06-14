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

	start time.Time
}

type StoreWithSummary struct {
	kvdb.Store

	written    []uint64
	erased     []uint64
	get        []uint64
	has        []uint64
	start      time.Time
	lastLogged time.Time
	log        io.Writer
	name       string
}

type BatchWithSummary struct {
	kvdb.Batch

	prePut    func(key []byte)
	preDelete func(key []byte)
}

func WrapDatabaseWithSummary(db kvdb.FlushableDBProducer) kvdb.FlushableDBProducer {
	wrapper := &DBProducerWithSummary{
		FlushableDBProducer: db,
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
		fmt.Fprintf(ds.log, "Get:\n")
		total = uint64(0)
		for i := 0; i < 256; i++ {
			if ds.get[i] == 0 {
				continue
			}
			//fmt.Fprintf(ds.log, "0x%02x: %d\n", i, ds.get[i])
			total += ds.get[i]
			ds.get[i] = 0
		}
		fmt.Fprintf(ds.log, "Get total: %d\n", total)
		fmt.Fprintf(ds.log, "Has:\n")
		total = uint64(0)
		for i := 0; i < 256; i++ {
			if ds.has[i] == 0 {
				continue
			}
			//fmt.Fprintf(ds.log, "0x%02x: %d\n", i, ds.has[i])
			total += ds.has[i]
			ds.has[i] = 0
		}
		fmt.Fprintf(ds.log, "Has total: %d\n", total)
	}
}

func (ds *StoreWithSummary) prePut(key []byte) {
	//ds.logTick()
	if len(key) == 0 {
		ds.written[[]byte("_")[0]]++
	} else {
		ds.written[key[0]]++
	}
}

func (ds *StoreWithSummary) preDelete(key []byte) {
	//ds.logTick()
	if len(key) == 0 {
		ds.erased[[]byte("_")[0]]++
	} else {
		ds.erased[key[0]]++
	}
}

func (ds *StoreWithSummary) preGet(key []byte) {
	//ds.logTick()
	if len(key) == 0 {
		ds.get[[]byte("_")[0]]++
	} else {
		ds.get[key[0]]++
	}
}

func (ds *StoreWithSummary) preHas(key []byte) {
	//ds.logTick()
	if len(key) == 0 {
		ds.has[[]byte("_")[0]]++
	} else {
		ds.has[key[0]]++
	}
}

func (ds *StoreWithSummary) Close() error {
	ds.logTick()
	return ds.Store.Close()
}

func (ds *StoreWithSummary) Put(key, val []byte) error {
	ds.prePut(key)
	return ds.Store.Put(key, val)
}

func (ds *StoreWithSummary) Delete(key []byte) error {
	ds.preDelete(key)
	return ds.Store.Delete(key)
}

func (ds *StoreWithSummary) Get(key []byte) ([]byte, error) {
	ds.preGet(key)
	return ds.Store.Get(key)
}

func (ds *StoreWithSummary) Has(key []byte) (bool, error) {
	ds.preHas(key)
	return ds.Store.Has(key)
}

func (ds *StoreWithSummary) NewBatch() kvdb.Batch {
	return &BatchWithSummary{
		Batch:     ds.Store.NewBatch(),
		prePut:    ds.prePut,
		preDelete: ds.preDelete,
	}
}

func (ds *BatchWithSummary) Put(key, val []byte) error {
	ds.prePut(key)
	return ds.Batch.Put(key, val)
}

func (ds *BatchWithSummary) Delete(key []byte) error {
	ds.preDelete(key)
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
		get:     make([]uint64, 256),
		has:     make([]uint64, 256),
		written: make([]uint64, 256),
		erased:  make([]uint64, 256),
		start:   db.start,
		log:     f,
		name:    name,
	}, nil
}
