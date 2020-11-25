package dagsemaphore

import (
	"sync"
	"time"

	"github.com/Fantom-foundation/lachesis-base/inter/idx"
)

type Metric struct {
	Num  idx.Event
	Size uint64
}

type EventsSemaphore struct {
	received      Metric
	processing    Metric
	maxProcessing Metric

	terminated bool

	mu   sync.Mutex
	cond *sync.Cond

	warning func(received Metric, processing Metric, releasing Metric)
}

func New(maxProcessing Metric, warning func(received Metric, processing Metric, releasing Metric)) EventsSemaphore {
	s := EventsSemaphore{
		maxProcessing: maxProcessing,
		warning:       warning,
	}
	s.cond = sync.NewCond(&s.mu)
	return s
}

func (s *EventsSemaphore) Received(events Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.received.Num += events.Num
	s.received.Size += events.Size
}

func (s *EventsSemaphore) Acquire(events Metric, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	s.mu.Lock()
	defer s.mu.Unlock()
	for !s.tryAcquire(events) {
		if s.terminated || time.Now().After(deadline) {
			return false
		}
		s.cond.Wait()
	}
	return true
}

func (s *EventsSemaphore) TryAcquire(events Metric) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.tryAcquire(events)
}

func (s *EventsSemaphore) tryAcquire(metric Metric) bool {
	tmp := s.processing
	tmp.Num += metric.Num
	tmp.Size += metric.Size
	if tmp.Num > s.maxProcessing.Num || tmp.Size > s.maxProcessing.Size {
		return false
	}
	s.processing = tmp
	return true
}

func (s *EventsSemaphore) Release(events Metric) {
	s.mu.Lock()
	defer s.mu.Unlock()
	receivedUnferflow := s.received.Num < events.Num || s.received.Size < events.Size
	processingUnderflow := s.processing.Num < events.Num || s.processing.Size < events.Size
	if receivedUnferflow || processingUnderflow {
		s.warning(s.processing, s.processing, events)
	}
	if receivedUnferflow {
		s.received = Metric{}
	} else {
		s.received.Num -= events.Num
		s.received.Size -= events.Size
	}
	if processingUnderflow {
		s.processing = Metric{}
	} else {
		s.processing.Num -= events.Num
		s.processing.Size -= events.Size
	}
	s.cond.Broadcast()
}

func (s *EventsSemaphore) Terminate() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.maxProcessing = Metric{}
	s.terminated = true
	s.cond.Broadcast()
}
