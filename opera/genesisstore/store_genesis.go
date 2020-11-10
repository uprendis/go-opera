package genesisstore

import (
	"github.com/Fantom-foundation/go-opera/inter"
	"github.com/Fantom-foundation/go-opera/opera"
	"github.com/Fantom-foundation/go-opera/opera/genesis"
	"github.com/Fantom-foundation/go-opera/opera/genesis/gpos"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/Fantom-foundation/lachesis-base/kvdb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"math/big"
)

type (
	Metadata struct {
		Validators gpos.Validators
		FirstEpoch idx.Epoch
		Time       inter.Timestamp
		ExtraData  []byte
	}
	Accounts struct {
		Raw kvdb.Iteratee
	}
	Storage struct {
		Raw kvdb.Iteratee
	}
	Delegations struct {
		Raw kvdb.Iteratee
	}
	Blocks struct {
		Raw kvdb.Iteratee
	}

	Delegation struct {
		Stake   *big.Int
		Rewards *big.Int
	}
)

func (s *Store) EvmAccounts() genesis.Accounts {
	return &Accounts{s.table.EvmAccounts}
}

func (s *Store) SetEvmAccount(addr common.Address, acc genesis.Account) {
	s.rlp.Set(s.table.EvmAccounts, addr.Bytes(), &acc)
}

func (s *Store) EvmStorage() genesis.Storage {
	return &Storage{s.table.EvmStorage}
}

func (s *Store) SetEvmState(addr common.Address, key common.Hash, value common.Hash) {
	s.rlp.Set(s.table.EvmStorage, append(addr.Bytes(), key.Bytes()...), &value)
}

func (s *Store) Delegations() genesis.Delegations {
	return &Delegations{s.table.Delegations}
}

func (s *Store) SetDelegation(addr common.Address, toValidatorID idx.ValidatorID, delegation genesis.Delegation) {
	s.rlp.Set(s.table.Delegations, append(addr.Bytes(), toValidatorID.Bytes()...), &delegation)
}

func (s *Store) Blocks() genesis.Blocks {
	return &Blocks{s.table.Blocks}
}

func (s *Store) SetBlock(index idx.Block, block genesis.Block) {
	s.rlp.Set(s.table.Blocks, index.Bytes(), &block)
}

func (s *Store) GetMetadata() Metadata {
	metadata := s.rlp.Get(s.table.Metadata, []byte("m"), &Metadata{}).(*Metadata)
	return *metadata
}

func (s *Store) SetMetadata(metadata Metadata) {
	s.rlp.Set(s.table.Metadata, []byte("m"), &metadata)
}

func (s *Store) GetRules() opera.Rules {
	cfg := s.rlp.Get(s.table.Rules, []byte("c"), &opera.Rules{}).(*opera.Rules)
	return *cfg
}

func (s *Store) SetRules(cfg opera.Rules) {
	s.rlp.Set(s.table.Rules, []byte("c"), &cfg)
}

func (s *Store) GetGenesisState() opera.GenesisState {
	meatadata := s.GetMetadata()
	return opera.GenesisState{
		Accounts:    s.EvmAccounts(),
		Storage:     s.EvmStorage(),
		Delegations: s.Delegations(),
		Blocks:      s.Blocks(),
		Validators:  meatadata.Validators,
		FirstEpoch:  meatadata.FirstEpoch,
		Time:        meatadata.Time,
		ExtraData:   meatadata.ExtraData,
	}
}

func (s *Store) GetGenesis() opera.Genesis {
	return opera.Genesis{
		Rules: s.GetRules(),
		State: s.GetGenesisState(),
	}
}

func (s *Accounts) ForEach(fn func(common.Address, genesis.Account)) {
	it := s.Raw.NewIterator()
	defer it.Release()
	for it.Next() {
		addr := common.BytesToAddress(it.Key())
		acc := genesis.Account{}
		err := rlp.DecodeBytes(it.Value(), &acc)
		if err != nil {
			log.Crit("Genesis accounts error", "err", err)
		}
		fn(addr, acc)
	}
}

func (s *Storage) ForEach(fn func(common.Address, common.Hash, common.Hash)) {
	it := s.Raw.NewIterator()
	defer it.Release()
	for it.Next() {
		addr := common.BytesToAddress(it.Key()[:20])
		key := common.BytesToHash(it.Key()[20:])
		val := common.BytesToHash(it.Value())
		fn(addr, key, val)
	}
}

func (s *Delegations) ForEach(fn func(common.Address, idx.ValidatorID, genesis.Delegation)) {
	it := s.Raw.NewIterator()
	defer it.Release()
	for it.Next() {
		addr := common.BytesToAddress(it.Key()[:20])
		to := idx.BytesToValidatorID(it.Key()[20:])
		delegation := genesis.Delegation{}
		err := rlp.DecodeBytes(it.Value(), &delegation)
		if err != nil {
			log.Crit("Genesis delegations error", "err", err)
		}
		fn(addr, to, delegation)
	}
}

func (s *Blocks) ForEach(fn func(idx.Block, genesis.Block)) {
	it := s.Raw.NewIterator()
	defer it.Release()
	for it.Next() {
		index := idx.BytesToBlock(it.Key())
		block := genesis.Block{}
		err := rlp.DecodeBytes(it.Value(), &block)
		if err != nil {
			log.Crit("Genesis blocks error", "err", err)
		}
		fn(index, block)
	}
}
