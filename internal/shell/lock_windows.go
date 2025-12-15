//go:build windows

package shell

import (
	"errors"
	"time"
)

type LockMetadata struct {
	PID          int       `json:"pid"`
	User         string    `json:"user"`
	Host         string    `json:"host"`
	TTY          string    `json:"tty"`
	StartedAt    time.Time `json:"started_at"`
	LastActivity time.Time `json:"last_activity"`
}

type LockIdentity struct {
	PID  int
	User string
	Host string
	TTY  string
}

type ErrLockHeld struct {
	Meta LockMetadata
	Idle time.Duration
}

func (e *ErrLockHeld) Error() string { return "configuration lock is not supported on windows" }

type HeldLock struct{}

func (h *HeldLock) Metadata() LockMetadata { return LockMetadata{} }
func (h *HeldLock) UpdateActivity() error  { return nil }
func (h *HeldLock) Release() error         { return nil }

type LockManager struct {
	Path string
	Now  func() time.Time
}

func DefaultIdentity() LockIdentity { return LockIdentity{} }

func (m *LockManager) Acquire(_ LockIdentity) (*HeldLock, error) {
	return nil, errors.New("configuration locking is not supported on windows")
}

func (m *LockManager) Status() (*LockMetadata, error) { return nil, nil }
func (m *LockManager) Break(_ bool) error             { return errors.New("configuration locking is not supported on windows") }

