//go:build !windows

package shell

import (
	"bytes"
	"path/filepath"
	"testing"
	"time"
)

func TestIdleTimeoutExitsConfigureMode(t *testing.T) {
	dir := t.TempDir()
	configPath, configDir := writeTestConfig(t, dir)

	var out bytes.Buffer
	var errOut bytes.Buffer

	lockPath := filepath.Join(dir, "config.lock")
	cur := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	mgr := &LockManager{
		Path: lockPath,
		Now:  func() time.Time { return cur },
	}

	sh, err := New(ShellOptions{
		Out:         &out,
		Err:         &errOut,
		ConfigPath:  configPath,
		ConfigDir:   configDir,
		LockManager: mgr,
		IdleTimeout: 1 * time.Minute,
		Now:         func() time.Time { return cur },
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := sh.ExecuteLine("configure"); err != nil {
		t.Fatalf("configure error: %v", err)
	}
	if sh.Mode() != ModeConfig {
		t.Fatalf("expected ModeConfig, got %v", sh.Mode())
	}

	cur = cur.Add(2 * time.Minute)
	if err := sh.ExecuteLine("show"); err != nil {
		t.Fatalf("show error: %v", err)
	}
	if sh.Mode() != ModeRoot {
		t.Fatalf("expected ModeRoot after timeout, got %v", sh.Mode())
	}
}
