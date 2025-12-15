package config

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

var (
	// Regex for validation
	nameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

	// Injection characters check
	injectionChars = []string{";", "'", "\"", "`", "&", "|", ">", "<"}
)

// Validate checks the configuration for errors
func Validate(cfg *Config) error {
	if err := validateGlobal(cfg); err != nil {
		return err
	}

	if err := validateServices(cfg); err != nil {
		return err
	}

	return nil
}

func validateGlobal(cfg *Config) error {
	const (
		defaultReconcileIntervalMS = 1000
		minReconcileIntervalMS     = 100
		maxReconcileIntervalMS     = 60_000

		defaultStateCacheTTLMS = 500
		minStateCacheTTLMS     = 1
		maxStateCacheTTLMS     = 60_000
	)

	// Mode
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	if mode != "" && mode != "dr" && mode != "nat" {
		return fmt.Errorf("invalid mode: %s", cfg.Mode)
	}

	// Node
	if !isValidName(cfg.Node.Name) {
		return fmt.Errorf("invalid node name: %s", cfg.Node.Name)
	}
	if cfg.Node.Role != "primary" && cfg.Node.Role != "secondary" {
		return fmt.Errorf("invalid node role: %s", cfg.Node.Role)
	}

	// Network
	if !isValidName(cfg.Network.Frontend.Interface) {
		return fmt.Errorf("invalid frontend interface: %s", cfg.Network.Frontend.Interface)
	}
	if cfg.Network.Frontend.VIP == "" {
		return fmt.Errorf("frontend VIP is required")
	}
	if net.ParseIP(cfg.Network.Frontend.VIP) == nil {
		return fmt.Errorf("invalid frontend VIP: %s", cfg.Network.Frontend.VIP)
	}
	if cfg.Network.Frontend.CIDR < 1 || cfg.Network.Frontend.CIDR > 32 {
		return fmt.Errorf("invalid frontend CIDR: %d", cfg.Network.Frontend.CIDR)
	}
	if !isValidName(cfg.Network.Backend.Interface) {
		return fmt.Errorf("invalid backend interface: %s", cfg.Network.Backend.Interface)
	}

	// VRRP
	if cfg.VRRP.VRID < 1 || cfg.VRRP.VRID > 255 {
		return fmt.Errorf("invalid VRID: %d", cfg.VRRP.VRID)
	}
	if cfg.VRRP.PriorityPrimary < 1 || cfg.VRRP.PriorityPrimary > 255 {
		return fmt.Errorf("invalid priority_primary: %d", cfg.VRRP.PriorityPrimary)
	}
	if cfg.VRRP.PrioritySecondary < 1 || cfg.VRRP.PrioritySecondary > 255 {
		return fmt.Errorf("invalid priority_secondary: %d", cfg.VRRP.PrioritySecondary)
	}
	if cfg.VRRP.AdvertIntervalMS < 100 {
		return fmt.Errorf("invalid advert_interval_ms: %d", cfg.VRRP.AdvertIntervalMS)
	}

	// Observability - logging
	if cfg.Observability.Logging.Console.Level != "" {
		level := strings.ToLower(cfg.Observability.Logging.Console.Level)
		switch level {
		case "debug", "info", "warn", "error":
		default:
			return fmt.Errorf("invalid console log level: %s", cfg.Observability.Logging.Console.Level)
		}
	}
	if cfg.Observability.Logging.GELF.Enabled {
		if cfg.Observability.Logging.GELF.Host == "" {
			return fmt.Errorf("gelf.host is required when gelf.enabled is true")
		}
		if cfg.Observability.Logging.GELF.Port < 1 || cfg.Observability.Logging.GELF.Port > 65535 {
			return fmt.Errorf("invalid gelf.port: %d", cfg.Observability.Logging.GELF.Port)
		}
		proto := strings.ToLower(cfg.Observability.Logging.GELF.Protocol)
		if proto != "udp" && proto != "tcp" {
			return fmt.Errorf("invalid gelf.protocol: %s", cfg.Observability.Logging.GELF.Protocol)
		}
		if cfg.Observability.Logging.GELF.Facility == "" {
			return fmt.Errorf("gelf.facility is required when gelf.enabled is true")
		}
	}

	// Observability - metrics
	if cfg.Observability.Metrics.InfluxDB.Enabled {
		if cfg.Observability.Metrics.InfluxDB.URL == "" ||
			cfg.Observability.Metrics.InfluxDB.Token == "" ||
			cfg.Observability.Metrics.InfluxDB.Org == "" ||
			cfg.Observability.Metrics.InfluxDB.Bucket == "" {
			return fmt.Errorf("influxdb url/token/org/bucket are required when influxdb.enabled is true")
		}
		if cfg.Observability.Metrics.InfluxDB.PushIntervalSeconds < 1 {
			return fmt.Errorf("invalid influxdb.push_interval_seconds: %d", cfg.Observability.Metrics.InfluxDB.PushIntervalSeconds)
		}
	}
	if cfg.Observability.Metrics.Prometheus.Enabled {
		if cfg.Observability.Metrics.Prometheus.Port < 1 || cfg.Observability.Metrics.Prometheus.Port > 65535 {
			return fmt.Errorf("invalid prometheus.port: %d", cfg.Observability.Metrics.Prometheus.Port)
		}
		if cfg.Observability.Metrics.Prometheus.Path == "" || !strings.HasPrefix(cfg.Observability.Metrics.Prometheus.Path, "/") {
			return fmt.Errorf("invalid prometheus.path: %s", cfg.Observability.Metrics.Prometheus.Path)
		}
		// Validate bind address if provided
		bind := cfg.Observability.Metrics.Prometheus.Bind
		if bind != "" && bind != "0.0.0.0" && bind != "::" {
			if net.ParseIP(bind) == nil {
				return fmt.Errorf("invalid prometheus.bind: %s", bind)
			}
		}
	}

	// System
	if cfg.System.TuningProfile != "" {
		switch strings.ToLower(cfg.System.TuningProfile) {
		case "minimal", "balanced", "aggressive":
		default:
			return fmt.Errorf("invalid tuning_profile: %s", cfg.System.TuningProfile)
		}
	}
	if cfg.System.LockIdleTimeoutMinutes < 0 {
		return fmt.Errorf("invalid lock_idle_timeout_minutes: %d", cfg.System.LockIdleTimeoutMinutes)
	}

	// Daemon
	if cfg.Daemon.ReconcileIntervalMS == 0 {
		cfg.Daemon.ReconcileIntervalMS = defaultReconcileIntervalMS
	}
	if cfg.Daemon.ReconcileIntervalMS < minReconcileIntervalMS || cfg.Daemon.ReconcileIntervalMS > maxReconcileIntervalMS {
		return fmt.Errorf("invalid daemon.reconcile_interval_ms: %d", cfg.Daemon.ReconcileIntervalMS)
	}
	if cfg.Daemon.StateCache.TTLMS < 0 {
		return fmt.Errorf("invalid daemon.state_cache.ttl_ms: %d", cfg.Daemon.StateCache.TTLMS)
	}
	if cfg.Daemon.StateCache.Enabled {
		if cfg.Daemon.StateCache.TTLMS == 0 {
			cfg.Daemon.StateCache.TTLMS = defaultStateCacheTTLMS
		}
		if cfg.Daemon.StateCache.TTLMS < minStateCacheTTLMS || cfg.Daemon.StateCache.TTLMS > maxStateCacheTTLMS {
			return fmt.Errorf("invalid daemon.state_cache.ttl_ms: %d", cfg.Daemon.StateCache.TTLMS)
		}
	}

	return nil
}

