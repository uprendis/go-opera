package cser

import (
	"bytes"

	"github.com/Fantom-foundation/go-lachesis/utils/bits"
)

func MarshalBinaryAdapter(marshalCser func(writer *Writer) error) ([]byte, error) {
	bodyBits := &bits.Array{Bytes: make([]byte, 0, 32)}
	bodyBytes := bytes.NewBuffer(make([]byte, 0, 200))
	bodyWriter := &Writer{
		BitsW:  bits.NewWriter(bodyBits),
		BytesW: bodyBytes,
	}
	err := marshalCser(bodyWriter)
	if err != nil {
		return nil, err
	}
	if len(bodyBits.Bytes) > 128 {
		return nil, ErrTooBigEncofing
	}

	bodyBytes.Write(bodyBits.Bytes)
	bodyBytes.WriteByte(byte(len(bodyBits.Bytes)))

	return bodyBytes.Bytes(), nil
}

// UnmarshalBinary implements encoding.BinaryUnmarshaler interface.
func BinaryToCSER(raw []byte) (bodyBits *bits.Array, bodyBytes []byte, err error) {
	// read bitsArray size
	bitsSize := uint64(raw[len(raw)-1])
	raw = raw[:len(raw)-1]

	if uint64(len(raw)) < bitsSize {
		return nil, nil, ErrMalformedEncoding
	}

	bodyBits = &bits.Array{Bytes: raw[uint64(len(raw))-bitsSize:]}
	bodyBytes = raw[:uint64(len(raw))-bitsSize]
	return bodyBits, bodyBytes, nil
}

func UnmmrshalBinaryAdapter(raw []byte, unmarshalCser func(reader *Reader) error) error {
	bodyBits, bodyBytes_, err := BinaryToCSER(raw)
	if err != nil {
		return err
	}
	bodyBytes := bytes.NewBuffer(bodyBytes_)

	bodyReader := &Reader{
		BitsR:  bits.NewReader(bodyBits),
		BytesR: bodyBytes,
	}
	err = unmarshalCser(bodyReader)
	if err != nil {
		return err
	}

	// check that everything is read
	if bodyReader.BitsR.NonReadBytes() > 1 {
		return ErrNonCanonicalEncoding
	}
	tail, err := bodyReader.BitsR.Read(bodyReader.BitsR.NonReadBits())
	if err != nil {
		return err
	}
	if tail != 0 {
		return ErrNonCanonicalEncoding
	}
	if n, _ := bodyReader.BytesR.Read([]byte{0}); n != 0 {
		return ErrNonCanonicalEncoding
	}

	return nil
}
