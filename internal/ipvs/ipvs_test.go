package ipvs

import (
	"fmt"
	"testing"

	"github.com/malindarathnayake/LibraFlux/internal/config"
	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

type MockManager struct {
	Services     map[string]*Service
	Destinations map[string][]*Destination
}

func NewMockManager() *MockManager {
	return &MockManager{
		Services:     make(map[string]*Service),
		Destinations: make(map[string][]*Destination),
	}
}

func (m *MockManager) GetServices() ([]*Service, error) {
	var svcs []*Service
	for _, s := range m.Services {
		svcs = append(svcs, s)
	}
	return svcs, nil
}

func (m *MockManager) GetDestinations(svc *Service) ([]*Destination, error) {
	return m.Destinations[svc.Key()], nil
}

func (m *MockManager) CreateService(svc *Service) error {
	m.Services[svc.Key()] = svc
	return nil
}

func (m *MockManager) UpdateService(svc *Service) error {
	m.Services[svc.Key()] = svc
	return nil
}

func (m *MockManager) DeleteService(svc *Service) error {
	delete(m.Services, svc.Key())
	delete(m.Destinations, svc.Key())
	return nil
}

func (m *MockManager) CreateDestination(svc *Service, dst *Destination) error {
	key := svc.Key()
	m.Destinations[key] = append(m.Destinations[key], dst)
	return nil
}

func (m *MockManager) UpdateDestination(svc *Service, dst *Destination) error {
	key := svc.Key()
	dests := m.Destinations[key]
	for i, d := range dests {
		if d.Key() == dst.Key() {
			dests[i] = dst
			return nil
		}
	}
	return fmt.Errorf("destination not found")
}

func (m *MockManager) DeleteDestination(svc *Service, dst *Destination) error {
	key := svc.Key()
	dests := m.Destinations[key]
	newDests := make([]*Destination, 0, len(dests))
	for _, d := range dests {
		if d.Key() != dst.Key() {
			newDests = append(newDests, d)
		}
	}
	m.Destinations[key] = newDests
	return nil
}

func TestReconciler(t *testing.T) {
	mock := NewMockManager()
	logger := observability.NewLogger(observability.DebugLevel)
	reconciler := NewReconciler(mock, logger)

	vip := "192.168.1.100"

	// 1. Initial Apply (Create)
	desired := []config.Service{
		{
			Name:      "test-svc",
			Protocol:  "tcp",
			Ports:     []int{80, 443},
			Scheduler: "rr",
			Backends: []config.Backend{
				{Address: "10.0.0.1", Port: 80, Weight: 1},
			},
		},
	}

	if err := reconciler.Apply(desired, vip); err != nil {
		t.Fatalf("Apply failed: %v", err)
	}

	// Verify services created
	// 2 services (80, 443)
	if len(mock.Services) != 2 {
		t.Errorf("Expected 2 services, got %d", len(mock.Services))
	}

	// Check Service 80
	key80 := fmt.Sprintf("tcp:%s:80", vip)
	if svc, ok := mock.Services[key80]; !ok {
		t.Error("Service 80 not created")
	} else {
		if svc.Scheduler != "rr" {
			t.Errorf("Expected rr scheduler, got %s", svc.Scheduler)
		}
		// Check Destinations
		dests := mock.Destinations[key80]
		if len(dests) != 1 {
			t.Errorf("Expected 1 destination, got %d", len(dests))
		}
		if dests[0].Port != 80 {
			t.Errorf("Expected destination port 80, got %d", dests[0].Port)
		}
	}

	// Check Service 443
	key443 := fmt.Sprintf("tcp:%s:443", vip)
	if _, ok := mock.Services[key443]; !ok {
		t.Error("Service 443 not created")
	} else {
		dests := mock.Destinations[key443]
		// Backend config says Port: 80. So for 443 service, dest port is 80.
		if dests[0].Port != 80 {
			t.Errorf("Expected destination port 80 (from backend config), got %d", dests[0].Port)
		}
	}

	// 2. Update (Change Scheduler)
	desired[0].Scheduler = "wrr"
	if err := reconciler.Apply(desired, vip); err != nil {
		t.Fatalf("Apply update failed: %v", err)
	}

	if mock.Services[key80].Scheduler != "wrr" {
		t.Error("Scheduler not updated to wrr")
	}

	// 3. Update (Change Backend Weight)
	desired[0].Backends[0].Weight = 2
	if err := reconciler.Apply(desired, vip); err != nil {
		t.Fatalf("Apply update weight failed: %v", err)
	}

	if mock.Destinations[key80][0].Weight != 2 {
		t.Error("Weight not updated to 2")
	}

	// 4. Delete (Remove Service 443)
	desired[0].Ports = []int{80} // Remove 443
	if err := reconciler.Apply(desired, vip); err != nil {
		t.Fatalf("Apply delete failed: %v", err)
	}

	if len(mock.Services) != 1 {
		t.Errorf("Expected 1 service, got %d", len(mock.Services))
	}
	if _, ok := mock.Services[key443]; ok {
		t.Error("Service 443 not deleted")
	}
}

func TestExpandConfig(t *testing.T) {
	// Test port ranges and port 0 handling
	r := &Reconciler{}
	vip := "192.168.1.100"

	desired := []config.Service{
		{
			Name:       "range-svc",
			Protocol:   "udp",
			PortRanges: []config.PortRange{{Start: 100, End: 102}}, // 100, 101, 102
			Backends: []config.Backend{
				{Address: "10.0.0.1", Port: 0, Weight: 1}, // Same port
			},
		},
	}

	state, err := r.expandConfig(desired, vip)
	if err != nil {
		t.Fatalf("expandConfig failed: %v", err)
	}

	if len(state) != 3 {
		t.Errorf("Expected 3 services, got %d", len(state))
	}

	// Check port 100
	key := fmt.Sprintf("udp:%s:100", vip)
	if s, ok := state[key]; !ok {
		t.Error("Service 100 missing")
	} else {
		if s.Destinations[0].Port != 100 {
			t.Errorf("Expected dest port 100 (inherited), got %d", s.Destinations[0].Port)
		}
	}
}
