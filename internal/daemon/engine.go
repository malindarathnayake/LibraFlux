package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/malindarathnayake/LibraFlux/internal/config"
	"github.com/malindarathnayake/LibraFlux/internal/health"
	"github.com/malindarathnayake/LibraFlux/internal/observability"
	"github.com/malindarathnayake/LibraFlux/internal/system"
)

type IPVSReconciler interface {
	Apply(desired []config.Service, vip string) error
}

type Ticker interface {
	C() <-chan time.Time
	Stop()
}

type realTicker struct{ t *time.Ticker }

func (rt realTicker) C() <-chan time.Time { return rt.t.C }
func (rt realTicker) Stop()               { rt.t.Stop() }

type EngineOptions struct {
	ConfigPath string

	Logger  *observability.Logger
	Auditor *observability.Auditor
	Metrics *observability.MetricsRegistry

	Network    system.NetworkManager
	Reconciler IPVSReconciler

	ReloadCh <-chan struct{}

	VIPCheckInterval time.Duration
	NewTicker        func(d time.Duration) Ticker

	LoadConfig     func(path string) (*config.Config, error)
	ValidateConfig func(cfg *config.Config) error

	Checker      health.Checker
	NewScheduler func(checker health.Checker, observer health.Observer) *health.Scheduler
}

type Engine struct {
	configPath string

	logger  *observability.Logger
	auditor *observability.Auditor
	metrics *observability.MetricsRegistry

	network    system.NetworkManager
	reconciler IPVSReconciler

	reloadCh <-chan struct{}

	vipCheckInterval time.Duration
	newTicker        func(d time.Duration) Ticker

	loadConfig     func(path string) (*config.Config, error)
	validateConfig func(cfg *config.Config) error

	checker      health.Checker
	newScheduler func(checker health.Checker, observer health.Observer) *health.Scheduler

	mu                 sync.Mutex
	cfg                *config.Config
	cfgHash            string
	active             bool
	pendingReconcile   bool
	pendingDisable     bool
	backendWeights     map[health.BackendKey]int
	scheduler          *health.Scheduler
	reconcileAttempts  int       // Tracks consecutive reconcile failures
	nextReconcileRetry time.Time // When next retry is allowed

	reconcileReqCh chan struct{}
}

func NewEngine(opts EngineOptions) (*Engine, error) {
	if opts.ConfigPath == "" {
		return nil, fmt.Errorf("missing config path")
	}
	if opts.Network == nil {
		return nil, fmt.Errorf("missing network manager")
	}
	if opts.Reconciler == nil {
		return nil, fmt.Errorf("missing reconciler")
	}

	logger := opts.Logger
	if logger == nil {
		logger = observability.NewLogger(observability.InfoLevel)
	}

	auditor := opts.Auditor
	if auditor == nil {
		auditor = observability.NewAuditor(logger).WithComponent("daemon")
	}

	metrics := opts.Metrics
	if metrics == nil {
		metrics = observability.NewMetricsRegistry()
	}

	vipInterval := opts.VIPCheckInterval
	if vipInterval <= 0 {
		vipInterval = time.Second
	}

	newTicker := opts.NewTicker
	if newTicker == nil {
		newTicker = func(d time.Duration) Ticker { return realTicker{t: time.NewTicker(d)} }
	}

	loadConfig := opts.LoadConfig
	if loadConfig == nil {
		loadConfig = config.LoadConfig
	}
	validateConfig := opts.ValidateConfig
	if validateConfig == nil {
		validateConfig = config.Validate
	}

	checker := opts.Checker
	if checker == nil {
		checker = &health.TCPChecker{Dialer: health.NetDialer{}}
	}
	newScheduler := opts.NewScheduler
	if newScheduler == nil {
		newScheduler = func(c health.Checker, o health.Observer) *health.Scheduler {
			return health.NewScheduler(c, o)
		}
	}

	e := &Engine{
		configPath:       opts.ConfigPath,
		logger:           logger,
		auditor:          auditor,
		metrics:          metrics,
		network:          opts.Network,
		reconciler:       opts.Reconciler,
		reloadCh:         opts.ReloadCh,
		vipCheckInterval: vipInterval,
		newTicker:        newTicker,
		loadConfig:       loadConfig,
		validateConfig:   validateConfig,
		checker:          checker,
		newScheduler:     newScheduler,
		backendWeights:   make(map[health.BackendKey]int),
		reconcileReqCh:   make(chan struct{}, 1),
	}

	e.initMetrics()
	return e, nil
}

