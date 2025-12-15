package system

import (
	"fmt"
	"strings"
	"testing"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

// MockNetworkManager
type MockNetworkManager struct {
	Interfaces map[string]bool
	VIPs       map[string]bool
	Err        error
}

func (m *MockNetworkManager) CheckVIPPresent(vip string) (bool, error) {
	if m.Err != nil {
		return false, m.Err
	}
	return m.VIPs[vip], nil
}

func (m *MockNetworkManager) GetInterfaceStatus(iface string) (bool, error) {
	if m.Err != nil {
		return false, m.Err
	}
	status, ok := m.Interfaces[iface]
	if !ok {
		return false, fmt.Errorf("interface not found")
	}
	return status, nil
}

func TestDoctor(t *testing.T) {
	mockNM := &MockNetworkManager{
		Interfaces: map[string]bool{
			"eth0": true,
			"eth1": true,
			"eth2": false,
		},
		VIPs: map[string]bool{
			"192.168.1.100": true,
		},
	}
	
	doctor := NewDoctor(mockNM)
	
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "eth0", VIP: "192.168.1.100"},
			Backend:  config.InterfaceConfig{Interface: "eth1"},
		},
	}
	
	results, err := doctor.RunChecks(cfg)
	if err != nil {
		t.Fatalf("RunChecks failed: %v", err)
	}
	
	// Expect Frontend UP, Backend UP, VIP Present
	checkMap := make(map[string]CheckResult)
	for _, res := range results {
		checkMap[res.Name] = res
	}
	
	if !checkMap["Frontend Interface"].Passed {
		t.Error("Frontend Interface should pass")
	}
	
	if !checkMap["Backend Interface"].Passed {
		t.Error("Backend Interface should pass")
	}
	
	if !checkMap["VIP Check"].Passed {
		t.Error("VIP Check should pass")
	}
	if !strings.Contains(checkMap["VIP Check"].Message, "PRESENT") {
		t.Errorf("VIP Check message should indicate presence, got: %s", checkMap["VIP Check"].Message)
	}
}

func TestDoctorFailures(t *testing.T) {
	mockNM := &MockNetworkManager{
		Interfaces: map[string]bool{
			"eth0": false, // Down
		},
		VIPs: map[string]bool{},
	}
	
	doctor := NewDoctor(mockNM)
	
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "eth0", VIP: "192.168.1.100"},
		},
	}
	
	results, err := doctor.RunChecks(cfg)
	if err != nil {
		t.Fatalf("RunChecks failed: %v", err)
	}
	
	checkMap := make(map[string]CheckResult)
	for _, res := range results {
		checkMap[res.Name] = res
	}
	
	if checkMap["Frontend Interface"].Passed {
		t.Error("Frontend Interface should fail (Down)")
	}
	
	// VIP not present should still pass check logic (just status report), 
	// based on my implementation
	if !checkMap["VIP Check"].Passed {
		t.Error("VIP Check should pass (informational)")
	}
	if !strings.Contains(checkMap["VIP Check"].Message, "NOT PRESENT") {
		t.Errorf("VIP Check message should indicate absence, got: %s", checkMap["VIP Check"].Message)
	}
}

func TestDoctorInterfaceMissing(t *testing.T) {
	mockNM := &MockNetworkManager{
		Interfaces: map[string]bool{},
	}
	
	doctor := NewDoctor(mockNM)
	
	cfg := &config.Config{
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "missing0"},
		},
	}
	
	results, _ := doctor.RunChecks(cfg)
	
	if results[0].Passed {
		t.Error("Missing interface should fail")
	}
}
