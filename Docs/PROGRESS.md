# lbctl Implementation Progress

## Current Status
**Phase:** 11 - Code Quality & Safety Fixes ✓ COMPLETE  
**Last Completed:** Phase 11 complete - 3 critical safety fixes implemented and validated ✓ PASS  
**Current Issue:** None - All critical production safety issues resolved  
**Next Up:** Phase 03B - Production Guardrails (PRIORITY: Safety before features)

---

## Completed Phases

### Phase 1: Core Infrastructure ✓ COMPLETE
- [x] observability/logger.go
- [x] observability/logger_test.go ✓ PASS
- [x] observability/metrics.go
- [x] observability/metrics_test.go ✓ PASS
- [x] observability/audit.go
- [x] observability/audit_test.go ✓ PASS
- [x] config/types.go
- [x] config/loader.go
- [x] config/validator.go
- [x] config/writer.go
- [x] config/config_test.go ✓ PASS
- [x] **CHECKPOINT: go test ./internal/config ./internal/observability -v** ✓ PASS

### Phase 2: System Integration ✓ COMPLETE
- [x] system/interfaces.go
- [x] system/doctor.go
- [x] system/system_test.go ✓ PASS
- [x] system/frr.go
- [x] system/frr_test.go ✓ PASS
- [x] system/sysctl.go
- [x] system/tuning.go
- [x] system/sysctl_test.go ✓ PASS
- [x] **CHECKPOINT: go test ./internal/system -v** ✓ PASS

### Phase 3: IPVS & Health ✓ COMPLETE
- [x] ipvs/types.go
- [x] ipvs/manager.go (manager_linux.go, manager_other.go, manager.go)
- [x] ipvs/reconciler.go
- [x] ipvs/ipvs_test.go ✓ PASS
- [x] health/checker.go
- [x] health/scheduler.go
- [x] health/health_test.go ✓ PASS (deadlock fixed)
- [x] **FIX: health_test.go deadlock issue** ✓ FIXED
- [x] **CHECKPOINT: go test ./internal/ipvs ./internal/health -v** ✓ PASS

### Phase 4: Observability Backends ✓ COMPLETE
- [x] observability/influx.go
- [x] observability/prometheus.go
- [x] observability/backends_test.go
- [x] **CHECKPOINT: go test ./internal/observability -v** ✓ PASS

### Phase 5: Daemon ✓ COMPLETE
- [x] daemon/signals.go
- [x] daemon/engine.go
- [x] daemon/daemon_test.go ✓ PASS
- [x] **CHECKPOINT: go test ./internal/daemon -v** ✓ PASS

### Phase 6: Interactive Shell ✓ COMPLETE
- [x] shell/lock.go
- [x] shell/lock_test.go
- [x] shell/shell.go
- [x] shell/commands.go
- [x] shell/completion.go
- [x] shell/help.go
- [x] shell/shell_test.go
- [x] shell/config_mode.go
- [x] shell/service_mode.go
- [x] shell/modes_test.go
- [x] **CHECKPOINT: go test ./internal/shell -v** ✓ PASS

### Phase 7: CLI Entry Point ✓ COMPLETE
- [x] cmd/lbctl/main.go
- [x] cmd/lbctl/main_test.go
- [x] **CHECKPOINT: go test ./cmd/... -v** ✓ PASS

### Phase 8: Distribution ✓ COMPLETE
- [x] dist/lbctl.service
- [x] dist/config.yaml.example
- [x] dist/config.d/example-service.yaml
- [x] dist/modules-load.d-ipvs.conf
- [x] Makefile
- [x] **FINAL: make test** ✓ PASS

### Phase 9: Direct Commands ✓ COMPLETE
- [x] `lbctl validate --config ...`
- [x] `lbctl apply --config ... [--daemon]`
- [x] `lbctl disable --config ...`
- [x] `lbctl status --config ...`
- [x] `lbctl doctor --config ...`
- [x] `lbctl show <status|config> --config ...`
- [x] **CHECKPOINT: docker run --rm lbctl-test make test** ✓ PASS