func (e *Engine) initMetrics() {
	e.metrics.NewGauge("lbctl_vip_is_owner", "1 if this node owns the VIP", []string{"node", "vip"})
	e.metrics.NewCounter("lbctl_vip_transitions_total", "VIP ownership transitions", []string{"node", "vip", "direction"})
	e.metrics.NewCounter("lbctl_reconcile_runs_total", "Reconcile attempts", []string{"node", "result"})
	e.metrics.NewGauge("lbctl_reconcile_duration_ms", "Last reconcile duration in ms", []string{"node"})
	e.metrics.NewGauge("lbctl_health_backend_healthy", "1 if backend is healthy", []string{"node", "service", "backend"})
	e.metrics.NewGauge("lbctl_health_backend_weight", "Effective backend weight", []string{"node", "service", "backend"})
}

func (e *Engine) Run(ctx context.Context) error {
	if err := e.loadAndSetConfig(true); err != nil {
		return err
	}

	if err := e.startHealthScheduler(); err != nil {
		return err
	}
	defer e.stopHealthScheduler()

	if err := e.initialVIPSync(ctx); err != nil {
		e.logger.Warn("Initial VIP sync failed", map[string]interface{}{"error": err.Error()})
	}

	tickInterval := e.vipCheckIntervalFromConfig()
	ticker := e.newTicker(tickInterval)
	defer func() { ticker.Stop() }()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C():
			e.onVIPTick(ctx)
		case <-e.reconcileReqCh:
			e.tryReconcile(ctx)
		case <-e.reloadCh:
			e.onReload(ctx)
			nextInterval := e.vipCheckIntervalFromConfig()
			if nextInterval != tickInterval {
				ticker.Stop()
				ticker = e.newTicker(nextInterval)
				tickInterval = nextInterval
			}
		}
	}
}

func (e *Engine) vipCheckIntervalFromConfig() time.Duration {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()

	if cfg != nil && cfg.Daemon.ReconcileIntervalMS > 0 {
		return time.Duration(cfg.Daemon.ReconcileIntervalMS) * time.Millisecond
	}
	return e.vipCheckInterval
}

func (e *Engine) loadAndSetConfig(isStartup bool) error {
	cfg, err := e.loadConfig(e.configPath)
	if err != nil {
		return err
	}
	if err := e.validateConfig(cfg); err != nil {
		return err
	}

	hash, err := hashConfig(cfg)
	if err != nil {
		return err
	}

	e.mu.Lock()
	oldHash := e.cfgHash
	e.cfg = cfg
	e.cfgHash = hash
	e.backendWeights = make(map[health.BackendKey]int)
	e.mu.Unlock()

	e.logger.SetNodeConfig(cfg.Node.Name, map[string]interface{}{
		"role": cfg.Node.Role,
	})

	e.auditor.Emit(observability.AuditConfigLoaded, map[string]interface{}{
		"config_hash":    hash,
		"services_count": len(cfg.Services),
		"backends_count": countBackends(cfg.Services),
		"startup":        isStartup,
	})
	if oldHash != "" && oldHash != hash {
		e.auditor.Emit(observability.AuditConfigChanged, map[string]interface{}{
			"old_hash": oldHash,
			"new_hash": hash,
		})
	}

	return nil
}

