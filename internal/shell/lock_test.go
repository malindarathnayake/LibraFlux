//go:build !windows

package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

type fakeChecker struct {
	alive map[int]bool
	comm  map[int]string
}

func (f fakeChecker) IsAlive(pid int) bool {
	return f.alive[pid]
}

func (f fakeChecker) CommandName(pid int) (string, error) {
	return f.comm[pid], nil
}

func (f fakeChecker) Signal(pid int, sig Signal) error { return nil }

func TestLockAcquireReleaseAndStatus(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "config.lock")

	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	var events []observability.AuditEvent

	m := &LockManager{
		Path:         lockPath,
		ExpectedComm: "lbctl",
		Checker:      fakeChecker{alive: map[int]bool{1: true}, comm: map[int]string{1: "lbctl"}},
		Now:          func() time.Time { return now },
		Audit: func(e observability.AuditEvent, _ map[string]interface{}) {
			events = append(events, e)
		},
	}

	held, err := m.Acquire(LockIdentity{PID: 1, User: "alice", Host: "h", TTY: "t"})
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}

	meta, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if meta == nil || meta.PID != 1 || meta.User != "alice" {
		t.Fatalf("unexpected status meta: %#v", meta)
	}

	now2 := now.Add(2 * time.Minute)
	m.Now = func() time.Time { return now2 }
	if err := held.UpdateActivity(); err != nil {
		t.Fatalf("UpdateActivity() error: %v", err)
	}

	meta2, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if meta2 == nil || meta2.LastActivity.Before(now2) {
		t.Fatalf("expected last_activity updated, got %#v", meta2)
	}

	if err := held.Release(); err != nil {
		t.Fatalf("Release() error: %v", err)
	}

	meta3, err := m.Status()
	if err != nil {
		t.Fatalf("Status() error: %v", err)
	}
	if meta3 != nil {
		t.Fatalf("expected unlocked, got %#v", meta3)
	}

	if len(events) < 2 || events[0] != observability.AuditLockAcquired || events[len(events)-1] != observability.AuditLockReleased {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestLockRecoversStaleMetadata(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "config.lock")

	stale := LockMetadata{
		PID:          999,
		User:         "old",
		Host:         "oldhost",
		TTY:          "oldt",
		StartedAt:    time.Now().Add(-time.Hour).UTC(),
		LastActivity: time.Now().Add(-time.Hour).UTC(),
	}
	b, _ := json.Marshal(stale)
	if err := os.WriteFile(lockPath, append(b, '\n'), 0644); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}

	var recovered bool
	m := &LockManager{
		Path:         lockPath,
		ExpectedComm: "lbctl",
		Checker:      fakeChecker{alive: map[int]bool{2: true, 999: false}, comm: map[int]string{2: "lbctl"}},
		Now:          time.Now,
		Audit: func(e observability.AuditEvent, _ map[string]interface{}) {
			if e == observability.AuditLockRecovered {
				recovered = true
			}
		},
	}

	held, err := m.Acquire(LockIdentity{PID: 2, User: "new", Host: "h", TTY: "t"})
	if err != nil {
		t.Fatalf("Acquire() error: %v", err)
	}
	defer held.Release()
	if !recovered {
		t.Fatalf("expected lock recovery audit event")
	}
}