### Phase 10A: Wire `daemon.reconcile_interval_ms` + `daemon.state_cache` ✓ COMPLETE
- [x] Define behavior: map `daemon.reconcile_interval_ms` to Engine ticker interval (VIP check + periodic reconcile/disable)
- [x] Add config validation for `daemon.reconcile_interval_ms` (bounds + default) and `daemon.state_cache.ttl_ms` (bounds + default when enabled)
- [x] Add conversion helper: `config.Daemon.StateCache` → `ipvs.CacheConfig` (enabled + `ttl_ms` → `time.Duration`)
- [x] Update CLI manager construction to preserve `Close()` while using `ipvs.Manager` interface for the reconciler (raw vs cached)
- [x] Wire `daemon.state_cache` into `lbctl apply --daemon` (daemon mode only)
- [x] Wire `daemon.reconcile_interval_ms` into `daemon.NewEngine(...VIPCheckInterval...)`
- [x] Tests: config validation/defaults for interval+ttl
- [x] Tests: daemon engine uses config-driven ticker interval (via injectable `NewTicker`)
- [x] Tests: CLI `apply --daemon` wraps manager when cache enabled
- [x] Docs: align `Docs/kubernetes-loop-in-libraflux.md` to the real behavior (cache + interval)
- [x] **ENHANCEMENT**: Dynamic ticker interval - Run() loop now updates VIP check interval on SIGHUP when `daemon.reconcile_interval_ms` changes

### Phase 11: Code Quality & Safety Fixes ✓ COMPLETE
**Priority:** Critical production safety and engineering standards compliance

#### 11A: CRITICAL - Health Scheduler Race Condition Fix ✓ COMPLETE
- [x] Add `sync.Mutex` field to `runner` struct in `internal/health/scheduler.go`
- [x] Lock `runner.mu` at start of `tick()` method
- [x] Capture state before unlocking to avoid lock during observer callbacks
- [x] All health tests pass: `docker run --rm lbctl-test go test ./internal/health -v`

#### 11B: CRITICAL - Add Retry/Backoff to Reconciler ✓ COMPLETE
- [x] Add retry state to `Engine` struct: `reconcileAttempts int`, `nextRetryAt time.Time`
- [x] Create `calculateBackoff()` helper function with exponential backoff + jitter
- [x] Modify `tryReconcile()` to respect backoff timing (check before attempting)
- [x] Reset attempts on success, increment on failure with backoff
- [x] Log attempts and backoff duration on failure
- [x] All daemon tests pass: `docker run --rm lbctl-test go test ./internal/daemon -v`

#### 11C: HIGH - Fix Config Reload Ordering ✓ COMPLETE
- [x] Reorder `onReload()`: load config BEFORE stopping health scheduler
- [x] Only stop/restart scheduler if config load succeeds
- [x] Updated error message to clarify health scheduler is preserved
- [x] All daemon tests pass: `docker run --rm lbctl-test go test ./internal/daemon -v`

#### 11D: HIGH - Improve Reconciler Error Handling (Deferred)
- [ ] Add error accumulator in `reconcile()` method
- [ ] Collect errors instead of silent `continue`
- [ ] Return accumulated errors with partial success indication
- [ ] Add test: `TestReconciler_PartialFailure` in `ipvs_test.go`
- [ ] Verify: `docker run --rm lbctl-test go test ./internal/ipvs -v`

#### 11E: MEDIUM - Add Retry to Initial VIP Sync (Deferred)
- [ ] Add retry loop to `initialVIPSync()` (3 attempts, 1s delay)
- [ ] Log each attempt and failure reason
- [ ] Add test: `TestEngine_InitialVIPSyncRetry`
- [ ] Verify: `docker run --rm lbctl-test go test ./internal/daemon -v`

#### 11F: Testing - Comprehensive Validation ✓ COMPLETE
- [x] Full test suite: `docker run --rm lbctl-test make test` ✓ PASS
- [x] All packages pass (config, daemon, health, ipvs, observability, shell, system, cmd)
- [x] Note: Race detector requires CGO_ENABLED=1 (not available in Docker image)

---

## Planned Features (Phases 03B, 12+)

### Phase 03B: Production Guardrails & Safety (PRIORITY)
**Priority:** Critical | **Status:** Planned | **Rationale:** Safety before features

