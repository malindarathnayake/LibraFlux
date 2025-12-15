package ipvs

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// mockManager is a mock implementation of Manager for testing
type mockManager struct {
	mu            sync.Mutex
	services      []*Service
	destinations  map[string][]*Destination
	getCallCount  int32
	destCallCount int32
	failNext      bool
}

func newMockManager() *mockManager {
	return &mockManager{
		destinations: make(map[string][]*Destination),
	}
}

func (m *mockManager) GetServices() ([]*Service, error) {
	atomic.AddInt32(&m.getCallCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return nil, errors.New("mock error")
	}
	// Return a copy
	result := make([]*Service, len(m.services))
	for i, svc := range m.services {
		copied := *svc
		result[i] = &copied
	}
	return result, nil
}

func (m *mockManager) GetDestinations(svc *Service) ([]*Destination, error) {
	atomic.AddInt32(&m.destCallCount, 1)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.failNext {
		m.failNext = false
		return nil, errors.New("mock error")
	}
	key := svc.Key()
	dests := m.destinations[key]
	result := make([]*Destination, len(dests))
	for i, dst := range dests {
		copied := *dst
		result[i] = &copied
	}
	return result, nil
}

func (m *mockManager) CreateService(svc *Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services = append(m.services, svc)
	return nil
}

func (m *mockManager) UpdateService(svc *Service) error {
	return nil
}

func (m *mockManager) DeleteService(svc *Service) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, s := range m.services {
		if s.Key() == svc.Key() {
			m.services = append(m.services[:i], m.services[i+1:]...)
			break
		}
	}
	return nil
}

func (m *mockManager) CreateDestination(svc *Service, dst *Destination) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := svc.Key()
	m.destinations[key] = append(m.destinations[key], dst)
	return nil
}

func (m *mockManager) UpdateDestination(svc *Service, dst *Destination) error {
	return nil
}

func (m *mockManager) DeleteDestination(svc *Service, dst *Destination) error {
	return nil
}

func (m *mockManager) setServices(services []*Service) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services = services
}

func (m *mockManager) setDestinations(svcKey string, dests []*Destination) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.destinations[svcKey] = dests
}

func (m *mockManager) setFailNext() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failNext = true
}

func (m *mockManager) getGetCallCount() int32 {
	return atomic.LoadInt32(&m.getCallCount)
}

func (m *mockManager) getDestCallCount() int32 {
	return atomic.LoadInt32(&m.destCallCount)
}

// Tests

func TestCachedManager_CacheHit(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Second}
	cached := NewCachedManager(mock, cfg)

	// First call - cache miss
	services1, err := cached.GetServices()
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if len(services1) != 1 {
		t.Errorf("expected 1 service, got %d", len(services1))
	}

	// Second call - cache hit (should not call mock again)
	services2, err := cached.GetServices()
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}
	if len(services2) != 1 {
		t.Errorf("expected 1 service, got %d", len(services2))
	}

	// Mock should only be called once
	if mock.getGetCallCount() != 1 {
		t.Errorf("expected 1 call to mock, got %d", mock.getGetCallCount())
	}

	// Verify stats
	hits, misses := cached.Stats()
	if hits != 1 {
		t.Errorf("expected 1 hit, got %d", hits)
	}
	if misses != 1 {
		t.Errorf("expected 1 miss, got %d", misses)
	}
}

func TestCachedManager_CacheExpiry(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	// Very short TTL
	cfg := CacheConfig{Enabled: true, TTL: 10 * time.Millisecond}
	cached := NewCachedManager(mock, cfg)

	// First call
	_, err := cached.GetServices()
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}

	// Wait for cache to expire
	time.Sleep(20 * time.Millisecond)

	// Second call - cache should be expired
	_, err = cached.GetServices()
	if err != nil {
		t.Fatalf("GetServices failed: %v", err)
	}

	// Mock should be called twice
	if mock.getGetCallCount() != 2 {
		t.Errorf("expected 2 calls to mock after expiry, got %d", mock.getGetCallCount())
	}
}

func TestCachedManager_InvalidateOnWrite(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Hour} // Long TTL
	cached := NewCachedManager(mock, cfg)

	// Populate cache
	_, _ = cached.GetServices()
	if mock.getGetCallCount() != 1 {
		t.Errorf("expected 1 call, got %d", mock.getGetCallCount())
	}

	// Create a service (should invalidate cache)
	newSvc := &Service{Address: parseIP("10.0.0.2"), Protocol: "tcp", Port: 443, Scheduler: "wrr"}
	_ = cached.CreateService(newSvc)

	// Next read should hit mock again
	_, _ = cached.GetServices()
	if mock.getGetCallCount() != 2 {
		t.Errorf("expected 2 calls after invalidation, got %d", mock.getGetCallCount())
	}
}

func TestCachedManager_Disabled(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: false, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	// Multiple calls should all hit the mock
	_, _ = cached.GetServices()
	_, _ = cached.GetServices()
	_, _ = cached.GetServices()

	if mock.getGetCallCount() != 3 {
		t.Errorf("expected 3 calls when cache disabled, got %d", mock.getGetCallCount())
	}
}

