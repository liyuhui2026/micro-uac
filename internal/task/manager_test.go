package task

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/liyuhui/micro-uac/internal/domain"
)

type fakeRunner struct {
	block chan struct{}
	res   domain.CallResult
	err   error
}

func (f *fakeRunner) Dial(ctx context.Context, req domain.CallRequest) (domain.CallResult, error) {
	if f.block != nil {
		select {
		case <-f.block:
		case <-ctx.Done():
			return domain.CallResult{}, ctx.Err()
		}
	}
	return f.res, f.err
}

func TestManagerSingleRunningTask(t *testing.T) {
	runner := &fakeRunner{block: make(chan struct{})}
	manager := NewManager(runner)

	if _, err := manager.Create(context.Background(), domain.CallRequest{}); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	if _, err := manager.Create(context.Background(), domain.CallRequest{}); !errors.Is(err, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", err)
	}

	close(runner.block)
	time.Sleep(20 * time.Millisecond)
}

func TestManagerCreateIgnoresCanceledRequestContext(t *testing.T) {
	runner := &fakeRunner{
		res: domain.CallResult{State: domain.CallStateCompleted},
	}
	manager := NewManager(runner)

	ctx, cancel := context.WithCancel(context.Background())
	if _, err := manager.Create(ctx, domain.CallRequest{}); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	cancel()

	time.Sleep(20 * time.Millisecond)

	manager.mu.RLock()
	defer manager.mu.RUnlock()
	if manager.running {
		t.Fatal("expected manager not running")
	}
	for _, res := range manager.tasks {
		if res.State == domain.CallStateFailed && res.Reason == context.Canceled.Error() {
			t.Fatal("expected task not to be canceled by request context")
		}
	}
}

func TestManagerAllowsNextCallAfterRunnerReturnsErrorResult(t *testing.T) {
	runner := &fakeRunner{
		res: domain.CallResult{State: domain.CallStateCompleted},
		err: errors.New("send bye: Timer_B timed out. transaction timeout"),
	}
	manager := NewManager(runner)

	if _, err := manager.Create(context.Background(), domain.CallRequest{}); err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	time.Sleep(20 * time.Millisecond)

	if _, err := manager.Create(context.Background(), domain.CallRequest{}); err != nil {
		t.Fatalf("second create failed: %v", err)
	}
}
