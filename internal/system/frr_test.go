package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

func TestFRRPatcher(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "frr.conf")
	backupDir := filepath.Join(tmpDir, "backups")
	
	patcher := NewFRRPatcher(configPath)
	patcher.SetBackupDir(backupDir)
	
	// Initial content
	initialContent := `
! Unmanaged content
router bgp 65000
 bgp router-id 1.2.3.4
!
`
	if err := os.WriteFile(configPath, []byte(initialContent), 0644); err != nil {
		t.Fatal(err)
	}
	
	cfg := &config.Config{
		Node: config.NodeConfig{Role: "primary"},
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "eth0", VIP: "192.168.1.100"},
		},
		VRRP: config.VRRPConfig{
			VRID:             50,
			PriorityPrimary:  150,
			AdvertIntervalMS: 1000,
		},
	}
	
	// Test Patch (Append)
	if err := patcher.Patch(cfg); err != nil {
		t.Fatalf("Patch() failed: %v", err)
	}
	
	// Verify content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	
	if !strings.Contains(s, "! Unmanaged content") {
		t.Error("Unmanaged content lost")
	}
	if !strings.Contains(s, FRRManagedBegin) {
		t.Error("Managed block start missing")
	}
	if !strings.Contains(s, "priority 150") {
		t.Error("Priority 150 missing")
	}
	if !strings.Contains(s, "advertisement-interval 100") {
		t.Error("advertisement-interval 100 missing") // 1000ms / 10 = 100
	}
	
	// Verify backup
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("Expected 1 backup, got %d", len(entries))
	}
	
	// Test Patch (Replace)
	// Change config
	cfg.Node.Role = "secondary"
	cfg.VRRP.PrioritySecondary = 100
	
	if err := patcher.Patch(cfg); err != nil {
		t.Fatalf("Patch() failed: %v", err)
	}
	
	content, err = os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s = string(content)
	
	if !strings.Contains(s, "! Unmanaged content") {
		t.Error("Unmanaged content lost")
	}
	if !strings.Contains(s, "priority 100") {
		t.Error("Priority 100 missing (secondary)")
	}
	
	// Ensure we don't have duplicates
	if strings.Count(s, FRRManagedBegin) != 1 {
		t.Errorf("Expected 1 managed block, got %d", strings.Count(s, FRRManagedBegin))
	}
}

func TestFRRNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "frr.conf")
	
	patcher := NewFRRPatcher(configPath)
	patcher.SetBackupDir(filepath.Join(tmpDir, "backups"))
	
	cfg := &config.Config{
		Node: config.NodeConfig{Role: "primary"},
		Network: config.NetworkConfig{Frontend: config.InterfaceConfig{Interface: "eth0"}},
		VRRP: config.VRRPConfig{VRID: 10},
	}
	
	if err := patcher.Patch(cfg); err != nil {
		t.Fatalf("Patch() failed: %v", err)
	}
	
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)
	
	if !strings.Contains(s, FRRManagedBegin) {
		t.Error("Managed block missing")
	}
}
