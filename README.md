<p align="center">
  <img src="_logo/logo.svg" alt="LibraFlux Logo" width="600">

Kernel-Level L4 Load Balancing with Native Observability

![LibraFlux Logo](_logo/logo.svg)

[![Go](https://img.shields.io/badge/Go-00ADD8?style=flat&logo=go&logoColor=white)](https://golang.org) [![License](https://img.shields.io/badge/License-Apache_2.0-blue?style=flat)](LICENSE) [![Linux](https://img.shields.io/badge/Linux-FCC624?style=flat&logo=linux&logoColor=black)](https://kernel.org)

## Overview

Declarative L4 load balancer using Linux IPVS with built-in observability and Kubernetes-style reconciliation.

LibraFlux (`lbctl`) is designed for:

- **Bare-Metal** infrastructure
- **High-Availability** setups with VRRP (FRR/keepalived)
- **Internal services** (databases, K8s clusters, DNS)
- **Monitoring-first** environments requiring native telemetry

**NOTE:** Full documentation available in the [Deployment](Deployment/) and [Docs](Docs/) directories.

## Features

- **IPVS Management** - Direct kernel-level L4 load balancing via netlink
- **Reconciliation Loop** - Kubernetes-style desired-state controller
- **Health Checking** - TCP probes with automatic weight adjustment (healthy → configured weight, unhealthy → 0)
- **HA Aware** - VIP-gated reconciliation for active/standby operation with VRRP
- **FRR Integration** - Optional managed-block patching for VRRP configuration
- **Interactive Shell** - Network-device-style CLI for configuration and inspection
- **Native Observability** - Built-in Prometheus metrics, structured logging, audit events (no sidecars required)
- **Direct Server Return** - Optional DR mode for high-throughput return paths

## Why?

Existing L4 load balancers often require separate exporters for metrics or complex multi-tool setups for HA. LibraFlux treats observability as a first-class citizen and provides a single binary that integrates with your existing VRRP infrastructure.

### Comparison with User-Space Proxies

| Feature | Nginx/HAProxy | LibraFlux |
|---------|---------------|-----------|
| Architecture | User-space proxy | Kernel-space controller |
| Data plane | All traffic proxied | IPVS in kernel |
| Return path | Through load balancer | Optional DR (bypass LB) |
| Observability | Requires exporters | Built-in Prometheus |
| Configuration | Reload-based | Reconciliation loop |

**Use LibraFlux when:** You need L4 load balancing for internal infrastructure with native metrics and declarative configuration.

**Use Nginx/HAProxy when:** You need L7 features (SSL termination, URL routing, content manipulation).

## Quick Start

## Quick Start

```bash
# Build
go build -o lbctl ./cmd/lbctl

# Validate configuration
sudo ./lbctl validate --config /etc/lbctl/config.yaml

# Run daemon (VIP-aware reconciliation)
sudo ./lbctl apply --daemon --config /etc/lbctl/config.yaml

# Interactive shell
sudo ./lbctl --config /etc/lbctl/config.yaml
```

See [Deployment/QUICK-START.md](Deployment/QUICK-START.md) for detailed setup instructions.

## Configuration Example

```yaml
vip: "192.168.1.100"
services:
  - name: "postgres-cluster"
    virtual_ip: "192.168.1.100"
    virtual_port: 5432
    protocol: "tcp"
    scheduler: "rr"
    backends:
      - address: "192.168.1.101"
        port: 5432
        weight: 100
      - address: "192.168.1.102"
        port: 5432
        weight: 100
    health_check:
      type: "tcp"
      interval: "5s"
      timeout: "2s"
      retries: 3
```

## Observability

Built-in Prometheus metrics at `/metrics`:

- `lbctl_health_backend_healthy` - Backend health status
- `lbctl_health_backend_weight` - Current backend weight
- `lbctl_reconcile_runs_total` - Reconciliation loop counter
- `lbctl_reconcile_duration_ms` - Reconciliation latency
- `lbctl_vip_is_owner` - VIP ownership status
- `lbctl_vip_transitions_total` - VIP failover counter

Optional integrations: InfluxDB push, GELF logging, structured audit events.

## Building

```bash
# Run tests
go test ./...

# Build binary
go build -o lbctl ./cmd/lbctl

# Docker-based testing (recommended)
make docker-test
```

## Troubleshooting and Feedback

Please raise issues on the [GitHub repository](https://github.com/yourusername/LibraFlux/issues) and check the documentation in the [Deployment](Deployment/) directory.

Run diagnostics:

```bash
sudo ./lbctl doctor
```

## Contributing

Thanks for taking the time to join our community and start contributing! We welcome pull requests. Feel free to dig through the [issues](https://github.com/yourusername/LibraFlux/issues) and jump in.

## License

Apache 2.0

---

**Documentation:**
- [Quick Start Guide](Deployment/QUICK-START.md)
- [Deployment Guide](Deployment/README.md)
- [Engineering Standards](Docs/engineering-standards.md)
- [Project Progress](Docs/PROGRESS.md)