func validateServices(cfg *Config) error {
	serviceNames := make(map[string]bool)

	for i, svc := range cfg.Services {
		// Name
		if !isValidName(svc.Name) {
			return fmt.Errorf("service[%d]: invalid name: %s", i, svc.Name)
		}
		if len(svc.Name) > 64 {
			return fmt.Errorf("service[%d]: name too long: %s", i, svc.Name)
		}
		if serviceNames[svc.Name] {
			return fmt.Errorf("duplicate service name: %s", svc.Name)
		}
		serviceNames[svc.Name] = true

		// Protocol
		proto := strings.ToLower(svc.Protocol)
		if proto != "tcp" && proto != "udp" {
			return fmt.Errorf("service %s: invalid protocol: %s", svc.Name, svc.Protocol)
		}

		// Scheduler
		sched := strings.ToLower(svc.Scheduler)
		validSchedulers := map[string]bool{"rr": true, "wrr": true, "sh": true}
		if !validSchedulers[sched] {
			return fmt.Errorf("service %s: invalid scheduler: %s", svc.Name, svc.Scheduler)
		}

		// Ports and Ranges
		if len(svc.Ports) == 0 && len(svc.PortRanges) == 0 {
			return fmt.Errorf("service %s: no ports defined", svc.Name)
		}
		for _, p := range svc.Ports {
			if p < 1 || p > 65535 {
				return fmt.Errorf("service %s: invalid port: %d", svc.Name, p)
			}
		}
		for _, pr := range svc.PortRanges {
			if pr.Start < 1 || pr.Start > 65535 || pr.End < 1 || pr.End > 65535 {
				return fmt.Errorf("service %s: invalid port range: %d-%d", svc.Name, pr.Start, pr.End)
			}
			if pr.Start > pr.End {
				return fmt.Errorf("service %s: invalid port range start > end: %d-%d", svc.Name, pr.Start, pr.End)
			}
		}

		// Backends
		for j, be := range svc.Backends {
			if net.ParseIP(be.Address) == nil {
				return fmt.Errorf("service %s backend[%d]: invalid address: %s", svc.Name, j, be.Address)
			}
			if be.Weight < 1 {
				return fmt.Errorf("service %s backend[%d]: invalid weight: %d", svc.Name, j, be.Weight)
			}
			// Port 0 is allowed (same as service port)
			if be.Port != 0 && (be.Port < 1 || be.Port > 65535) {
				return fmt.Errorf("service %s backend[%d]: invalid port: %d", svc.Name, j, be.Port)
			}
		}

		// Health Check
		if svc.Health.Enabled {
			if strings.ToLower(svc.Health.Type) != "tcp" {
				return fmt.Errorf("service %s: invalid health check type: %s", svc.Name, svc.Health.Type)
			}
			if svc.Health.Port < 1 || svc.Health.Port > 65535 {
				return fmt.Errorf("service %s: invalid health check port: %d", svc.Name, svc.Health.Port)
			}
			if svc.Health.IntervalMS < 100 {
				return fmt.Errorf("service %s: health interval too low: %d", svc.Name, svc.Health.IntervalMS)
			}
			if svc.Health.TimeoutMS < 100 {
				return fmt.Errorf("service %s: health timeout too low: %d", svc.Name, svc.Health.TimeoutMS)
			}
			if svc.Health.FailAfter < 1 {
				return fmt.Errorf("service %s: invalid health fail_after: %d", svc.Name, svc.Health.FailAfter)
			}
			if svc.Health.RecoverAfter < 1 {
				return fmt.Errorf("service %s: invalid health recover_after: %d", svc.Name, svc.Health.RecoverAfter)
			}
		}
	}

	return nil
}

func isValidName(s string) bool {
	if s == "" {
		return false
	}
	if !nameRegex.MatchString(s) {
		return false
	}
	return !containsInjectionChars(s)
}

func containsInjectionChars(s string) bool {
	for _, char := range injectionChars {
		if strings.Contains(s, char) {
			return true
		}
	}
	return false
}
