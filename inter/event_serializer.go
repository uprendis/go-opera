package inter

import (
	"errors"
	"io"

	"github.com/Fantom-foundation/go-lachesis/utils/cser"
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/dag"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/ethereum/go-ethereum/rlp"
)

var (
	ErrSerMalformedEvent = errors.New("serialization of malformed event")
)

func (e *Event) MarshalCSER(w *cser.Writer) error {
	// base fields
	w.U32(uint32(e.Epoch()))
	w.U32(uint32(e.Lamport()))
	w.U32(uint32(e.Creator()))
	w.U32(uint32(e.Seq()))
	w.U32(uint32(e.Frame()))
	w.Bool(e.IsRoot())
	w.U64(e.creationTime)
	medianTimeDiff := int64(e.creationTime) - int64(e.medianTime)
	w.I64(medianTimeDiff)
	// gas power
	w.U64(e.gasPowerUsed)
	w.U64(e.gasPowerLeft.Gas[0])
	w.U64(e.gasPowerLeft.Gas[1])
	// parents
	w.U32(uint32(len(e.Parents())))
	for _, p := range e.Parents() {
		if e.Lamport() < p.Lamport() {
			return ErrSerMalformedEvent
		}
		// lamport difference
		w.U32(uint32(e.Lamport() - p.Lamport()))
		// without epoch and lamport
		w.FixedBytes(p.Bytes()[8:])
	}
	// prev epoch hash
	w.Bool(e.prevEpochHash != nil)
	if e.prevEpochHash != nil {
		w.FixedBytes(e.prevEpochHash.Bytes())
	}
	// tx hash
	w.Bool(!e.NoTxs())
	if !e.NoTxs() {
		w.FixedBytes(e.txHash.Bytes())
	}
	// extra
	w.SliceBytes(e.Extra())
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler interface.
func (e *Event) MarshalBinary() ([]byte, error) {
	return cser.MarshalBinaryAdapter(e.MarshalCSER)
}

func eventUnmarshalCSER(r *cser.Reader, e *MutableEventPayload) error {
	// base fields
	epoch, err := r.U32()
	if err != nil {
		return err
	}
	lamport, err := r.U32()
	if err != nil {
		return err
	}
	creator, err := r.U32()
	if err != nil {
		return err
	}
	seq, err := r.U32()
	if err != nil {
		return err
	}
	frame, err := r.U32()
	if err != nil {
		return err
	}
	isRoot, err := r.Bool()
	if err != nil {
		return err
	}
	creationTime, err := r.U64()
	if err != nil {
		return err
	}
	medianTimeDiff, err := r.I64()
	if err != nil {
		return err
	}
	// gas power
	gasPowerUsed, err := r.U64()
	if err != nil {
		return err
	}
	gasPowerLeft0, err := r.U64()
	if err != nil {
		return err
	}
	gasPowerLeft1, err := r.U64()
	if err != nil {
		return err
	}
	// parents
	parentsNum, err := r.U32()
	if err != nil {
		return err
	}
	parents := make(hash.Events, 0, parentsNum)
	for i := uint32(0); i < parentsNum; i++ {
		// lamport difference
		lamportDiff, err := r.U32()
		if err != nil {
			return err
		}
		// hash
		h := [24]byte{}
		err = r.FixedBytes(h[:])
		if err != nil {
			return err
		}
		eID := dag.MutableBaseEvent{}
		eID.SetEpoch(idx.Epoch(epoch))
		eID.SetLamport(idx.Lamport(lamport - lamportDiff))
		eID.SetID(h)
		parents.Add(eID.ID())
	}
	// prev epoch hash
	var prevEpochHash *hash.Hash
	prevEpochHashExists, err := r.Bool()
	if err != nil {
		return err
	}
	if prevEpochHashExists {
		prevEpochHash_ := hash.Hash{}
		err = r.FixedBytes(prevEpochHash_[:])
		if err != nil {
			return err
		}
		prevEpochHash = &prevEpochHash_
	}
	// tx hash
	txHash := EmptyTxHash
	txHashExists, err := r.Bool()
	if err != nil {
		return err
	}
	if txHashExists {
		err = r.FixedBytes(txHash[:])
		if err != nil {
			return err
		}
		if txHash == EmptyTxHash {
			return cser.ErrNonCanonicalEncoding
		}
	}
	// extra
	extra, err := r.SliceBytes()
	if err != nil {
		return err
	}

	e.SetEpoch(idx.Epoch(epoch))
	e.SetLamport(idx.Lamport(lamport))
	e.SetCreator(idx.ValidatorID(creator))
	e.SetSeq(idx.Event(seq))
	e.SetFrame(idx.Frame(frame))
	e.SetIsRoot(isRoot)
	e.creationTime = creationTime
	e.medianTime = uint64(int64(creationTime) - medianTimeDiff)
	e.SetGasPowerUsed(gasPowerUsed)
	e.SetGasPowerLeft(GasPowerLeft{[2]uint64{gasPowerLeft0, gasPowerLeft1}})
	e.SetParents(parents)
	e.SetPrevEpochHash(prevEpochHash)
	e.SetTxHash(txHash)
	e.SetExtra(extra)
	return nil
}

func (e *EventPayload) MarshalCSER(w *cser.Writer) error {
	if e.NoTxs() != (e.txs.Len() == 0) {
		return ErrSerMalformedEvent
	}
	err := e.Event.MarshalCSER(w)
	if err != nil {
		return err
	}
	w.FixedBytes(e.sig.Bytes())
	if !e.NoTxs() {
		// txs size
		w.U64fromZero(uint64(e.txs.Len()))
		// txs
		for _, tx := range e.txs {
			err := TransactionMarshalCSER(w, tx)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (e *MutableEventPayload) UnmarshalCSER(r *cser.Reader) error {
	err := eventUnmarshalCSER(r, e)
	if err != nil {
		return err
	}
	err = r.FixedBytes(e.sig[:])
	if err != nil {
		return err
	}
	txs := types.Transactions{}
	if !e.NoTxs() {
		// txs size
		size, err := r.U64fromZero()
		if err != nil {
			return err
		}
		for i := uint64(0); i < size; i++ {
			tx, err := TransactionUnmarshalCSER(r)
			if err != nil {
				return err
			}
			txs = append(txs, tx)
		}
	}
	e.SetTxs(txs)
	return nil
}

// MarshalBinary implements encoding.BinaryMarshaler interface.
func (e *EventPayload) MarshalBinary() ([]byte, error) {
	return cser.MarshalBinaryAdapter(e.MarshalCSER)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface.
func (e *MutableEventPayload) UnmarshalBinary(raw []byte) (err error) {
	return cser.UnmmrshalBinaryAdapter(raw, e.UnmarshalCSER)
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface.
func (e *EventPayload) UnmarshalBinary(raw []byte) (err error) {
	mutE := MutableEventPayload{}
	err = mutE.UnmarshalBinary(raw)
	if err != nil {
		return err
	}
	eventSer, _ := mutE.immutable().Event.MarshalBinary()
	h := eventHash(eventSer)
	*e = *mutE.build(h, len(raw))
	return nil
}

// EncodeRLP implements rlp.Encoder interface.
func (e *EventPayload) EncodeRLP(w io.Writer) error {
	bytes, err := e.MarshalBinary()
	if err != nil {
		return err
	}

	err = rlp.Encode(w, &bytes)

	return err
}

// DecodeRLP implements rlp.Decoder interface.
func (e *EventPayload) DecodeRLP(src *rlp.Stream) error {
	bytes, err := src.Bytes()
	if err != nil {
		return err
	}

	return e.UnmarshalBinary(bytes)
}

// DecodeRLP implements rlp.Decoder interface.
func (e *MutableEventPayload) DecodeRLP(src *rlp.Stream) error {
	bytes, err := src.Bytes()
	if err != nil {
		return err
	}

	return e.UnmarshalBinary(bytes)
}