func (e *Engine) initialVIPSync(ctx context.Context) error {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("missing config")
	}

	present, err := e.network.CheckVIPPresent(cfg.Network.Frontend.VIP)
	if err != nil {
		return err
	}

	e.mu.Lock()
	e.active = present
	e.pendingReconcile = present
	e.pendingDisable = false
	e.mu.Unlock()

	e.updateVIPGauge(cfg, present)

	if present {
		e.logger.Info("VIP present at startup; starting active", map[string]interface{}{"vip": cfg.Network.Frontend.VIP})
		e.tryReconcile(ctx)
	} else {
		e.logger.Info("VIP not present at startup; starting standby", map[string]interface{}{"vip": cfg.Network.Frontend.VIP})
	}
	return nil
}

func (e *Engine) onVIPTick(ctx context.Context) {
	e.mu.Lock()
	cfg := e.cfg
	wasActive := e.active
	e.mu.Unlock()
	if cfg == nil {
		return
	}

	present, err := e.network.CheckVIPPresent(cfg.Network.Frontend.VIP)
	if err != nil {
		e.logger.Warn("VIP check failed", map[string]interface{}{
			"vip":   cfg.Network.Frontend.VIP,
			"error": err.Error(),
		})
		return
	}

	switch {
	case present && !wasActive:
		e.onVIPAcquired(ctx, cfg)
	case !present && wasActive:
		e.onVIPReleased(ctx, cfg)
	default:
		e.updateVIPGauge(cfg, present)
	}

	if present {
		e.tryReconcile(ctx)
	} else {
		e.tryDisable(ctx)
	}
}

func (e *Engine) onVIPAcquired(ctx context.Context, cfg *config.Config) {
	e.logger.Info("VIP acquired; becoming active", map[string]interface{}{"vip": cfg.Network.Frontend.VIP})
	e.auditor.Emit(observability.AuditVIPAcquired, map[string]interface{}{"vip": cfg.Network.Frontend.VIP})

	e.mu.Lock()
	e.active = true
	e.pendingDisable = false
	e.pendingReconcile = true
	e.mu.Unlock()

	e.metrics.Counter("lbctl_vip_transitions_total", prometheus.Labels{
		"node":      cfg.Node.Name,
		"vip":       cfg.Network.Frontend.VIP,
		"direction": "acquire",
	}).Inc()

	e.updateVIPGauge(cfg, true)
	e.tryReconcile(ctx)
}

func (e *Engine) onVIPReleased(ctx context.Context, cfg *config.Config) {
	e.logger.Info("VIP released; becoming standby", map[string]interface{}{"vip": cfg.Network.Frontend.VIP})
	e.auditor.Emit(observability.AuditVIPReleased, map[string]interface{}{"vip": cfg.Network.Frontend.VIP})

	e.mu.Lock()
	e.active = false
	e.pendingReconcile = false
	e.pendingDisable = true
	e.mu.Unlock()

	e.metrics.Counter("lbctl_vip_transitions_total", prometheus.Labels{
		"node":      cfg.Node.Name,
		"vip":       cfg.Network.Frontend.VIP,
		"direction": "release",
	}).Inc()

	e.updateVIPGauge(cfg, false)
	e.tryDisable(ctx)
}

func (e *Engine) updateVIPGauge(cfg *config.Config, present bool) {
	val := 0.0
	if present {
		val = 1.0
	}
	e.metrics.Gauge("lbctl_vip_is_owner", prometheus.Labels{
		"node": cfg.Node.Name,
		"vip":  cfg.Network.Frontend.VIP,
	}).Set(val)
}

func (e *Engine) onReload(ctx context.Context) {
	e.logger.Info("Reload requested (SIGHUP)", nil)

	// Load and validate new config FIRST - don't stop scheduler until we know new config is valid
	if err := e.loadAndSetConfig(false); err != nil {
		e.logger.Error("Config reload failed; keeping previous config and health scheduler", map[string]interface{}{"error": err.Error()})
		return
	}

	// Config is valid - now safe to stop old scheduler and start new one
	e.stopHealthScheduler()

	if err := e.startHealthScheduler(); err != nil {
		e.logger.Error("Failed to restart health scheduler after reload", map[string]interface{}{"error": err.Error()})
	}

	e.mu.Lock()
	active := e.active
	e.pendingReconcile = true
	e.mu.Unlock()

	if active {
		e.tryReconcile(ctx)
	}
}

