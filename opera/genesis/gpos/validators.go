package gpos

import (
	"github.com/ethereum/go-ethereum/common"

	"github.com/Fantom-foundation/go-opera/inter/validator"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
)

type (
	// Validator is a helper structure to define genesis validators
	Validator struct {
		ID        idx.ValidatorID
		Address   common.Address
		PubKey    validator.PubKey
	}

	Validators []Validator
)

// Map converts Validators to map
func (gv Validators) Map() map[idx.ValidatorID]Validator {
	validators := map[idx.ValidatorID]Validator{}
	for _, val := range gv {
		validators[val.ID] = val
	}
	return validators
}

// PubKeys returns not sorted genesis pub keys
func (gv Validators) PubKeys() []validator.PubKey {
	res := make([]validator.PubKey, 0, len(gv))
	for _, v := range gv {
		res = append(res, v.PubKey)
	}
	return res
}

// Addresses returns not sorted genesis addresses
func (gv Validators) Addresses() []common.Address {
	res := make([]common.Address, 0, len(gv))
	for _, v := range gv {
		res = append(res, v.Address)
	}
	return res
}
