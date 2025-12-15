package health

import (
	"fmt"
	"sync"
	"time"
)

type State string

const (
	StateUnknown   State = "UNKNOWN"
	StateHealthy   State = "HEALTHY"
	StateUnhealthy State = "UNHEALTHY"
)

type BackendKey struct {
	Service string
	Backend string
}

type Target struct {
	Key              BackendKey
	CheckPort        int
	Interval         time.Duration
	Timeout          time.Duration
	FailAfter        int
	RecoverAfter     int
	ConfiguredWeight int
}

type StateChange struct {
	Key BackendKey
	Old State
	New State
}

type WeightChange struct {
	Key       BackendKey
	OldWeight int
	NewWeight int
	Reason    string
}

type Observer interface {
	OnStateChange(change StateChange)
	OnWeightChange(change WeightChange)
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct {
	t *time.Ticker
}

func (rt realTicker) C() <-chan time.Time { return rt.t.C }
func (rt realTicker) Stop()               { rt.t.Stop() }

type tickerFactory func(d time.Duration) Ticker

type Scheduler struct {
	checker Checker
	obs     Observer

	mu      sync.Mutex
	runners map[BackendKey]*runner
	tickers tickerFactory
	stopped bool
}

type runner struct {
	target Target
	
	mu                   sync.Mutex // Protects state fields below
	state                State
	consecutiveSuccesses int
	consecutiveFailures  int
	effectiveWeight      int

	stopCh chan struct{}
	doneCh chan struct{}
	ticker Ticker
}

func NewScheduler(checker Checker, observer Observer) *Scheduler {
	return &Scheduler{
		checker: checker,
		obs:     observer,
		runners: make(map[BackendKey]*runner),
		tickers: func(d time.Duration) Ticker { return realTicker{t: time.NewTicker(d)} },
	}
}

func (s *Scheduler) SetTickerFactory(factory func(d time.Duration) Ticker) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tickers = factory
}

func (s *Scheduler) Start(targets []Target) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return fmt.Errorf("scheduler stopped")
	}
	if s.checker == nil {
		return fmt.Errorf("missing checker")
	}
	for _, t := range targets {
		if err := validateTarget(t); err != nil {
			return err
		}
		if _, exists := s.runners[t.Key]; exists {
			return fmt.Errorf("duplicate target: %s/%s", t.Key.Service, t.Key.Backend)
		}

		r := &runner{
			target:          t,
			state:           StateUnknown,
			effectiveWeight: -1,
			stopCh:          make(chan struct{}),
			doneCh:          make(chan struct{}),
		}
		r.ticker = s.tickers(t.Interval)
		s.runners[t.Key] = r
		go s.run(r)
	}
	return nil
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	runners := make([]*runner, 0, len(s.runners))
	for _, r := range s.runners {
		runners = append(runners, r)
	}
	s.mu.Unlock()

	for _, r := range runners {
		close(r.stopCh)
		if r.ticker != nil {
			r.ticker.Stop()
		}
		<-r.doneCh
	}
}

func validateTarget(t Target) error {
	if t.Key.Service == "" {
		return fmt.Errorf("missing service name")
	}
	if t.Key.Backend == "" {
		return fmt.Errorf("missing backend address")
	}
	if t.CheckPort < 1 || t.CheckPort > 65535 {
		return fmt.Errorf("invalid check port: %d", t.CheckPort)
	}
	if t.Interval <= 0 {
		return fmt.Errorf("invalid interval: %s", t.Interval)
	}
	if t.Timeout <= 0 {
		return fmt.Errorf("invalid timeout: %s", t.Timeout)
	}
	if t.FailAfter < 1 {
		return fmt.Errorf("invalid fail_after: %d", t.FailAfter)
	}
	if t.RecoverAfter < 1 {
		return fmt.Errorf("invalid recover_after: %d", t.RecoverAfter)
	}
	return nil
}

func (s *Scheduler) run(r *runner) {
	defer close(r.doneCh)
	for {
		select {
		case <-r.stopCh:
			return
		case <-r.ticker.C():
			s.tick(r)
		}
	}
}

func (s *Scheduler) tick(r *runner) {
	// Perform health check without holding lock (I/O operation)
	err := s.checker.Check(r.target.Key.Backend, r.target.CheckPort, r.target.Timeout)
	success := err == nil

	// Lock for all state modifications
	r.mu.Lock()
	oldState := r.state
	oldWeight := r.effectiveWeight

	if success {
		r.consecutiveSuccesses++
		r.consecutiveFailures = 0
		switch r.state {
		case StateUnknown:
			r.state = StateHealthy
		case StateUnhealthy:
			if r.consecutiveSuccesses >= r.target.RecoverAfter {
				r.state = StateHealthy
			}
		}
	} else {
		r.consecutiveFailures++
		r.consecutiveSuccesses = 0
		switch r.state {
		case StateUnknown:
			r.state = StateUnhealthy
		case StateHealthy:
			if r.consecutiveFailures >= r.target.FailAfter {
				r.state = StateUnhealthy
			}
		}
	}

	if r.state == StateHealthy {
		r.effectiveWeight = r.target.ConfiguredWeight
	} else if r.state == StateUnhealthy {
		r.effectiveWeight = 0
	}

	// Capture state changes before unlocking
	stateChanged := oldState != r.state
	weightChanged := oldWeight != r.effectiveWeight
	newState := r.state
	newWeight := r.effectiveWeight
	r.mu.Unlock()

	// Call observers after releasing lock (to avoid holding lock during callbacks)
	if stateChanged && s.obs != nil {
		s.obs.OnStateChange(StateChange{Key: r.target.Key, Old: oldState, New: newState})
	}
	if weightChanged && s.obs != nil {
		s.obs.OnWeightChange(WeightChange{
			Key:       r.target.Key,
			OldWeight: oldWeight,
			NewWeight: newWeight,
			Reason:    "health",
		})
	}
}