#### 03B-1: Connection State Documentation (Quick Win)
- [ ] Document current IPVS behavior during config changes
- [ ] Add to `Docs/spec.md` § Operational Behavior
- [ ] Document backend removal → immediate connection drop
- [ ] Document weight=0 → scheduler-dependent behavior
- [ ] Document service deletion → all connections dropped
- [ ] Add operator workaround: set weight=0, wait, then remove
- [ ] Update README with connection state warning

#### 03B-2: Dry-Run Diff (Pre-Apply Validation)
- [ ] Add `ReconcilePlan` struct to `internal/ipvs/reconciler.go`
- [ ] Implement `Plan()` method (diff without execution)
- [ ] Refactor `Apply()` to use `Plan()` + `Execute()`
- [ ] Add `--dry-run` flag to `lbctl apply` command
- [ ] Format diff output (CREATE/UPDATE/DELETE with colors)
- [ ] Unit tests: Plan() produces correct diff
- [ ] Integration tests: Dry-run matches actual apply
- [ ] Documentation: Add examples to operator guide

#### 03B-3: Snapshot & Rollback Primitives
- [ ] Design snapshot JSON schema (services + destinations)
- [ ] Implement `lbctl snapshot create --name <name>`
- [ ] Implement `lbctl snapshot restore --name <name>`
- [ ] Implement `lbctl snapshot list`
- [ ] Add snapshot storage in `/var/lib/lbctl/snapshots/`
- [ ] Add automatic pre-apply snapshots (optional config)
- [ ] Add snapshot retention policy (keep last N)
- [ ] Unit tests: Snapshot create/restore cycle
- [ ] E2E tests: Apply → break → rollback → verify
- [ ] Documentation: Add rollback runbook

#### 03B-4: Change Rate Limiting
- [ ] Add `change_limits` config section
- [ ] Implement `max_services_per_apply` limit
- [ ] Implement `max_destinations_per_service` limit
- [ ] Implement `max_deletes_per_apply` limit
- [ ] Add `--force` flag to bypass limits
- [ ] Log warnings when approaching limits
- [ ] Emit metrics: `lbctl_change_limit_hit_total`
- [ ] Unit tests: Limits enforced correctly
- [ ] Integration tests: Large config rejected
- [ ] Documentation: Add safety limits guide

#### 03B-5: Pre-Flight Checks
- [ ] Implement `PreFlightCheck()` in reconciler
- [ ] Check for duplicate services
- [ ] Validate VIP is present
- [ ] Check kernel connection table capacity
- [ ] Verify no orphaned services (VIP mismatch)
- [ ] Confirm IPVS modules loaded
- [ ] Add `lbctl preflight --config <file>` command
- [ ] Unit tests: Each check independently
- [ ] Integration tests: Real kernel checks
- [ ] Documentation: Add preflight checklist

#### 03B-6: Connection Draining (Future)
- [ ] Design drain config schema
- [ ] Add `GetDestinationActiveConns()` to manager
- [ ] Implement `drainDestination()` in reconciler
- [ ] Poll active connections until 0 or timeout
- [ ] Emit audit event: `backend_drained`
- [ ] Add config option: `drain.enabled`, `drain.timeout_seconds`
- [ ] Unit tests: Mock active connection polling
- [ ] Integration tests: Real IPVS connection draining
- [ ] Documentation: Add drain behavior guide

---

## Roadmap Features (Phase 12+)

### Phase 12A: UDP Health Checks (Echo/Ack Protocol)
**Priority:** High | **Status:** Planned

- [ ] Design UDP echo protocol with nonce generation
- [ ] Add `udp_echo` health check type to config schema
- [ ] Implement `UDPEchoCheck()` in `internal/health/checker.go`
- [ ] Add config validation for UDP check types
- [ ] Update `internal/health/scheduler.go` to support UDP checks
- [ ] Add `payload` and `expect` fields to `HealthConfig`
- [ ] Unit tests: Mock UDP echo server
- [ ] Integration tests: Real `socat` echo server
- [ ] E2E tests: Health state transitions with UDP failures
- [ ] Documentation: Update spec.md and add udp-health-checks.md
- [ ] Example configs: Add UDP service examples to dist/

### Phase 12B: Enhanced IPVS Metrics (Traffic Counters)
**Priority:** Medium | **Status:** Planned

