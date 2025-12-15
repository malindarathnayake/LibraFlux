//go:build windows && lbctl_full

package shell

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

type Signal int

const (
	SignalTerm Signal = iota + 1
	SignalKill
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

type ProcessChecker interface {
	IsAlive(pid int) bool
	CommandName(pid int) (string, error)
	Signal(pid int, sig Signal) error
}

type defaultProcessChecker struct{}

func (defaultProcessChecker) IsAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(syscall.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		if errors.Is(err, syscall.ERROR_ACCESS_DENIED) {
			return true
		}
		return false
	}
	defer syscall.CloseHandle(h)

	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return false
	}
	return code == syscall.STILL_ACTIVE
}

func (defaultProcessChecker) CommandName(pid int) (string, error) {
	return "", errors.New("process command name not supported on windows")
}

func (defaultProcessChecker) Signal(pid int, sig Signal) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	switch sig {
	case SignalTerm, SignalKill:
		return p.Kill()
	default:
		return fmt.Errorf("unknown signal: %d", sig)
	}
}

type ErrLockHeld struct {
	Meta LockMetadata
	Idle time.Duration
}

func (e *ErrLockHeld) Error() string {
	if e.Meta.User == "" {
		return "configuration locked by another session"
	}
	idle := "unknown"
	if e.Idle >= 0 {
		idle = e.Idle.Round(time.Second).String()
	}
	return fmt.Sprintf("configuration locked by %s@%s (pid %d), idle %s", e.Meta.User, e.Meta.Host, e.Meta.PID, idle)
}

type AuditEmitter func(event observability.AuditEvent, fields map[string]interface{})

type LockManager struct {
	Path         string
	ExpectedComm string
	Checker      ProcessChecker
	Now          func() time.Time
	Audit        AuditEmitter

	mu sync.Mutex
}

func (m *LockManager) ensureDefaults() {
	if m.ExpectedComm == "" {
		m.ExpectedComm = "lbctl"
	}
	if m.Checker == nil {
		m.Checker = defaultProcessChecker{}
	}
	if m.Now == nil {
		m.Now = time.Now
	}
}

func DefaultIdentity() LockIdentity {
	host, _ := os.Hostname()
	u := "unknown"
	if cu, err := user.Current(); err == nil && cu != nil && cu.Username != "" {
		u = cu.Username
	}
	tty := os.Getenv("TTY")
	if tty == "" {
		tty = "unknown"
	}
	return LockIdentity{
		PID:  os.Getpid(),
		User: u,
		Host: host,
		TTY:  tty,
	}
}

type HeldLock struct {
	mgr      *LockManager
	file     *os.File
	meta     LockMetadata
	released bool
	mu       sync.Mutex
}

func (h *HeldLock) Metadata() LockMetadata {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.meta
}

func (h *HeldLock) UpdateActivity() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.released {
		return errors.New("lock already released")
	}
	h.meta.LastActivity = h.mgr.Now().UTC()
	return writeMetadata(h.file, h.meta)
}

func (h *HeldLock) Release() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.released {
		return nil
	}
	h.released = true

	if h.mgr.Audit != nil {
		duration := h.mgr.Now().UTC().Sub(h.meta.StartedAt)
		h.mgr.Audit(observability.AuditLockReleased, map[string]interface{}{
			"user":        h.meta.User,
			"pid":         h.meta.PID,
			"tty":         h.meta.TTY,
			"duration_ms": duration.Milliseconds(),
		})
	}

	_ = unlockFile(h.file)
	_ = h.file.Close()
	_ = os.Remove(h.mgr.Path)
	return nil
}

