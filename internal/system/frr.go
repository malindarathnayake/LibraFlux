package system

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

const (
	FRRManagedBegin = "! BEGIN LBCTL MANAGED - DO NOT EDIT"
	FRRManagedEnd   = "! END LBCTL MANAGED"
	FRRBackupDir    = "/var/lib/lbctl/backups"
)

type FRRPatcher struct {
	configPath string
	backupDir  string
}

func NewFRRPatcher(configPath string) *FRRPatcher {
	return &FRRPatcher{
		configPath: configPath,
		backupDir:  FRRBackupDir,
	}
}

// SetBackupDir overrides the backup directory (for testing)
func (p *FRRPatcher) SetBackupDir(dir string) {
	p.backupDir = dir
}

// Patch updates the managed block in the FRR config
func (p *FRRPatcher) Patch(cfg *config.Config) error {
	// 1. Read existing config
	content, err := os.ReadFile(p.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// If file doesn't exist, create it with managed block
			content = []byte{}
		} else {
			return fmt.Errorf("failed to read FRR config: %w", err)
		}
	}

	// 2. Generate new managed block
	newBlock := generateManagedBlock(cfg)

	// 3. Replace or Append
	newContent, err := replaceManagedBlock(content, newBlock)
	if err != nil {
		return err
	}
	
	// 4. Backup
	if err := p.backup(content); err != nil {
		// Log warning but proceed? Or fail? Spec says "Back up full file before first patch"
		// and "Back up managed block".
		// For simplicity, we backup the full file if it exists.
		return fmt.Errorf("failed to backup FRR config: %w", err)
	}

	// 5. Write new config
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(p.configPath), 0755); err != nil {
		return err
	}
	
	if err := os.WriteFile(p.configPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write FRR config: %w", err)
	}

	return nil
}

func (p *FRRPatcher) backup(content []byte) error {
	if len(content) == 0 {
		return nil
	}
	
	if err := os.MkdirAll(p.backupDir, 0750); err != nil {
		return err
	}
	
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(p.backupDir, fmt.Sprintf("frr.conf.%s", timestamp))
	
	return os.WriteFile(backupPath, content, 0640)
}

func generateManagedBlock(cfg *config.Config) string {
	var sb strings.Builder
	
	sb.WriteString(FRRManagedBegin)
	sb.WriteString("\n")
	
	sb.WriteString(fmt.Sprintf("interface %s\n", cfg.Network.Frontend.Interface))
	sb.WriteString(fmt.Sprintf(" vrrp %d version 3\n", cfg.VRRP.VRID))
	
	priority := cfg.VRRP.PriorityPrimary
	if cfg.Node.Role == "secondary" {
		priority = cfg.VRRP.PrioritySecondary
	}
	sb.WriteString(fmt.Sprintf(" vrrp %d priority %d\n", cfg.VRRP.VRID, priority))
	
	// advert_interval_ms to centiseconds (ms / 10)
	advert := cfg.VRRP.AdvertIntervalMS / 10
	if advert < 1 {
		advert = 100 // Default to 1s if invalid
	}
	sb.WriteString(fmt.Sprintf(" vrrp %d advertisement-interval %d\n", cfg.VRRP.VRID, advert))
	
	if cfg.Network.Frontend.VIP != "" {
		sb.WriteString(fmt.Sprintf(" vrrp %d ip %s\n", cfg.VRRP.VRID, cfg.Network.Frontend.VIP))
	}
	
	sb.WriteString(FRRManagedEnd)
	sb.WriteString("\n")
	
	return sb.String()
}

func replaceManagedBlock(content []byte, newBlock string) ([]byte, error) {
	s := string(content)
	
	startIdx := strings.Index(s, FRRManagedBegin)
	endIdx := strings.Index(s, FRRManagedEnd)
	
	if startIdx == -1 {
		// Block not found, append to end (with newline if needed)
		if len(s) > 0 && !strings.HasSuffix(s, "\n") {
			s += "\n"
		}
		return []byte(s + newBlock), nil
	}
	
	if endIdx == -1 {
		return nil, fmt.Errorf("found managed block start but no end")
	}
	
	// Include the end marker and newline in replacement
	endOfBlock := endIdx + len(FRRManagedEnd)
	if endOfBlock < len(s) && s[endOfBlock] == '\n' {
		endOfBlock++
	}
	
	// Construct new content
	before := s[:startIdx]
	after := s[endOfBlock:]
	
	return []byte(before + newBlock + after), nil
}