func (e *Engine) tryReconcile(ctx context.Context) {
	e.mu.Lock()
	cfg := e.cfg
	active := e.active
	pending := e.pendingReconcile
	
	// Check backoff timing - skip if we're in backoff period
	if !time.Now().After(e.nextReconcileRetry) {
		e.mu.Unlock()
		return
	}
	
	weights := make(map[health.BackendKey]int, len(e.backendWeights))
	for k, v := range e.backendWeights {
		weights[k] = v
	}
	attempts := e.reconcileAttempts
	e.mu.Unlock()

	if cfg == nil || !active || !pending {
		return
	}

	desired := applyEffectiveWeights(cfg.Services, weights)
	start := time.Now()
	err := e.reconciler.Apply(desired, cfg.Network.Frontend.VIP)
	durationMS := float64(time.Since(start).Milliseconds())
	e.metrics.Gauge("lbctl_reconcile_duration_ms", prometheus.Labels{"node": cfg.Node.Name}).Set(durationMS)

	if err != nil {
		e.metrics.Counter("lbctl_reconcile_runs_total", prometheus.Labels{"node": cfg.Node.Name, "result": "failure"}).Inc()
		
		// Calculate backoff with jitter
		backoff := calculateBackoff(attempts + 1)
		e.mu.Lock()
		e.pendingReconcile = true
		e.reconcileAttempts++
		e.nextReconcileRetry = time.Now().Add(backoff)
		e.mu.Unlock()
		
		e.logger.Error("Reconcile failed", map[string]interface{}{
			"error":    err.Error(),
			"attempts": attempts + 1,
			"backoff":  backoff.String(),
		})
		return
	}

	// Success - reset retry state
	e.metrics.Counter("lbctl_reconcile_runs_total", prometheus.Labels{"node": cfg.Node.Name, "result": "success"}).Inc()
	e.mu.Lock()
	e.pendingReconcile = false
	e.reconcileAttempts = 0
	e.nextReconcileRetry = time.Time{}
	e.mu.Unlock()
}

func (e *Engine) tryDisable(ctx context.Context) {
	e.mu.Lock()
	cfg := e.cfg
	active := e.active
	pending := e.pendingDisable
	e.mu.Unlock()

	if cfg == nil || active || !pending {
		return
	}

	start := time.Now()
	err := e.reconciler.Apply(nil, cfg.Network.Frontend.VIP)
	durationMS := float64(time.Since(start).Milliseconds())
	e.metrics.Gauge("lbctl_reconcile_duration_ms", prometheus.Labels{"node": cfg.Node.Name}).Set(durationMS)

	if err != nil {
		e.metrics.Counter("lbctl_reconcile_runs_total", prometheus.Labels{"node": cfg.Node.Name, "result": "failure"}).Inc()
		e.logger.Error("Disable failed", map[string]interface{}{"error": err.Error()})
		e.mu.Lock()
		e.pendingDisable = true
		e.mu.Unlock()
		return
	}

	e.metrics.Counter("lbctl_reconcile_runs_total", prometheus.Labels{"node": cfg.Node.Name, "result": "success"}).Inc()
	e.mu.Lock()
	e.pendingDisable = false
	e.mu.Unlock()
}

func (e *Engine) requestReconcile() {
	select {
	case e.reconcileReqCh <- struct{}{}:
	default:
	}
}

func (e *Engine) startHealthScheduler() error {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if cfg == nil {
		return fmt.Errorf("missing config")
	}

	e.stopHealthScheduler()

	targets := healthTargets(cfg.Services)
	if len(targets) == 0 {
		return nil
	}

	s := e.newScheduler(e.checker, e)
	if err := s.Start(targets); err != nil {
		return err
	}

	e.mu.Lock()
	e.scheduler = s
	e.mu.Unlock()

	return nil
}

func (e *Engine) stopHealthScheduler() {
	e.mu.Lock()
	s := e.scheduler
	e.scheduler = nil
	e.mu.Unlock()

	if s != nil {
		s.Stop()
	}
}

