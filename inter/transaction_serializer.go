package inter

import (
	"math/big"

	"github.com/Fantom-foundation/go-lachesis/utils/cser"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func encodeSig(v, r, s *big.Int) (sig [65]byte) {
	copy(sig[0:], cser.PaddedBytes(r.Bytes(), 32)[:32])
	copy(sig[32:], cser.PaddedBytes(s.Bytes(), 32)[:32])
	copy(sig[64:], cser.PaddedBytes(v.Bytes(), 1)[:1])
	return sig
}

func decodeSig(sig [65]byte) (v, r, s *big.Int) {
	r = new(big.Int).SetBytes(sig[:32])
	s = new(big.Int).SetBytes(sig[32:64])
	v = new(big.Int).SetBytes([]byte{sig[64]})
	return
}

func TransactionMarshalCSER(w *cser.Writer, tx *types.Transaction) error {
	w.U64(tx.Nonce())
	w.U64(tx.Gas())
	w.BigInt(tx.GasPrice())
	w.BigInt(tx.Value())
	w.Bool(tx.To() != nil)
	if tx.To() != nil {
		w.FixedBytes(tx.To().Bytes())
	}
	w.SliceBytes(tx.Data())
	sig := encodeSig(tx.RawSignatureValues())
	w.FixedBytes(sig[:])
	return nil
}

func TransactionUnmarshalCSER(r *cser.Reader) (*types.Transaction, error) {
	nonce, err := r.U64()
	if err != nil {
		return nil, err
	}
	gasLimit, err := r.U64()
	if err != nil {
		return nil, err
	}
	gasPrice, err := r.BigInt()
	if err != nil {
		return nil, err
	}
	amount, err := r.BigInt()
	if err != nil {
		return nil, err
	}
	toExists, err := r.Bool()
	if err != nil {
		return nil, err
	}
	var to *common.Address
	if toExists {
		var _to common.Address
		err = r.FixedBytes(_to[:])
		if err != nil {
			return nil, err
		}
		to = &_to
	}
	data, err := r.SliceBytes()
	if err != nil {
		return nil, err
	}
	// sig
	var sig [65]byte
	err = r.FixedBytes(sig[:])
	if err != nil {
		return nil, err
	}

	v, _r, s := decodeSig(sig)
	return types.NewRawTransaction(nonce, to, amount, gasLimit, gasPrice, data, v, _r, s), nil
}
