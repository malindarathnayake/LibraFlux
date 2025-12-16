# LibraFlux Roadmap

This document outlines what's coming next for LibraFlux. If you're interested in contributing or have feedback on priorities, please open an issue!

---

## Current Focus: Production Safety First

Before adding new features, we're prioritizing production safety mechanisms. Why? Because kernel-level load balancing is powerful but unforgiving—a bug can drop all your traffic.

---

## Phase 03B: Safety Guardrails (In Progress)

### Document Connection Behavior

**The Problem:** What happens to active connections when you remove a backend or change its weight?

Right now, IPVS drops connections immediately when you remove a backend. This can surprise operators during maintenance windows.

**What we're doing:**
- Document the current behavior clearly in the spec
- Provide workarounds (set `weight: 0`, wait for drain, then remove)
- Plan future connection draining feature

**Why this matters:** L4 load balancers often fail during config changes, not steady-state operation.

---

### Dry-Run Mode

**The Problem:** You want to see what changes will happen before applying them.

**What we're adding:**

```bash
lbctl apply --config /etc/lbctl/config.yaml --dry-run
```

This will show you:
- Services to be created (with backends)
- Services to be updated (what's changing)
- Services to be deleted

Think of it like `kubectl diff` or `terraform plan`.

---

### Snapshot & Rollback

**The Problem:** You apply a config change and something breaks. How do you quickly roll back?

**What we're adding:**

```bash
# Before making changes
lbctl snapshot create --name before-upgrade

# Apply your changes
lbctl apply --config /etc/lbctl/config.yaml

# Oops, something broke - roll back
lbctl snapshot restore --name before-upgrade
```

Snapshots are stored in `/var/lib/lbctl/snapshots/` as JSON files containing your IPVS state.

---

### Change Rate Limiting

**The Problem:** A typo in your config could accidentally delete 100 services at once.

**What we're adding:**

Safety limits with sane defaults:
- Max 100 services created per apply
- Max 50 backends per service
- Max 10 services deleted per apply

You can override with `--force` if you really mean it, but the default is to prevent accidents.

---

### Pre-Flight Checks

**The Problem:** Config looks valid, but will it actually work on this system?

**What we're adding:**

Before applying config, check:
- Is the VIP actually present on this node?
- Are IPVS kernel modules loaded?
- Will this exceed kernel connection table limits?
- Any duplicate services?

Fail fast with clear error messages instead of partially applying broken config.

---

## Phase 02: Advanced Features

### UDP Health Checks (Real Protocol Validation)

**The Problem:** TCP health checks don't work for UDP services like DNS or SIP.

**What we're NOT doing:** Generic UDP echo checks. They require deploying separate echo listeners on every backend (security risk + operational burden).

**What we ARE doing:** Real protocol checks that validate your actual service.

**Priority order:**

1. **DNS checks** (most requested)
   ```yaml
   health:
     type: udp_dns
     dns_query: "health.example.com"
     expect_rcode: NOERROR
   ```

2. **SIP checks** (VoIP/telephony)
   ```yaml
   health:
     type: udp_sip
     sip_uri: "sip:health@192.168.1.10:5060"
   ```

3. **NTP checks** (time services)

4. **Generic echo** (last resort, with big warnings)

**Why DNS first?** It's the most common UDP service, and you can validate it without deploying extra infrastructure.

---

### Enhanced IPVS Metrics

**The Problem:** Current metrics show health and reconciliation, but not actual traffic stats.

**What we're adding:**

New Prometheus metrics:
- `lbctl_ipvs_connections_total` - Total connections per backend
- `lbctl_ipvs_connections_active` - Currently active connections
- `lbctl_ipvs_packets_in_total` / `lbctl_ipvs_packets_out_total`
- `lbctl_ipvs_bytes_in_total` / `lbctl_ipvs_bytes_out_total`
- `lbctl_health_check_latency_seconds` - Health check RTT histogram

**Cardinality warning:** These metrics are labeled by `node × service × backend`. If you have 10 services with 20 backends each, that's 200 metric series per metric type. We're adding:
- Configurable collection interval (default: 30s, separate from reconcile loop)
- In-memory caching between Prometheus scrapes
- Cardinality limits to prevent overload

---

## Phase 03: Enterprise Features

### Automatic Config Replication (Primary → Secondary)

**The Problem:** In HA pairs, you need to keep configs in sync. Current options are manual `rsync` or external tools like Ansible.

**What we're considering:**

Built-in mTLS tunnel for config replication:
- Primary node pushes config changes to Secondary
- Mutual TLS authentication
- Atomic file writes (no partial updates)
- Primary always wins (no merge conflicts)

**Design challenges we're addressing:**
- Certificate rotation (what happens when certs expire?)
- Conflict resolution (what if someone edits Secondary directly?)
- Replay protection (prevent rollback attacks)
- Reconnect storms (network partition heals)

**Current thinking:** Content-addressed bundles instead of JSON-RPC:
- Tar up `config.d/*` → `bundle-<sha256>.tar.gz`
- Send hash to Secondary
- Secondary downloads if not already present
- Atomic extraction and validation
- SIGHUP daemon

This approach is simpler and more robust than streaming file-by-file updates.

---

### Nginx → LibraFlux Config Converter

**The Problem:** Migrating from Nginx `stream` blocks is tedious and error-prone.

**What we're adding:**

```bash
lbctl convert nginx --input /etc/nginx/nginx.conf --output /etc/lbctl/config.d/
```

**What it converts:**
- `upstream` blocks → `backends`
- `server` blocks → `services`
- `least_conn` → `scheduler: lc`
- `ip_hash` → `scheduler: sh`
- `weight` / `max_fails` / `fail_timeout` → equivalent LibraFlux config

**What it can't convert:**
- SSL termination (L7 feature)
- HTTP routing (L7 feature)
- Nginx Plus dynamic upstreams
- Lua scripts

The tool will warn you about unsupported features and suggest alternatives.

---

## Phase 04+: Future Ideas

### Kubernetes Integration

Run LibraFlux as an external load balancer for Kubernetes `type: LoadBalancer` services (alternative to MetalLB or cloud providers).

This is a major undertaking and requires significant design work. Not planned for near-term.

---

## Implementation Priority

Here's what we're building, in order:

| Feature | Priority | Why |
|---------|----------|-----|
| **Connection state docs** | **Critical** | Quick win, high operator value |
| **Dry-run diff** | **Critical** | Prevent production incidents |
| **Snapshot/rollback** | **Critical** | Fast recovery from mistakes |
| **Change rate limiting** | **High** | Prevent accidental mass deletion |
| **Pre-flight checks** | **High** | Fail fast with clear errors |
| UDP health checks (DNS) | High | Most requested feature |
| Enhanced IPVS metrics | Medium | Operational visibility |
| Config replication | Medium | Enterprise HA feature |
| Nginx converter | Low | Migration convenience |
| Kubernetes integration | Low | Major project, future |

**Key principle:** Safety before features. We won't add new capabilities until we have proper guardrails in place.

---

## What's Still Missing

Ideas for future work (not yet prioritized):

- **Observability for config churn:** Metrics on reconcile frequency, change velocity, rollback count
- **Gradual rollout:** Canary backends (route 10% traffic to new backend before full weight)
- **Circuit breakers:** Automatically disable backends after N consecutive failures
- **Multi-VIP support:** Single daemon managing multiple VIPs (currently one daemon per VIP)

---

## How to Contribute

We welcome feature requests and design feedback! When opening an issue, please include:

- **Use case:** What problem are you trying to solve?
- **Expected behavior:** What should happen?
- **Production impact:** What breaks if this feature has a bug?

When proposing features, consider:
- **Blast radius:** How much damage can a bug cause?
- **Operational burden:** What new failure modes does this introduce?
- **Footguns:** Can operators misconfigure this in dangerous ways?

We prioritize features that are:
1. Safe by default
2. Hard to misuse
3. Solve real production problems

---

## Questions?

- **Why DNS checks before echo checks?** Echo requires deploying separate listeners (security risk, ops burden). DNS validates your actual service.
- **Why dry-run and snapshots?** IPVS programming is kernel-level—mistakes drop traffic. We need multiple safety layers.
- **Why cardinality warnings for metrics?** Per-backend metrics can create thousands of series. This can overload Prometheus at scale.
- **Why content-addressed bundles for config sync?** Simpler than JSON-RPC, atomic updates, replay protection built-in.

---

## References

- [Phase 01 Spec](spec.md) - Current implementation details
- [Engineering Standards](engineering-standards.md) - Code quality guidelines
- [Progress Tracker](PROGRESS.md) - What's been completed
- [Kubernetes Loop Article](kubernetes-loop-in-libraflux.md) - Design philosophy

---

**Last Updated:** December 2025
