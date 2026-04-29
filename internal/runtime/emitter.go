package runtime

import (
	"errors"
	"sync"

	"zheng-harness/internal/domain"
)

var errEventChannelClosed = errors.New("runtime event channel closed")

// EventChannel provides non-blocking runtime event delivery for streaming consumers.
type EventChannel struct {
	mu     sync.RWMutex
	ch     chan domain.StreamingEvent
	closed bool
}

// NewEventChannel creates a new buffered event channel.
func NewEventChannel(buffer int) *EventChannel {
	if buffer < 0 {
		buffer = 0
	}
	return &EventChannel{ch: make(chan domain.StreamingEvent, buffer)}
}

// Emit attempts a non-blocking send and drops the event when the buffer is full.
func (ec *EventChannel) Emit(event domain.StreamingEvent) error {
	if ec == nil {
		return nil
	}

	ec.mu.RLock()
	defer ec.mu.RUnlock()
	if ec.closed {
		return errEventChannelClosed
	}

	select {
	case ec.ch <- event:
	default:
	}
	return nil
}

// Events exposes a read-only event stream to consumers.
func (ec *EventChannel) Events() <-chan domain.StreamingEvent {
	if ec == nil {
		return nil
	}
	return ec.ch
}

// Close marks the channel as closed and closes the underlying stream exactly once.
func (ec *EventChannel) Close() {
	if ec == nil {
		return
	}

	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.closed {
		return
	}
	ec.closed = true
	close(ec.ch)
}
