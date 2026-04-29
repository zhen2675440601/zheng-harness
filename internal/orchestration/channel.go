package orchestration

import (
	"sync"
	"time"

	"zheng-harness/internal/domain"
)

const (
	defaultRequestChannelBuffer = defaultMaxWorkers
	defaultResultChannelBuffer  = 1024
)

// TaskRequest carries one subtask dispatch from orchestrator to a worker.
type TaskRequest struct {
	Subtask    Subtask
	Context    string
	ResultChan chan<- TaskResult
}

// TaskResult carries one worker completion back to the orchestrator.
type TaskResult struct {
	SubtaskID          string
	Output             string
	Error              error
	VerificationStatus domain.VerificationStatus
	Duration           time.Duration
}

// RequestChannel provides safe typed request transport.
type RequestChannel struct {
	ch   chan TaskRequest
	once sync.Once
}

// ResultChannel provides safe typed result transport.
type ResultChannel struct {
	ch   chan TaskResult
	once sync.Once
}

// NewRequestChannel constructs a buffered request channel.
func NewRequestChannel(buffer int) *RequestChannel {
	return &RequestChannel{ch: make(chan TaskRequest, normalizeChannelBuffer(buffer, defaultRequestChannelBuffer))}
}

// NewResultChannel constructs a buffered result channel.
func NewResultChannel(buffer int) *ResultChannel {
	return &ResultChannel{ch: make(chan TaskResult, normalizeChannelBuffer(buffer, defaultResultChannelBuffer))}
}

// Channel exposes the underlying typed request channel.
func (c *RequestChannel) Channel() <-chan TaskRequest {
	if c == nil {
		return nil
	}
	return c.ch
}

// Channel exposes the underlying typed result channel.
func (c *ResultChannel) Channel() <-chan TaskResult {
	if c == nil {
		return nil
	}
	return c.ch
}

// Send publishes one request and reports whether it was accepted.
func (c *RequestChannel) Send(request TaskRequest) (sent bool) {
	if c == nil || c.ch == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	c.ch <- request
	return true
}

// Send publishes one result and reports whether it was accepted.
func (c *ResultChannel) Send(result TaskResult) (sent bool) {
	if c == nil || c.ch == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			sent = false
		}
	}()
	c.ch <- result
	return true
}

// Receive reads one request and reports whether the channel remains open.
func (c *RequestChannel) Receive() (TaskRequest, bool) {
	if c == nil || c.ch == nil {
		return TaskRequest{}, false
	}
	request, ok := <-c.ch
	return request, ok
}

// Receive reads one result and reports whether the channel remains open.
func (c *ResultChannel) Receive() (TaskResult, bool) {
	if c == nil || c.ch == nil {
		return TaskResult{}, false
	}
	result, ok := <-c.ch
	return result, ok
}

// Close idempotently closes the request channel.
func (c *RequestChannel) Close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		if c.ch != nil {
			close(c.ch)
		}
	})
}

// Close idempotently closes the result channel.
func (c *ResultChannel) Close() {
	if c == nil {
		return
	}
	c.once.Do(func() {
		if c.ch != nil {
			close(c.ch)
		}
	})
}

func normalizeChannelBuffer(buffer, fallback int) int {
	if buffer > 0 {
		return buffer
	}
	if fallback > 0 {
		return fallback
	}
	return 1
}