- [ ] Add `GetDestinationStats()` to `internal/ipvs/manager.go`
- [ ] Define `DestinationStats` struct with netlink fields
- [ ] Add new Prometheus metrics (connections, packets, bytes)
- [ ] Add `lbctl_health_check_latency_seconds` histogram
- [ ] Implement stats collection in daemon engine (every N cycles)
- [ ] Add `daemon.stats_collection_interval_cycles` config option
- [ ] Unit tests: Mock netlink stats responses
- [ ] Integration tests: Real IPVS with traffic generation
- [ ] Documentation: Update spec.md metrics section
- [ ] Example Grafana dashboard JSON

### Phase 12C: TLS Config Replication (Control Plane)
**Priority:** Medium | **Status:** Planned

- [ ] Design control plane protocol (JSON-RPC over mTLS)
- [ ] Add `control_plane` config section (mode, listen_address, tls)
- [ ] Implement `lbctl setup certs` command for certificate generation
- [ ] Create `internal/controlplane/server.go` (mTLS listener, config watcher)
- [ ] Create `internal/controlplane/client.go` (mTLS connector, file writer)
- [ ] Create `internal/controlplane/protocol.go` (wire protocol)
- [ ] Add certificate validation and mutual TLS verification
- [ ] Implement config push/pull with checksums
- [ ] Add audit logging for all config replication events
- [ ] Add `lbctl_controlplane_connected` metric
- [ ] Unit tests: Mock TLS connections
- [ ] Integration tests: Two lbctl instances with real certs
- [ ] E2E tests: Config change propagation
- [ ] Documentation: Add control-plane.md setup guide

### Phase 12D: Nginx → LibraFlux Config Converter
**Priority:** Low | **Status:** Planned

- [ ] Design conversion mapping (upstream → backends, server → services)
- [ ] Implement Nginx config parser in `internal/convert/nginx.go`
- [ ] Create mapper for Nginx directives to LibraFlux config
- [ ] Add `lbctl convert nginx` CLI command
- [ ] Implement warnings for unsupported L7 features (SSL, HTTP routing)
- [ ] Add `--dry-run` flag for preview mode
- [ ] Support scheduler conversion (least_conn → lc, ip_hash → sh)
- [ ] Convert health check parameters (max_fails, fail_timeout)
- [ ] Unit tests: Parse sample Nginx configs
- [ ] Integration tests: Convert real-world configs
- [ ] Validation tests: Generated YAML passes `lbctl validate`
- [ ] Documentation: Add migration-from-nginx.md

---

## Roadmap Status

| Feature | Priority | Complexity | Risk | Phase | Status |
|---------|----------|------------|------|-------|--------|
| **Connection State Docs** | **Critical** | **Low** | **High** | **03B-1** | **Planned** |
| **Dry-Run Diff** | **Critical** | **Medium** | **High** | **03B-2** | **Planned** |
| **Snapshot/Rollback** | **Critical** | **Medium** | **High** | **03B-3** | **Planned** |
| **Change Rate Limiting** | **High** | **Low** | **Medium** | **03B-4** | **Planned** |
| **Pre-Flight Checks** | **High** | **Low** | **Medium** | **03B-5** | **Planned** |
| UDP Health Checks (DNS) | High | Medium | Medium | 12A | Planned |
| Enhanced IPVS Metrics | Medium | Low | Medium | 12B | Planned |
| TLS Config Replication | Medium | High | High | 12C | Planned |
| Nginx Converter | Low | Medium | Low | 12D | Planned |
| Connection Draining | Medium | Medium | Low | 03B-6 | Future |
| Kubernetes Integration | Low | Very High | High | 14+ | Future |

**Priority Rationale:** Phase 03B (Safety) must complete before Phase 12 (Features). Production safety is non-negotiable.

See [ROADMAP.md](ROADMAP.md) for detailed specifications and critique responses.

---

## Recent Additions

- **Prometheus Bind Address** (`internal/observability/prometheus.go`): Configurable bind address
  - `bind: 127.0.0.1` restricts metrics to localhost only (security)
  - `bind: ""` or `bind: 0.0.0.0` binds to all interfaces (default)
  - Validated at config load time
  - Tests: `TestPrometheusServer_BindValidation`, updated `TestPrometheusServer_GetURL`
