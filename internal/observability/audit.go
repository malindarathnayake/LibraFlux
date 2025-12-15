package observability

// AuditEvent is a stable, machine-parseable audit event name (per spec).
type AuditEvent string

const (
	AuditConfigLoaded         AuditEvent = "config_loaded"
	AuditConfigChanged        AuditEvent = "config_changed"
	AuditVIPAcquired          AuditEvent = "vip_acquired"
	AuditVIPReleased          AuditEvent = "vip_released"
	AuditServiceAdded         AuditEvent = "service_added"
	AuditServiceRemoved       AuditEvent = "service_removed"
	AuditBackendAdded         AuditEvent = "backend_added"
	AuditBackendRemoved       AuditEvent = "backend_removed"
	AuditBackendWeightChanged AuditEvent = "backend_weight_changed"
	AuditHealthStateChanged   AuditEvent = "health_state_changed"
	AuditFRRConfigPatched     AuditEvent = "frr_config_patched"
	AuditSysctlApplied        AuditEvent = "sysctl_applied"

	AuditLockAcquired  AuditEvent = "lock_acquired"
	AuditLockReleased  AuditEvent = "lock_released"
	AuditLockTimeout   AuditEvent = "lock_timeout"
	AuditLockRecovered AuditEvent = "lock_recovered"
	AuditLockBroken    AuditEvent = "lock_broken"
)

// Auditor handles recording of audit events
type Auditor struct {
	logger    *Logger
	component string
}

// NewAuditor creates a new auditor using the provided logger
func NewAuditor(logger *Logger) *Auditor {
	return &Auditor{
		logger: logger,
	}
}

// WithComponent returns a shallow copy of the auditor that tags emitted events with `_component`.
func (a *Auditor) WithComponent(component string) *Auditor {
	return &Auditor{
		logger:    a.logger,
		component: component,
	}
}

// Emit records an audit event via the structured logger.
func (a *Auditor) Emit(event AuditEvent, fields map[string]interface{}) {
	merged := make(map[string]interface{})
	for k, v := range fields {
		merged[k] = v
	}

	if a.component != "" {
		merged["_component"] = a.component
	}
	merged["_event_type"] = "audit"
	merged["_audit_event"] = string(event)

	a.logger.Info("AUDIT", merged)
}
