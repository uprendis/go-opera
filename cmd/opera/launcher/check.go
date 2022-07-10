package launcher

import (
	"math/big"
	"path"
	"time"

	"github.com/Fantom-foundation/lachesis-base/common/bigendian"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/inter/pos"
	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"gopkg.in/urfave/cli.v1"

	"github.com/Fantom-foundation/go-opera/integration"
	"github.com/Fantom-foundation/go-opera/inter"
	futils "github.com/Fantom-foundation/go-opera/utils"
)

const (
	TxTurnNonces = 32
)

func txTurn(roundIndex int, sender common.Address, accountNonce uint64, validators *pos.Validators, epoch idx.Epoch) idx.ValidatorID {
	roundsHash := hash.Of(sender.Bytes(), bigendian.Uint64ToBytes(accountNonce/TxTurnNonces), epoch.Bytes())
	rounds := futils.WeightedPermutation(roundIndex+1, validators.SortedWeights(), roundsHash)
	return validators.GetID(idx.Validator(rounds[roundIndex]))
}

type stat struct {
	V [5]int
}

func checkEvm(ctx *cli.Context) error {
	if len(ctx.Args()) != 0 {
		utils.Fatalf("This command doesn't require an argument.")
	}

	cfg := makeAllConfigs(ctx)

	rawProducer := integration.DBProducer(path.Join(cfg.Node.DataDir, "chaindata"), cfg.cachescale)
	gdb, err := makeRawGossipStore(rawProducer, cfg)
	if err != nil {
		log.Crit("DB opening error", "datadir", cfg.Node.DataDir, "err", err)
	}
	defer gdb.Close()

	stats := map[idx.ValidatorID]stat{}
	vals := gdb.GetValidators()
	prev := idx.Epoch(0)
	gdb.ForEachEvent(130898, func(event *inter.EventPayload) bool {
		for _, tx := range event.Txs() {
			signer := types.NewLondonSigner(big.NewInt(0xfa))
			sender, _ := signer.Sender(tx)
			s := stats[event.Creator()]
			for ri := 0; ri <= 3; ri++ {
				if txTurn(ri, sender, tx.Nonce(), vals, event.Epoch()) == event.Creator() {
					s.V[ri]++
					goto end
				}
			}
			s.V[4]++
		end:
			stats[event.Creator()] = s
		}
		if event.Epoch() > prev {
			totalWeight := pos.Weight(0)
			totalTxs := 0
			for _, vid := range vals.SortedIDs() {
				s := stats[vid]
				println(event.Epoch(), vid, vals.Get(vid), s.V[0], s.V[1], s.V[2], s.V[3], s.V[4])
				totalWeight += vals.Get(vid)
				for _, v := range s.V {
					totalTxs += v
				}
			}
			println(totalTxs, totalWeight)
			println("----")
			prev = event.Epoch()
		}
		return true
	})
	return nil

	evms := gdb.EvmStore()

	start, reported := time.Now(), time.Now()

	var prevPoint idx.Block
	checkBlocks := func(stateOK func(root common.Hash) (bool, error)) {
		var (
			lastIdx            = gdb.GetLatestBlockIndex()
			prevPointRootExist bool
		)
		gdb.ForEachBlock(func(index idx.Block, block *inter.Block) {
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

	err = evms.CheckEvm(checkBlocks)
	if err != nil {
		return err
	}
	log.Info("EVM storage is verified", "last", prevPoint, "elapsed", common.PrettyDuration(time.Since(start)))
	return nil
}
