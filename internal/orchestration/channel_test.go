package orchestration

import (
	"fmt"
	"testing"
	"time"

	"zheng-harness/internal/domain"
)

func TestChannelSendReceive(t *testing.T) {
	t.Parallel()

	resultCh := NewResultChannel(1)
	requestCh := NewRequestChannel(1)
	request := TaskRequest{
		Subtask: Subtask{ID: "subtask-1", Description: "dispatch", Status: SubtaskStatusPending},
		Context: "worker-context",
		ResultChan: func() chan<- TaskResult {
			return resultCh.ch
		}(),
	}
	wantResult := TaskResult{
		SubtaskID:          "subtask-1",
		Output:             "done",
		VerificationStatus: domain.VerificationStatusPassed,
		Duration:           25 * time.Millisecond,
	}

	go func() {
		gotRequest, ok := requestCh.Receive()
		if !ok {
			t.Error("Receive() closed unexpectedly")
			return
		}
		if gotRequest.Subtask.ID != request.Subtask.ID {
			t.Errorf("request Subtask.ID = %q, want %q", gotRequest.Subtask.ID, request.Subtask.ID)
		}
		if gotRequest.Context != request.Context {
			t.Errorf("request Context = %q, want %q", gotRequest.Context, request.Context)
		}
		if gotRequest.ResultChan == nil {
			t.Error("request ResultChan = nil, want non-nil")
			return
		}
		gotRequest.ResultChan <- wantResult
	}()

	if sent := requestCh.Send(request); !sent {
		t.Fatal("Send() = false, want true")
	}
	gotResult, ok := resultCh.Receive()
	if !ok {
		t.Fatal("result Receive() closed unexpectedly")
	}
	if gotResult.SubtaskID != wantResult.SubtaskID {
		t.Fatalf("result SubtaskID = %q, want %q", gotResult.SubtaskID, wantResult.SubtaskID)
	}
	if gotResult.Output != wantResult.Output {
		t.Fatalf("result Output = %q, want %q", gotResult.Output, wantResult.Output)
	}
	if gotResult.VerificationStatus != wantResult.VerificationStatus {
		t.Fatalf("result VerificationStatus = %q, want %q", gotResult.VerificationStatus, wantResult.VerificationStatus)
	}
	if gotResult.Duration != wantResult.Duration {
		t.Fatalf("result Duration = %s, want %s", gotResult.Duration, wantResult.Duration)
	}

	requestCh.Close()
	resultCh.Close()
}

func TestChannelBufferFull(t *testing.T) {
	t.Parallel()

	requestCh := NewRequestChannel(2)
	if cap(requestCh.ch) != 2 {
		t.Fatalf("request channel capacity = %d, want 2", cap(requestCh.ch))
	}
	if sent := requestCh.Send(TaskRequest{Subtask: Subtask{ID: "a", Description: "a", Status: SubtaskStatusPending}}); !sent {
		t.Fatal("first Send() = false, want true")
	}
	if sent := requestCh.Send(TaskRequest{Subtask: Subtask{ID: "b", Description: "b", Status: SubtaskStatusPending}}); !sent {
		t.Fatal("second Send() = false, want true")
	}
	select {
	case requestCh.ch <- TaskRequest{Subtask: Subtask{ID: "c", Description: "c", Status: SubtaskStatusPending}}:
		t.Fatal("send to full request channel unexpectedly succeeded")
	default:
	}

	resultCh := NewResultChannel(2)
	if cap(resultCh.ch) != 2 {
		t.Fatalf("result channel capacity = %d, want 2", cap(resultCh.ch))
	}
	if sent := resultCh.Send(TaskResult{SubtaskID: "a", VerificationStatus: domain.VerificationStatusPassed}); !sent {
		t.Fatal("first result Send() = false, want true")
	}
	if sent := resultCh.Send(TaskResult{SubtaskID: "b", VerificationStatus: domain.VerificationStatusFailed}); !sent {
		t.Fatal("second result Send() = false, want true")
	}
	select {
	case resultCh.ch <- TaskResult{SubtaskID: "c"}:
		t.Fatal("send to full result channel unexpectedly succeeded")
	default:
	}

	requestCh.Close()
	resultCh.Close()
}

func TestChannelCloseGraceful(t *testing.T) {
	t.Parallel()

	requestCh := NewRequestChannel(0)
	if cap(requestCh.ch) != defaultRequestChannelBuffer {
		t.Fatalf("default request capacity = %d, want %d", cap(requestCh.ch), defaultRequestChannelBuffer)
	}
	requestCh.Close()
	requestCh.Close()
	if sent := requestCh.Send(TaskRequest{Subtask: Subtask{ID: "closed", Description: "closed", Status: SubtaskStatusPending}}); sent {
		t.Fatal("Send() after close = true, want false")
	}
	if got, ok := requestCh.Receive(); ok {
		t.Fatalf("Receive() ok = true after close, got %#v", got)
	}

	resultCh := NewResultChannel(0)
	if cap(resultCh.ch) != defaultResultChannelBuffer {
		t.Fatalf("default result capacity = %d, want %d", cap(resultCh.ch), defaultResultChannelBuffer)
	}
	resultCh.Close()
	resultCh.Close()
	result := TaskResult{SubtaskID: "closed", Error: fmt.Errorf("closed"), VerificationStatus: domain.VerificationStatusFailed}
	if sent := resultCh.Send(result); sent {
		t.Fatal("result Send() after close = true, want false")
	}
	if got, ok := resultCh.Receive(); ok {
		t.Fatalf("result Receive() ok = true after close, got %#v", got)
	}
}
