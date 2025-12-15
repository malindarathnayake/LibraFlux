//go:build !windows

package shell

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShellRootHelpAndCompletion(t *testing.T) {
	dir := t.TempDir()
	configPath, configDir := writeTestConfig(t, dir)

	var out bytes.Buffer
	var errOut bytes.Buffer
	now := time.Now
	lockPath := filepath.Join(dir, "config.lock")
	mgr := &LockManager{Path: lockPath, ExpectedComm: "lbctl", Now: now}

	sh, err := New(ShellOptions{
		Out:         &out,
		Err:         &errOut,
		ConfigPath:  configPath,
		ConfigDir:   configDir,
		LockManager: mgr,
		Now:         now,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	if err := sh.ExecuteLine("help"); err != nil {
		t.Fatalf("help error: %v", err)
	}
	if got := out.String(); !bytes.Contains([]byte(got), []byte("configure")) {
		t.Fatalf("expected help to mention configure, got: %s", got)
	}

	compl := sh.Complete("sh")
	found := false
	for _, c := range compl {
		if c == "show" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected completion to include show, got %#v", compl)
	}
}

func TestShellConfigureServiceCommit(t *testing.T) {
	dir := t.TempDir()
	configPath, configDir := writeTestConfig(t, dir)

	var out bytes.Buffer
	var errOut bytes.Buffer

	lockPath := filepath.Join(dir, "config.lock")
	mgr := &LockManager{Path: lockPath, ExpectedComm: "lbctl"}
	sh, err := New(ShellOptions{
		Out:         &out,
		Err:         &errOut,
		ConfigPath:  configPath,
		ConfigDir:   configDir,
		LockManager: mgr,
	})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	steps := []string{
		"configure service svc1",
		"protocol udp",
		"port-range 1000-1001",
		"scheduler sh",
		"backend 10.0.0.1",
		"health tcp port 8080 interval 1000 timeout 300",
		"exit",
		"commit",
		"exit",
	}
	for _, step := range steps {
		if err := sh.ExecuteLine(step); err != nil {
			t.Fatalf("step %q error: %v", step, err)
		}
	}

	if _, err := os.Stat(filepath.Join(configDir, "svc1.yaml")); err != nil {
		t.Fatalf("expected service file written: %v", err)
	}
}

func writeTestConfig(t *testing.T, dir string) (string, string) {
	t.Helper()

	configDir := filepath.Join(dir, "config.d")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	configPath := filepath.Join(dir, "config.yaml")
	data := []byte(`mode: dr
node:
  name: n1
  role: primary
network:
  frontend:
    interface: eth0
    vip: 192.168.0.1
    cidr: 24
  backend:
    interface: eth1
vrrp:
  vrid: 1
  priority_primary: 150
  priority_secondary: 100
  advert_interval_ms: 1000
observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: false
      host: ""
      port: 0
      protocol: udp
      facility: lbctl
  metrics:
    influxdb:
      enabled: false
      url: ""
      token: ""
      org: ""
      bucket: ""
      push_interval_seconds: 0
    prometheus:
      enabled: false
      port: 0
      path: "/metrics"
system:
  state_dir: varliblbctl
  frr_config: etcfrrfrr.conf
  sysctl_file: etcsysctl.d99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
include: "config.d/*.yaml"
`)
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath, configDir
}
