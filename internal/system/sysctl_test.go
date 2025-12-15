package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

func TestSysctlGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "99-lbctl.conf")
	
	mgr := NewSysctlManager(path)
	
	// Test DR Mode + Minimal
	cfg := &config.Config{
		Mode: "dr",
		System: config.SystemConfig{
			TuningProfile: "minimal",
		},
	}
	
	if err := mgr.Apply(cfg); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
	
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	
	if !strings.Contains(s, "Mode: dr") {
		t.Error("Missing mode header")
	}
	if !strings.Contains(s, "net.ipv4.ip_forward = 1") {
		t.Error("Missing ip_forward")
	}
	if strings.Contains(s, "net.ipv4.vs.conntrack") {
		t.Error("conntrack should not be present in DR mode")
	}
	
	// Check minimal profile settings
	if !strings.Contains(s, "net.ipv4.vs.conn_tab_bits = 12") {
		t.Error("Minimal profile setting missing")
	}
	
	// Test NAT Mode + Aggressive
	cfg.Mode = "nat"
	cfg.System.TuningProfile = "aggressive"
	
	if err := mgr.Apply(cfg); err != nil {
		t.Fatalf("Apply() failed: %v", err)
	}
	
	content, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s = string(content)
	
	if !strings.Contains(s, "Mode: nat") {
		t.Error("Missing mode header")
	}
	if !strings.Contains(s, "net.ipv4.vs.conntrack = 1") {
		t.Error("Missing conntrack in NAT mode")
	}
	
	// Check aggressive profile settings
	if !strings.Contains(s, "net.ipv4.vs.conn_tab_bits = 20") {
		t.Error("Aggressive profile setting missing")
	}
}

func TestGetTuningProfile(t *testing.T) {
	p := GetTuningProfile("minimal")
	if p["net.ipv4.vs.conn_tab_bits"] != "12" {
		t.Error("Expected minimal profile")
	}
	
	p = GetTuningProfile("unknown")
	if p["net.ipv4.vs.conn_tab_bits"] != "18" { // Balanced default
		t.Error("Expected balanced profile for unknown")
	}
}
