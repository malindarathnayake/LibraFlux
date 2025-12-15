package daemon

import (
	"context"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/config"
	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

type fakeTicker struct {
	ch chan time.Time
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop()               {}

type fakeNetworkManager struct {
	mu      sync.Mutex
	present bool
}

func (f *fakeNetworkManager) setPresent(p bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.present = p
}

func (f *fakeNetworkManager) CheckVIPPresent(_ string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.present, nil
}

func (f *fakeNetworkManager) GetInterfaceStatus(_ string) (bool, error) {
	return true, nil
}

type applyCall struct {
	vip          string
	serviceCount int
}

type fakeReconciler struct {
	mu    sync.Mutex
	calls []applyCall
}

func (r *fakeReconciler) Apply(desired []config.Service, vip string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, applyCall{
		vip:          vip,
		serviceCount: len(desired),
	})
	return nil
}

func (r *fakeReconciler) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *fakeReconciler) lastCall() (applyCall, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.calls) == 0 {
		return applyCall{}, false
	}
	return r.calls[len(r.calls)-1], true
}

func eventually(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
	t.Fatalf("condition not met within %s", timeout)
}

func TestEngine_VIPTransitions_ApplyAndDisable(t *testing.T) {
	net := &fakeNetworkManager{}
	rec := &fakeReconciler{}
	reloadCh := make(chan struct{}, 1)

	ticker := &fakeTicker{ch: make(chan time.Time, 10)}

	cfg1 := &config.Config{
		Node: config.NodeConfig{Name: "node-a", Role: "primary"},
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "ens160", VIP: "192.0.2.10", CIDR: 32},
			Backend:  config.InterfaceConfig{Interface: "ens192"},
		},
		VRRP: config.VRRPConfig{VRID: 1, PriorityPrimary: 100, PrioritySecondary: 90, AdvertIntervalMS: 1000},
		Services: []config.Service{
			{
				Name:      "svc1",
				Protocol:  "tcp",
				Ports:     []int{80},
				Scheduler: "rr",
				Backends: []config.Backend{
					{Address: "192.0.2.20", Port: 0, Weight: 1},
				},
			},
		},
	}

	engine, err := NewEngine(EngineOptions{
		ConfigPath:     "ignored",
		Logger:         observability.NewLogger(observability.ErrorLevel),
		Network:        net,
		Reconciler:     rec,
		ReloadCh:       reloadCh,
		NewTicker:      func(time.Duration) Ticker { return ticker },
		LoadConfig:     func(string) (*config.Config, error) { return cfg1, nil },
		ValidateConfig: func(*config.Config) error { return nil },
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- engine.Run(ctx) }()

	net.setPresent(false)
	ticker.ch <- time.Now()
	time.Sleep(5 * time.Millisecond)
	if rec.callCount() != 0 {
		t.Fatalf("expected no reconcile while standby, got %d", rec.callCount())
	}

	net.setPresent(true)
	ticker.ch <- time.Now()
	eventually(t, 200*time.Millisecond, func() bool { return rec.callCount() >= 1 })
	last, _ := rec.lastCall()
	if last.vip != "192.0.2.10" || last.serviceCount != 1 {
		t.Fatalf("unexpected apply call: %+v", last)
	}

	net.setPresent(false)
	ticker.ch <- time.Now()
	eventually(t, 200*time.Millisecond, func() bool {
		c, ok := rec.lastCall()
		return ok && c.serviceCount == 0
	})

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("engine returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("engine did not exit")
	}
}

