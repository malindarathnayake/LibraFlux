package system

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

type SysctlManager struct {
	path string
}

func NewSysctlManager(path string) *SysctlManager {
	return &SysctlManager{path: path}
}

func (s *SysctlManager) Apply(cfg *config.Config) error {
	// 1. Generate content
	content := s.generate(cfg)
	
	// 2. Write file
	if err := os.MkdirAll(filepath.Dir(s.path), 0755); err != nil {
		return err
	}
	
	if err := os.WriteFile(s.path, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write sysctl file: %w", err)
	}
	
	// 3. Apply (mockable or real?)
	// For this exercise, we just generate the file.
	// Real implementation would exec "sysctl --system" or similar.
	
	return nil
}

func (s *SysctlManager) generate(cfg *config.Config) string {
	var sb strings.Builder
	
	sb.WriteString("# lbctl managed sysctl configuration\n")
	sb.WriteString(fmt.Sprintf("# Mode: %s\n", cfg.Mode))
	sb.WriteString(fmt.Sprintf("# Profile: %s\n\n", cfg.System.TuningProfile))
	
	// Mode specific
	sb.WriteString("# Mode settings\n")
	if cfg.Mode == "nat" {
		sb.WriteString("net.ipv4.ip_forward = 1\n")
		sb.WriteString("net.ipv4.vs.conntrack = 1\n")
	} else {
		// DR
		sb.WriteString("net.ipv4.ip_forward = 1\n")
	}
	sb.WriteString("\n")
	
	// Tuning profile
	sb.WriteString("# Tuning profile settings\n")
	profile := GetTuningProfile(cfg.System.TuningProfile)
	
	// Sort keys for deterministic output
	keys := make([]string, 0, len(profile))
	for k := range profile {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	
	for _, k := range keys {
		sb.WriteString(fmt.Sprintf("%s = %s\n", k, profile[k]))
	}
	
	return sb.String()
}