func TestCachedManager_CopySemantics(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	// Get services
	services1, _ := cached.GetServices()

	// Modify the returned slice
	services1[0].Port = 9999

	// Get services again
	services2, _ := cached.GetServices()

	// Original cached value should be unchanged
	if services2[0].Port == 9999 {
		t.Error("cache returned reference instead of copy - mutation leaked")
	}
}

func TestCachedManager_Destinations(t *testing.T) {
	mock := newMockManager()
	svc := &Service{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"}
	mock.setServices([]*Service{svc})
	mock.setDestinations(svc.Key(), []*Destination{
		{Address: parseIP("192.168.1.1"), Port: 8080, Weight: 1},
		{Address: parseIP("192.168.1.2"), Port: 8080, Weight: 2},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	// First call - miss
	dests1, err := cached.GetDestinations(svc)
	if err != nil {
		t.Fatalf("GetDestinations failed: %v", err)
	}
	if len(dests1) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(dests1))
	}

	// Second call - hit
	dests2, err := cached.GetDestinations(svc)
	if err != nil {
		t.Fatalf("GetDestinations failed: %v", err)
	}
	if len(dests2) != 2 {
		t.Errorf("expected 2 destinations, got %d", len(dests2))
	}

	// Mock should only be called once
	if mock.getDestCallCount() != 1 {
		t.Errorf("expected 1 call to mock, got %d", mock.getDestCallCount())
	}
}

func TestCachedManager_ErrorHandling(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	// Set mock to fail
	mock.setFailNext()

	// Should return error and not cache anything
	_, err := cached.GetServices()
	if err == nil {
		t.Error("expected error from mock")
	}

	// Next call should try again (no bad data cached)
	services, err := cached.GetServices()
	if err != nil {
		t.Fatalf("second GetServices failed: %v", err)
	}
	if len(services) != 1 {
		t.Errorf("expected 1 service, got %d", len(services))
	}
}

func TestCachedManager_ConcurrentAccess(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: 50 * time.Millisecond}
	cached := NewCachedManager(mock, cfg)

	var wg sync.WaitGroup
	errors := make(chan error, 100)

	// Spawn multiple goroutines reading concurrently
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_, err := cached.GetServices()
				if err != nil {
					errors <- err
				}
				// Occasionally invalidate
				if j%20 == 0 {
					cached.Invalidate()
				}
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("concurrent access error: %v", err)
	}
}

func TestCachedManager_ManualInvalidate(t *testing.T) {
	mock := newMockManager()
	mock.setServices([]*Service{
		{Address: parseIP("10.0.0.1"), Protocol: "tcp", Port: 80, Scheduler: "rr"},
	})

	cfg := CacheConfig{Enabled: true, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	// Populate cache
	_, _ = cached.GetServices()

	// Manual invalidate
	cached.Invalidate()

	// Next read should hit mock
	_, _ = cached.GetServices()

	if mock.getGetCallCount() != 2 {
		t.Errorf("expected 2 calls after manual invalidation, got %d", mock.getGetCallCount())
	}
}

func TestCachedManager_Inner(t *testing.T) {
	mock := newMockManager()
	cfg := CacheConfig{Enabled: true, TTL: time.Hour}
	cached := NewCachedManager(mock, cfg)

	if cached.Inner() != mock {
		t.Error("Inner() should return the wrapped manager")
	}

	if !cached.Enabled() {
		t.Error("Enabled() should return true")
	}
}

func TestDefaultCacheConfig(t *testing.T) {
	cfg := DefaultCacheConfig()

	if !cfg.Enabled {
		t.Error("default config should have caching enabled")
	}

	if cfg.TTL != 500*time.Millisecond {
		t.Errorf("expected 500ms TTL, got %v", cfg.TTL)
	}
}

// Helper to parse IP for tests
func parseIP(s string) []byte {
	// Simple parse - tests use valid IPs
	parts := make([]byte, 4)
	var a, b, c, d int
	_, _ = parseIPParts(s, &a, &b, &c, &d)
	parts[0] = byte(a)
	parts[1] = byte(b)
	parts[2] = byte(c)
	parts[3] = byte(d)
	return parts
}

func parseIPParts(s string, a, b, c, d *int) (int, error) {
	n, err := sscanf(s, "%d.%d.%d.%d", a, b, c, d)
	return n, err
}

// Simple sscanf for IP parsing
func sscanf(s, format string, args ...interface{}) (int, error) {
	var a, b, c, d int
	n := 0
	_, err := parseIPManual(s, &a, &b, &c, &d)
	if err == nil {
		if len(args) > 0 {
			*args[0].(*int) = a
			n++
		}
		if len(args) > 1 {
			*args[1].(*int) = b
			n++
		}
		if len(args) > 2 {
			*args[2].(*int) = c
			n++
		}
		if len(args) > 3 {
			*args[3].(*int) = d
			n++
		}
	}
	return n, err
}

func parseIPManual(s string, a, b, c, d *int) (int, error) {
	var parts [4]int
	part := 0
	idx := 0
	for i := 0; i <= len(s); i++ {
		if i == len(s) || s[i] == '.' {
			if idx >= 4 {
				return 0, errors.New("too many parts")
			}
			parts[idx] = part
			idx++
			part = 0
		} else if s[i] >= '0' && s[i] <= '9' {
			part = part*10 + int(s[i]-'0')
		}
	}
	*a, *b, *c, *d = parts[0], parts[1], parts[2], parts[3]
	return 4, nil
}