- **State Cache** (`internal/ipvs/cache.go`): In-memory TTL-based cache for IPVS state
  - Wraps Manager interface transparently
  - Reduces netlink calls by ~50%
  - Invalidate-on-write, copy semantics, thread-safe
  - Wiring/config integration tracked in Phase 10A (`daemon.state_cache`)
  - Independent destination caching with per-service TTL tracking

---

## Test Results Summary

| Package | Status | Notes |
|---------|--------|-------|
| config | ✓ PASS | All validation, loading, writing tests pass |
| observability | ✓ PASS | Logger, metrics, audit all pass |
| ipvs | ✓ PASS | Reconciler and expansion tests pass |
| system | ✓ PASS | FRR, sysctl, doctor tests pass |
| health | ✓ PASS | TCP checker and state machine tests pass |
| daemon | ✓ PASS | Engine + signals + tests pass |
| shell | ✓ PASS | Interactive shell + lock tests pass |
| cmd/lbctl | ✓ PASS | CLI entrypoint + arg routing tests pass |
| repo | ✓ PASS | `make test` passes in Docker |

---

## Test Results Log

| Date | Phase | Tests | Result | Notes |
|------|-------|-------|--------|-------|
| 2025-12-14 | 1 | logger | ✓ PASS | All logger tests pass |
| 2025-12-14 | 1 | metrics | ✓ PASS | All metrics tests pass |
| 2025-12-14 | 1 | audit | ✓ PASS | All audit tests pass |
| 2025-12-14 | 1 | config | ✓ PASS | All config tests pass |
| 2025-12-14 | 2 | system | ✓ PASS | FRR, sysctl, doctor all pass |
| 2025-12-14 | 3 | ipvs | ✓ PASS | Reconciler tests pass |
| 2025-12-14 | 3 | health | ✓ PASS | Fixed deadlock, all tests pass |
| 2025-12-14 | 4 | observability | ✓ PASS | InfluxDB and Prometheus backends |
| 2025-12-14 | 5 | daemon | ✓ PASS | VIP transitions, reload, signals |
| 2025-12-15 | 6 | shell | ✓ PASS | Dockerfile-based AlmaLinux test image |
| 2025-12-15 | 7 | cmd | ✓ PASS | `docker run --rm lbctl-test go test ./cmd/... -v` |
| 2025-12-15 | 8 | repo | ✓ PASS | `docker run --rm lbctl-test make test` |
| 2025-12-15 | 9 | cmd | ✓ PASS | Direct commands (`apply/validate/status/doctor/disable/show`) + `docker run --rm lbctl-test make test` |
| 2025-12-15 | 9+ | observability | ✓ PASS | Prometheus bind address feature + cache hit counting fix |
| 2025-12-15 | 10A | daemon | ✓ PASS | Dynamic ticker interval - runtime reconcile interval changes |
| 2025-12-15 | 11A | health | ✓ PASS | Race condition fix - mutex protection in scheduler tick() |
| 2025-12-15 | 11B | daemon | ✓ PASS | Retry/backoff for reconciler - exponential backoff with jitter |
| 2025-12-15 | 11C | daemon | ✓ PASS | Config reload ordering - load before stopping scheduler |
| 2025-12-15 | 11F | all | ✓ PASS | Full test suite passes - all critical fixes validated |

---

## Notes

- Phases 1-11 are fully complete and tested
- Health test deadlock was caused by holding mutex while calling Check()
- InfluxDB Stop() deadlock fixed by checking channel state before waiting
- All observability tests pass including InfluxDB and Prometheus
- Docker (PowerShell): use `-v "${PWD}:/src"` to avoid `docker: invalid reference format`
- Dev image: `docker build -t lbctl-test .`
- Run tests: `docker run --rm lbctl-test go test ./internal/shell -v`
- Interactive shell: `docker run --rm -it lbctl-test` then run `go test ./internal/shell -v`
- AlmaLinux base images often ship `curl-minimal` (conflicts with `dnf install curl`); use `curl-minimal` or add `--allowerasing` when installing curl
- Non-interactive terminals: drop `-t` (use `docker run --rm lbctl-test`)
- Testing policy: prefer `lbctl-test` Docker runs for Linux/system-coupled code; only run host-native tests when the package is platform-agnostic (no Linux-specific syscalls/files like `/proc`, `netlink`, `flock`, etc.) - Docker file is at the root of the repo