func (e *Engine) OnStateChange(change health.StateChange) {
	e.mu.Lock()
	cfg := e.cfg
	e.mu.Unlock()
	if cfg == nil {
		return
	}

	val := 0.0
	if change.New == health.StateHealthy {
		val = 1.0
	}

	e.metrics.Gauge("lbctl_health_backend_healthy", prometheus.Labels{
		"node":    cfg.Node.Name,
		"service": change.Key.Service,
		"backend": change.Key.Backend,
	}).Set(val)

	e.auditor.Emit(observability.AuditHealthStateChanged, map[string]interface{}{
		"service_name": change.Key.Service,
		"backend":      change.Key.Backend,
		"old_state":    string(change.Old),
		"new_state":    string(change.New),
	})
}

func (e *Engine) OnWeightChange(change health.WeightChange) {
	e.mu.Lock()
	cfg := e.cfg
	if cfg == nil {
		e.mu.Unlock()
		return
	}
	e.backendWeights[change.Key] = change.NewWeight
	e.pendingReconcile = true
	active := e.active
	e.mu.Unlock()

	e.metrics.Gauge("lbctl_health_backend_weight", prometheus.Labels{
		"node":    cfg.Node.Name,
		"service": change.Key.Service,
		"backend": change.Key.Backend,
	}).Set(float64(change.NewWeight))

	e.auditor.Emit(observability.AuditBackendWeightChanged, map[string]interface{}{
		"service_name": change.Key.Service,
		"backend":      change.Key.Backend,
		"old_weight":   change.OldWeight,
		"new_weight":   change.NewWeight,
		"reason":       change.Reason,
	})

	if active {
		e.requestReconcile()
	}
}

func hashConfig(cfg *config.Config) (string, error) {
	b, err := json.Marshal(cfg)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func countBackends(services []config.Service) int {
	total := 0
	for _, svc := range services {
		total += len(svc.Backends)
	}
	return total
}

func healthTargets(services []config.Service) []health.Target {
	var targets []health.Target
	for _, svc := range services {
		if !svc.Health.Enabled {
			continue
		}
		for _, be := range svc.Backends {
			targets = append(targets, health.Target{
				Key: health.BackendKey{
					Service: svc.Name,
					Backend: be.Address,
				},
				CheckPort:        svc.Health.Port,
				Interval:         time.Duration(svc.Health.IntervalMS) * time.Millisecond,
				Timeout:          time.Duration(svc.Health.TimeoutMS) * time.Millisecond,
				FailAfter:        svc.Health.FailAfter,
				RecoverAfter:     svc.Health.RecoverAfter,
				ConfiguredWeight: be.Weight,
			})
		}
	}
	return targets
}

// calculateBackoff returns exponential backoff with jitter
// Attempt 1: 0s (immediate)
// Attempt 2: 5s + jitter (0-1s)
// Attempt 3+: 10s + jitter (0-2s)
func calculateBackoff(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}
	
	var base time.Duration
	var jitter time.Duration
	
	if attempt == 2 {
		base = 5 * time.Second
		jitter = time.Duration(time.Now().UnixNano()%1000) * time.Millisecond
	} else {
		base = 10 * time.Second
		jitter = time.Duration(time.Now().UnixNano()%2000) * time.Millisecond
	}
	
	return base + jitter
}

func applyEffectiveWeights(services []config.Service, weights map[health.BackendKey]int) []config.Service {
	copied := make([]config.Service, len(services))
	for i, svc := range services {
		copied[i] = svc
		if len(svc.Backends) == 0 {
			continue
		}

		backends := make([]config.Backend, len(svc.Backends))
		copy(backends, svc.Backends)

		for j := range backends {
			key := health.BackendKey{Service: svc.Name, Backend: backends[j].Address}
			if w, ok := weights[key]; ok && w >= 0 {
				backends[j].Weight = w
			}
		}
		copied[i].Backends = backends
	}
	return copied
}