func TestEngine_ReloadWhileActive_Reconciles(t *testing.T) {
	net := &fakeNetworkManager{}
	net.setPresent(true)

	rec := &fakeReconciler{}
	reloadCh := make(chan struct{}, 1)
	ticker := &fakeTicker{ch: make(chan time.Time, 10)}

	cfg1 := &config.Config{
		Node: config.NodeConfig{Name: "node-a", Role: "primary"},
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "ens160", VIP: "192.0.2.10", CIDR: 32},
			Backend:  config.InterfaceConfig{Interface: "ens192"},
		},
		VRRP: config.VRRPConfig{VRID: 1, PriorityPrimary: 100, PrioritySecondary: 90, AdvertIntervalMS: 1000},
		Services: []config.Service{
			{Name: "svc1", Protocol: "tcp", Ports: []int{80}, Scheduler: "rr", Backends: []config.Backend{{Address: "192.0.2.20", Weight: 1}}},
		},
	}
	cfg2 := &config.Config{
		Node:    cfg1.Node,
		Network: cfg1.Network,
		VRRP:    cfg1.VRRP,
		Services: []config.Service{
			{Name: "svc1", Protocol: "tcp", Ports: []int{80}, Scheduler: "rr", Backends: []config.Backend{{Address: "192.0.2.20", Weight: 1}}},
			{Name: "svc2", Protocol: "tcp", Ports: []int{443}, Scheduler: "rr", Backends: []config.Backend{{Address: "192.0.2.21", Weight: 1}}},
		},
	}

	var loadMu sync.Mutex
	loadCount := 0
	loadFn := func(string) (*config.Config, error) {
		loadMu.Lock()
		defer loadMu.Unlock()
		loadCount++
		if loadCount == 1 {
			return cfg1, nil
		}
		return cfg2, nil
	}

	engine, err := NewEngine(EngineOptions{
		ConfigPath:     "ignored",
		Logger:         observability.NewLogger(observability.ErrorLevel),
		Network:        net,
		Reconciler:     rec,
		ReloadCh:       reloadCh,
		NewTicker:      func(time.Duration) Ticker { return ticker },
		LoadConfig:     loadFn,
		ValidateConfig: func(*config.Config) error { return nil },
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- engine.Run(ctx) }()

	ticker.ch <- time.Now()
	eventually(t, 200*time.Millisecond, func() bool {
		last, ok := rec.lastCall()
		return ok && last.serviceCount == 1
	})

	reloadCh <- struct{}{}
	eventually(t, 200*time.Millisecond, func() bool {
		last, ok := rec.lastCall()
		return ok && last.serviceCount == 2
	})

	cancel()
	select {
	case <-errCh:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("engine did not exit")
	}
}

func TestEngine_UsesConfigReconcileIntervalForTicker(t *testing.T) {
	net := &fakeNetworkManager{}
	rec := &fakeReconciler{}
	reloadCh := make(chan struct{}, 1)

	ticker := &fakeTicker{ch: make(chan time.Time, 1)}
	intervalCh := make(chan time.Duration, 1)

	cfg := &config.Config{
		Node: config.NodeConfig{Name: "node-a", Role: "primary"},
		Network: config.NetworkConfig{
			Frontend: config.InterfaceConfig{Interface: "ens160", VIP: "192.0.2.10", CIDR: 32},
			Backend:  config.InterfaceConfig{Interface: "ens192"},
		},
		VRRP: config.VRRPConfig{VRID: 1, PriorityPrimary: 100, PrioritySecondary: 90, AdvertIntervalMS: 1000},
		Daemon: config.DaemonConfig{
			ReconcileIntervalMS: 2500,
		},
	}

	engine, err := NewEngine(EngineOptions{
		ConfigPath:     "ignored",
		Logger:         observability.NewLogger(observability.ErrorLevel),
		Network:        net,
		Reconciler:     rec,
		ReloadCh:       reloadCh,
		LoadConfig:     func(string) (*config.Config, error) { return cfg, nil },
		ValidateConfig: func(*config.Config) error { return nil },
		NewTicker: func(d time.Duration) Ticker {
			intervalCh <- d
			return ticker
		},
	})
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() { errCh <- engine.Run(ctx) }()

	select {
	case got := <-intervalCh:
		want := 2500 * time.Millisecond
		if got != want {
			t.Fatalf("expected ticker interval %s, got %s", want, got)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected NewTicker to be called")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("engine returned error: %v", err)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("engine did not exit")
	}
}

func TestContextWithSignals_ReloadAndCancel(t *testing.T) {
	origNotify := notifySignals
	origStop := stopSignals
	defer func() {
		notifySignals = origNotify
		stopSignals = origStop
	}()

	var captured chan<- os.Signal
	notifySignals = func(c chan<- os.Signal, _ ...os.Signal) { captured = c }
	stopSignals = func(chan<- os.Signal) {}

	ctx, reload, stop := ContextWithSignals(context.Background(), observability.NewLogger(observability.ErrorLevel))
	defer stop()

	if captured == nil {
		t.Fatalf("expected signal channel to be captured")
	}

	captured <- syscall.SIGHUP
	select {
	case <-reload:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected reload notification")
	}

	captured <- syscall.SIGTERM
	select {
	case <-ctx.Done():
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("expected context cancellation")
	}
}
