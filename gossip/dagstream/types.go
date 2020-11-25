package essteam

import (
	"github.com/Fantom-foundation/lachesis-base/hash"
	"github.com/Fantom-foundation/lachesis-base/inter/idx"
)

type Request struct {
	Session Session
	Limit   Metric
	Type    RequestType
}

type Response struct {
	SessionID uint32
	Done      bool
	IDs       hash.Events
	Events    []interface{}
}

type Metric struct {
	Num  idx.Event
	Size uint64
}

type Session struct {
	ID    uint32
	Start []byte
	Stop  []byte
}

type RequestType uint8

const (
	RequestIDs    RequestType = 0
	RequestEvents RequestType = 2
)
