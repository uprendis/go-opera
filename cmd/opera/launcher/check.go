package launcher

import (
	"bytes"
	"time"

	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/kvdb"
	"github.com/Fantom-foundation/lachesis-base/kvdb/leveldb"
	"github.com/Fantom-foundation/lachesis-base/kvdb/pebble"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"gopkg.in/urfave/cli.v1"

	"github.com/Fantom-foundation/go-opera/inter"
)

func openDB(path string) kvdb.Store {
	var db kvdb.Store
	var err error
	db, err = leveldb.New(path, 400 * opt.MiB, utils.MakeDatabaseHandles()/4, nil, nil)
	if err != nil {
		db, err = pebble.New(path, 40 * opt.MiB, utils.MakeDatabaseHandles()/4, nil, nil)
	}
	if err != nil {
		panic(err)
	}
	return db
}

func checkEvm(ctx *cli.Context) error {
	if len(ctx.Args()) != 2 {
		utils.Fatalf("This command requires 2 arguments.")
	}

	adb := openDB(ctx.Args()[0])
	bdb := openDB(ctx.Args()[1])

	it := adb.NewIterator(nil, nil)
	for it.Next() {
		val, _ := bdb.Get(it.Key())
		if val == nil {
			println("missing", common.Bytes2Hex(it.Key()), common.Bytes2Hex(it.Value()))
		} else if !bytes.Equal(val, it.Value()) {
			println("mismatch", common.Bytes2Hex(it.Key()), common.Bytes2Hex(it.Value()), common.Bytes2Hex(val))
		}
	}

	return nil

	if len(ctx.Args()) != 0 {
		utils.Fatalf("This command doesn't require an argument.")
	}

	cfg := makeAllConfigs(ctx)

	rawDbs := makeDirectDBsProducer(cfg)
	gdb := makeGossipStore(rawDbs, cfg)
	defer gdb.Close()
	evms := gdb.EvmStore()

	start, reported := time.Now(), time.Now()

	var prevPoint idx.Block
	var prevIndex idx.Block
	checkBlocks := func(stateOK func(root common.Hash) (bool, error)) {
		var (
			lastIdx            = gdb.GetLatestBlockIndex()
			prevPointRootExist bool
		)
		gdb.ForEachBlock(func(index idx.Block, block *inter.Block) {
			prevIndex = index
			found, err := stateOK(common.Hash(block.Root))
			if found != prevPointRootExist {
				if index > 0 && found {
					log.Warn("EVM history is pruned", "fromBlock", prevPoint, "toBlock", index-1)
				}
				prevPointRootExist = found
				prevPoint = index
			}
			if index == lastIdx && !found {
				log.Crit("State trie for the latest block is not found", "block", index)
			}
			if !found {
				return
			}
			if err != nil {
				log.Crit("State trie error", "err", err, "block", index)
			}
			if time.Since(reported) >= statsReportLimit {
				log.Info("Checking presence of every node", "last", index, "pruned", !prevPointRootExist, "elapsed", common.PrettyDuration(time.Since(start)))
				reported = time.Now()
			}
		})
	}

	if err := evms.CheckEvm(checkBlocks, true); err != nil {
		return err
	}
	log.Info("EVM storage is verified", "last", prevIndex, "elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}