func (m *LockManager) Acquire(id LockIdentity) (*HeldLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureDefaults()

	if id.PID == 0 {
		id.PID = os.Getpid()
	}
	if id.User == "" || id.Host == "" || id.TTY == "" {
		def := DefaultIdentity()
		if id.User == "" {
			id.User = def.User
		}
		if id.Host == "" {
			id.Host = def.Host
		}
		if id.TTY == "" {
			id.TTY = def.TTY
		}
	}

	if err := os.MkdirAll(filepath.Dir(m.Path), 0755); err != nil {
		return nil, fmt.Errorf("create lock directory: %w", err)
	}

	for attempt := 0; attempt < 2; attempt++ {
		f, err := os.OpenFile(m.Path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, fmt.Errorf("open lock file: %w", err)
		}

		locked, err := tryLockExclusiveNonBlocking(f)
		if err != nil {
			_ = f.Close()
			return nil, err
		}
		if !locked {
			meta, metaErr := readMetadataFromFile(f)
			_ = f.Close()
			if metaErr == nil && m.isStale(meta) {
				if m.Audit != nil {
					m.Audit(observability.AuditLockRecovered, map[string]interface{}{
						"old_user": meta.User,
						"old_pid":  meta.PID,
						"new_user": id.User,
						"new_pid":  id.PID,
					})
				}
				_ = os.Remove(m.Path)
				continue
			}

			idle := time.Duration(-1)
			if !meta.LastActivity.IsZero() {
				idle = m.Now().UTC().Sub(meta.LastActivity)
			}
			return nil, &ErrLockHeld{Meta: meta, Idle: idle}
		}

		previous, prevErr := readMetadataFromFile(f)
		if prevErr == nil && previous.PID != 0 && previous.PID != id.PID && m.isStale(previous) {
			if m.Audit != nil {
				m.Audit(observability.AuditLockRecovered, map[string]interface{}{
					"old_user": previous.User,
					"old_pid":  previous.PID,
					"new_user": id.User,
					"new_pid":  id.PID,
				})
			}
		}

		now := m.Now().UTC()
		meta := LockMetadata{
			PID:          id.PID,
			User:         id.User,
			Host:         id.Host,
			TTY:          id.TTY,
			StartedAt:    now,
			LastActivity: now,
		}
		if err := writeMetadata(f, meta); err != nil {
			_ = unlockFile(f)
			_ = f.Close()
			return nil, err
		}

		if m.Audit != nil {
			m.Audit(observability.AuditLockAcquired, map[string]interface{}{
				"user": id.User,
				"pid":  id.PID,
				"tty":  id.TTY,
			})
		}

		return &HeldLock{mgr: m, file: f, meta: meta}, nil
	}

	return nil, errors.New("failed to recover stale lock")
}

func (m *LockManager) Status() (*LockMetadata, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureDefaults()

	f, err := os.OpenFile(m.Path, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close()

	locked, err := tryLockExclusiveNonBlocking(f)
	if err != nil {
		return nil, err
	}
	if locked {
		_ = unlockFile(f)
		return nil, nil
	}

	meta, err := readMetadataFromFile(f)
	if err != nil {
		return nil, err
	}
	return &meta, nil
}

func (m *LockManager) Break(force bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ensureDefaults()

	if !force {
		return errors.New("refusing to break lock without --force")
	}

	f, err := os.OpenFile(m.Path, os.O_RDWR, 0644)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("no configuration lock held")
		}
		return fmt.Errorf("open lock file: %w", err)
	}
	defer f.Close()

	locked, err := tryLockExclusiveNonBlocking(f)
	if err != nil {
		return err
	}
	if locked {
		_ = unlockFile(f)
		return errors.New("no configuration lock held")
	}

	meta, err := readMetadataFromFile(f)
	if err != nil {
		return err
	}
	if meta.PID == os.Getpid() {
		return errors.New("cannot break your own lock; use abort/exit")
	}

	_ = m.Checker.Signal(meta.PID, SignalTerm)
	deadline := m.Now().Add(5 * time.Second)
	for m.Checker.IsAlive(meta.PID) && m.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
	}
	if m.Checker.IsAlive(meta.PID) {
		_ = m.Checker.Signal(meta.PID, SignalKill)
	}

	_ = os.Remove(m.Path)
	if m.Audit != nil {
		m.Audit(observability.AuditLockBroken, map[string]interface{}{
			"holder_user": meta.User,
			"holder_pid":  meta.PID,
			"breaker_user": func() string {
				id := DefaultIdentity()
				return id.User
			}(),
		})
	}
	return nil
}

func (m *LockManager) isStale(meta LockMetadata) bool {
	if meta.PID <= 0 {
		return true
	}
	if !m.Checker.IsAlive(meta.PID) {
		return true
	}
	comm, err := m.Checker.CommandName(meta.PID)
	if err != nil {
		return false
	}
	return comm != m.ExpectedComm
}

func tryLockExclusiveNonBlocking(f *os.File) (bool, error) {
	var ol syscall.Overlapped
	err := syscall.LockFileEx(syscall.Handle(f.Fd()), syscall.LOCKFILE_EXCLUSIVE_LOCK|syscall.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &ol)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, syscall.ERROR_LOCK_VIOLATION) {
		return false, nil
	}
	return false, err
}

func unlockFile(f *os.File) error {
	var ol syscall.Overlapped
	return syscall.UnlockFileEx(syscall.Handle(f.Fd()), 0, 1, 0, &ol)
}

func readMetadataFromFile(f *os.File) (LockMetadata, error) {
	b, err := os.ReadFile(f.Name())
	if err != nil {
		return LockMetadata{}, err
	}
	b = bytes.TrimSpace(b)
	if len(b) == 0 {
		return LockMetadata{}, errors.New("lock metadata empty")
	}
	var meta LockMetadata
	if err := json.Unmarshal(b, &meta); err != nil {
		return LockMetadata{}, err
	}
	return meta, nil
}

func writeMetadata(f *os.File, meta LockMetadata) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := f.Truncate(0); err != nil {
		return err
	}
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}
