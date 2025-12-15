package system

import (
	"fmt"
	"os"
	"strings"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

type CheckResult struct {
	Name    string
	Passed  bool
	Message string
}

type Doctor struct {
	netManager NetworkManager
}

func NewDoctor(nm NetworkManager) *Doctor {
	return &Doctor{
		netManager: nm,
	}
}

func (d *Doctor) RunChecks(cfg *config.Config) ([]CheckResult, error) {
	var results []CheckResult

	// Check Frontend Interface
	passed, err := d.netManager.GetInterfaceStatus(cfg.Network.Frontend.Interface)
	msg := fmt.Sprintf("Interface %s UP", cfg.Network.Frontend.Interface)
	if err != nil {
		passed = false
		msg = fmt.Sprintf("Interface %s check failed: %v", cfg.Network.Frontend.Interface, err)
	} else if !passed {
		msg = fmt.Sprintf("Interface %s is DOWN", cfg.Network.Frontend.Interface)
	}
	results = append(results, CheckResult{"Frontend Interface", passed, msg})

	// Check Backend Interface
	if cfg.Network.Backend.Interface != "" {
		passed, err = d.netManager.GetInterfaceStatus(cfg.Network.Backend.Interface)
		msg = fmt.Sprintf("Interface %s UP", cfg.Network.Backend.Interface)
		if err != nil {
			passed = false
			msg = fmt.Sprintf("Interface %s check failed: %v", cfg.Network.Backend.Interface, err)
		} else if !passed {
			msg = fmt.Sprintf("Interface %s is DOWN", cfg.Network.Backend.Interface)
		}
		results = append(results, CheckResult{"Backend Interface", passed, msg})
	}

	// Check VIP
	if cfg.Network.Frontend.VIP != "" {
		present, err := d.netManager.CheckVIPPresent(cfg.Network.Frontend.VIP)
		if err != nil {
			msg = fmt.Sprintf("VIP check failed: %v", err)
			results = append(results, CheckResult{"VIP Check", false, msg})
		} else {
			if present {
				msg = fmt.Sprintf("VIP %s PRESENT", cfg.Network.Frontend.VIP)
			} else {
				msg = fmt.Sprintf("VIP %s NOT PRESENT (Standby?)", cfg.Network.Frontend.VIP)
			}
			results = append(results, CheckResult{"VIP Check", true, msg})
		}
	}
	
	// Check Kernel Modules
	// We verify if /proc/modules exists and is readable
	if _, err := os.Stat("/proc/modules"); err == nil {
		content, err := os.ReadFile("/proc/modules")
		if err == nil {
			modules := []string{"ip_vs", "ip_vs_rr", "ip_vs_wrr", "ip_vs_sh"}
			modulesContent := string(content)
			for _, mod := range modules {
				if strings.Contains(modulesContent, mod) {
					results = append(results, CheckResult{"Kernel Module " + mod, true, "Loaded"})
				} else {
					results = append(results, CheckResult{"Kernel Module " + mod, false, "Not Loaded"})
				}
			}
		} else {
			results = append(results, CheckResult{"Kernel Modules", false, fmt.Sprintf("Cannot read /proc/modules: %v", err)})
		}
	} else {
		// Likely not Linux or restricted, skip strictly or fail?
		// Spec says "Kernel module ip_vs LOADED", so it's expected.
		// In tests (often not root/linux), we might want to skip or mock.
		// For now, we just add a failed check if not present.
		results = append(results, CheckResult{"Kernel Modules", false, "Cannot access /proc/modules"})
	}

	return results, nil
}
