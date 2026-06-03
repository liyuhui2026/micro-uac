package task

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/liyuhui/micro-uac/internal/domain"
)

var ErrBusy = errors.New("another call is already running")

type Runner interface {
	Dial(ctx context.Context, req domain.CallRequest) (domain.CallResult, error)
}

type Manager struct {
	mu      sync.RWMutex
	runner  Runner
	running bool
	tasks   map[string]domain.CallResult
}

func NewManager(runner Runner) *Manager {
	return &Manager{
		runner: runner,
		tasks:  make(map[string]domain.CallResult),
	}
}

func (m *Manager) Create(ctx context.Context, req domain.CallRequest) (domain.CallResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return domain.CallResult{}, ErrBusy
	}

	callID := uuid.NewString()
	result := domain.CallResult{
		CallID:    callID,
		State:     domain.CallStateCreated,
		StartedAt: time.Now(),
	}
	m.tasks[callID] = result
	m.running = true

	runCtx := context.Background()

	go m.run(runCtx, callID, req)
	return result, nil
}

func (m *Manager) Get(callID string) (domain.CallResult, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	res, ok := m.tasks[callID]
	return res, ok
}

func (m *Manager) run(ctx context.Context, callID string, req domain.CallRequest) {
	res, err := m.runner.Dial(ctx, req)
	if res.CallID == "" {
		res.CallID = callID
	}
	if err != nil && res.Reason == "" {
		res.Reason = err.Error()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	if existing, ok := m.tasks[callID]; ok {
		if res.StartedAt.IsZero() {
			res.StartedAt = existing.StartedAt
		}
	}
	res.CallID = callID
	m.tasks[callID] = res
	m.running = false
}
