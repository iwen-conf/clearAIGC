package service

import (
	"sync"

	"github.com/google/uuid"
)

type executionState struct {
	RoundID        uuid.UUID
	PauseRequested bool
}

type ExecutionManager struct {
	mu         sync.RWMutex
	executions map[uuid.UUID]*executionState
}

func NewExecutionManager() *ExecutionManager {
	return &ExecutionManager{
		executions: map[uuid.UUID]*executionState{},
	}
}

func (m *ExecutionManager) Register(sessionID, roundID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executions[sessionID] = &executionState{RoundID: roundID}
}

func (m *ExecutionManager) Unregister(sessionID uuid.UUID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.executions, sessionID)
}

func (m *ExecutionManager) RequestPause(sessionID uuid.UUID) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	execution, ok := m.executions[sessionID]
	if !ok {
		return false
	}
	execution.PauseRequested = true
	return true
}

func (m *ExecutionManager) ShouldPause(sessionID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	execution, ok := m.executions[sessionID]
	return ok && execution.PauseRequested
}

func (m *ExecutionManager) IsRunning(sessionID uuid.UUID) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.executions[sessionID]
	return ok
}
