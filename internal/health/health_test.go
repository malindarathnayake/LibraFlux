package health

import (
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

type stubConn struct{}

func (stubConn) Read([]byte) (int, error)         { return 0, nil }
func (stubConn) Write([]byte) (int, error)        { return 0, nil }
func (stubConn) Close() error                     { return nil }
func (stubConn) LocalAddr() net.Addr              { return nil }
func (stubConn) RemoteAddr() net.Addr             { return nil }
func (stubConn) SetDeadline(time.Time) error      { return nil }
func (stubConn) SetReadDeadline(time.Time) error  { return nil }
func (stubConn) SetWriteDeadline(time.Time) error { return nil }

type fakeDialer struct {
	mu    sync.Mutex
	calls []string
	err   error
}

func (d *fakeDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	d.mu.Lock()
	d.calls = append(d.calls, network+" "+address+" "+timeout.String())
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return nil, err
	}
	return stubConn{}, nil
}

func TestHealthTCPChecker(t *testing.T) {
	d := &fakeDialer{}
	c := &TCPChecker{Dialer: d}

	if err := c.Check("10.0.0.1", 8080, 50*time.Millisecond); err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	d.mu.Lock()
	if len(d.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(d.calls))
	}
	if d.calls[0] != "tcp 10.0.0.1:8080 50ms" {
		t.Fatalf("unexpected dial call: %q", d.calls[0])
	}
	d.mu.Unlock()

	// Test error case - must be outside the lock to avoid deadlock
	d.mu.Lock()
	d.err = errors.New("dial failed")
	d.mu.Unlock()

	if err := c.Check("10.0.0.1", 8080, 50*time.Millisecond); err == nil {
		t.Fatalf("expected error")
	}
}

type fakeTicker struct {
	ch chan time.Time
}

func newFakeTicker() *fakeTicker          { return &fakeTicker{ch: make(chan time.Time, 32)} }
func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop()               {}

type scriptedChecker struct {
	mu     sync.Mutex
	script map[BackendKey][]error
	seen   chan BackendKey
}

func (c *scriptedChecker) Check(address string, port int, timeout time.Duration) error {
	key := BackendKey{Service: "svc", Backend: address}
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.script[key]
	var err error
	if len(list) > 0 {
		err = list[0]
		c.script[key] = list[1:]
	}
	c.seen <- key
	return err
}

type recordingObserver struct {
	mu      sync.Mutex
	states  []StateChange
	weights []WeightChange
}

func (o *recordingObserver) OnStateChange(change StateChange) {
	o.mu.Lock()
	o.states = append(o.states, change)
	o.mu.Unlock()
}

func (o *recordingObserver) OnWeightChange(change WeightChange) {
	o.mu.Lock()
	o.weights = append(o.weights, change)
	o.mu.Unlock()
}

func TestHealthStateMachineTransitions(t *testing.T) {
	ticker := newFakeTicker()

	checker := &scriptedChecker{
		script: map[BackendKey][]error{
			{Service: "svc", Backend: "10.0.0.1"}: {
				errors.New("fail"), // UNKNOWN -> UNHEALTHY
				nil,                // UNHEALTHY (1 success)
				nil,                // UNHEALTHY -> HEALTHY (recover_after=2)
				errors.New("fail"), // HEALTHY (1 fail)
				errors.New("fail"), // HEALTHY -> UNHEALTHY (fail_after=2)
			},
		},
		seen: make(chan BackendKey, 32),
	}
	obs := &recordingObserver{}

	s := NewScheduler(checker, obs)
	s.SetTickerFactory(func(d time.Duration) Ticker { return ticker })
	t.Cleanup(s.Stop)

	if err := s.Start([]Target{
		{
			Key:              BackendKey{Service: "svc", Backend: "10.0.0.1"},
			CheckPort:        8080,
			Interval:         10 * time.Millisecond,
			Timeout:          5 * time.Millisecond,
			FailAfter:        2,
			RecoverAfter:     2,
			ConfiguredWeight: 5,
		},
	}); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	drive := func() {
		ticker.ch <- time.Now()
		<-checker.seen
	}

	drive() // fail -> UNHEALTHY
	drive() // success
	drive() // success -> HEALTHY
	drive() // fail
	drive() // fail -> UNHEALTHY

	obs.mu.Lock()
	defer obs.mu.Unlock()

	if len(obs.states) != 3 {
		t.Fatalf("expected 3 state changes, got %d: %#v", len(obs.states), obs.states)
	}
	if obs.states[0].Old != StateUnknown || obs.states[0].New != StateUnhealthy {
		t.Fatalf("unexpected first transition: %#v", obs.states[0])
	}
	if obs.states[1].Old != StateUnhealthy || obs.states[1].New != StateHealthy {
		t.Fatalf("unexpected second transition: %#v", obs.states[1])
	}
	if obs.states[2].Old != StateHealthy || obs.states[2].New != StateUnhealthy {
		t.Fatalf("unexpected third transition: %#v", obs.states[2])
	}

	if len(obs.weights) != 3 {
		t.Fatalf("expected 3 weight changes, got %d: %#v", len(obs.weights), obs.weights)
	}
	if obs.weights[0].NewWeight != 0 {
		t.Fatalf("expected first weight 0, got %#v", obs.weights[0])
	}
	if obs.weights[1].NewWeight != 5 {
		t.Fatalf("expected second weight 5, got %#v", obs.weights[1])
	}
	if obs.weights[2].NewWeight != 0 {
		t.Fatalf("expected third weight 0, got %#v", obs.weights[2])
	}
}
