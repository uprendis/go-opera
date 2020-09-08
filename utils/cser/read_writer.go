package cser

import (
	"errors"
	"io"
	"math/big"

	"github.com/Fantom-foundation/go-lachesis/utils/bits"
)

var (
	ErrNonCanonicalEncoding = errors.New("non canonical encoding")
	ErrMalformedEncoding    = errors.New("malformed encoding")
	ErrTooBigEncofing       = errors.New("too big encoding")
)

type Writer struct {
	BitsW  *bits.Writer
	BytesW io.Writer
}

type Reader struct {
	BitsR  *bits.Reader
	BytesR io.Reader
}

func writeUint64Compact(bytesW io.Writer, v uint64, minSize int) (size int) {
	for size < minSize || v != 0 {
		_, _ = bytesW.Write([]byte{byte(v)})
		size++
		v = v >> 8
	}
	return
}

func readUintCompact(bytesR io.Reader, size int) (uint64, error) {
	var (
		v    uint64
		last byte
	)
	buf := make([]byte, size)
	n, err := bytesR.Read(buf)
	if err != nil {
		return 0, err
	}
	if n != len(buf) {
		return 0, io.ErrUnexpectedEOF
	}
	for i, b := range buf {
		v |= uint64(b) << uint(8*i)
		last = b
	}

	if size > 1 && last == 0 {
		return 0, ErrNonCanonicalEncoding
	}

	return v, nil
}

func (r *Reader) U8() (uint8, error) {
	buf := []byte{0}
	n, err := r.BytesR.Read(buf)
	if err != nil {
		return 0, err
	}
	if n != len(buf) {
		return 0, io.ErrUnexpectedEOF
	}
	return buf[0], nil
}

func (w *Writer) U8(v uint8) {
	buf := []byte{0}
	buf[0] = v
	_, _ = w.BytesW.Write(buf)
}

func (r *Reader) readU64(minSize int, bitsForSize int) (uint64, error) {
	size, err := r.BitsR.Read(bitsForSize)
	if err != nil {
		return 0, err
	}
	size += uint(minSize)
	return readUintCompact(r.BytesR, int(size))
}

func (w *Writer) writeU64(minSize int, bitsForSize int, v uint64) {
	size := writeUint64Compact(w.BytesW, v, minSize)
	w.BitsW.Write(bitsForSize, uint(size-minSize))
}

func (r *Reader) U16() (uint16, error) {
	v64, err := r.readU64(1, 1)
	if err != nil {
		return 0, err
	}
	return uint16(v64), nil
}

func (w *Writer) U16(v uint16) {
	w.writeU64(1, 1, uint64(v))
}

func (r *Reader) U32() (uint32, error) {
	v64, err := r.readU64(1, 2)
	if err != nil {
		return 0, err
	}
	return uint32(v64), nil
}

func (w *Writer) U32(v uint32) {
	w.writeU64(1, 2, uint64(v))
}

func (r *Reader) U64() (uint64, error) {
	return r.readU64(1, 3)
}

func (w *Writer) U64(v uint64) {
	w.writeU64(1, 3, v)
}

func (r *Reader) I64() (int64, error) {
	neg, err := r.Bool()
	if err != nil {
		return 0, err
	}
	abs, err := r.U64()
	if err != nil {
		return 0, err
	}
	if neg && abs == 0 {
		return 0, ErrNonCanonicalEncoding
	}
	if neg {
		return -int64(abs), nil
	}
	return int64(abs), nil
}

func (w *Writer) I64(v int64) {
	w.Bool(v < 0)
	if v < 0 {
		w.U64(uint64(-v))
	} else {
		w.U64(uint64(v))
	}
}

func (r *Reader) U64fromZero() (uint64, error) {
	return r.readU64(0, 3)
}

func (w *Writer) U64fromZero(v uint64) {
	w.writeU64(0, 3, v)
}

func (r *Reader) Bool() (bool, error) {
	u8, err := r.BitsR.Read(1)
	if err != nil {
		return false, err
	}
	return u8 != 0, nil
}

func (w *Writer) Bool(v bool) {
	u8 := uint(0)
	if v {
		u8 = 1
	}
	w.BitsW.Write(1, u8)
}

func (r *Reader) FixedBytes(v []byte) error {
	n, err := r.BytesR.Read(v)
	if err != nil {
		return err
	}
	if n != len(v) {
		return io.ErrUnexpectedEOF
	}
	return nil
}

func (w *Writer) FixedBytes(v []byte) {
	_, _ = w.BytesW.Write(v)
}

func (r *Reader) SliceBytes() ([]byte, error) {
	// read slice size
	size, err := r.U64fromZero()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, size)
	// read slice content
	err = r.FixedBytes(buf)
	if err != nil {
		return buf, err
	}
	return buf, nil
}

func (w *Writer) SliceBytes(v []byte) {
	// write slice size
	w.U64fromZero(uint64(len(v)))
	// write slice content
	w.FixedBytes(v)
}

// PaddedBytes returns a slice with length of the slice is at least n bytes.
func PaddedBytes(b []byte, n int) []byte {
	if len(b) >= n {
		return b
	}
	padding := make([]byte, n-len(b))
	return append(padding, b...)
}

func (w *Writer) BigInt(v *big.Int) {
	/*if len(sizeOptions) >= 1 << sizeOptionsBits {
		panic("inconsistent sizeOptionsBits")
	}
	// use empty bytes for zero number
	bigBytes := []byte{}
	if v.Sign() != 0 {
		bigBytes = v.Bytes()
	}
	ok, option := w.findSizeOption(uint32(len(bigBytes)), sizeOptions)
	w.Bool(ok)
	if ok {
		// option number
		w.BitsW.Write(sizeOptionsBits, uint(option))
		// padded bigint
		w.FixedBytes(PaddedBytes(bigBytes, int(sizeOptions[option])))
	} else {
		// serialize as an ordinary slice
		w.SliceBytes(bigBytes)
	}*/
	// serialize as an ordinary slice
	bigBytes := []byte{}
	if v.Sign() != 0 {
		bigBytes = v.Bytes()
	}
	w.SliceBytes(bigBytes)
}

func (r *Reader) BigInt() (*big.Int, error) {
	/*if len(sizeOptions) >= 1 << sizeOptionsBits {
		panic("inconsistent sizeOptionsBits")
	}
	optioned, err := r.Bool()
	if err != nil {
		return nil, err
	}
	var buf []byte
	if optioned {
		// option number
		option, err := r.BitsR.Read(sizeOptionsBits)
		if err != nil {
			return nil, err
		}
		// padded bigint
		b := make([]byte, int(sizeOptions[option]))
		err = r.FixedBytes(b)
		if err != nil {
			return nil, err
		}
	} else {
		// deserialize as an ordinary slice
		buf, err = r.SliceBytes()
		if err != nil {
			return nil, err
		}
		maxOption := sizeOptions[len(sizeOptions) - 1]
		if uint32(len(buf)) <= maxOption {
			return nil, ErrNonCanonicalEncoding
		}
	}
	if len(buf) == 0 {
		return new(big.Int), nil
	}
	return new(big.Int).SetBytes(buf), nil*/
	// deserialize as an ordinary slice
	buf, err := r.SliceBytes()
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return new(big.Int), nil
	}
	return new(big.Int).SetBytes(buf), nil
}
