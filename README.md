<p align="center">
  <img src="_logo/logo.svg" alt="LibraFlux Logo" width="600">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/License-Apache_2.0-blue?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux">
</p>

**Kernel-level L4 load balancer using Linux IPVS for data plane and FRR VRRP for VIP failover.**

LibraFlux (ships as the `lbctl` binary) runs a Kubernetes-style reconciliation loop to keep Linux IPVS aligned to a declarative config, with health checks driving backend weight (healthy → configured weight, unhealthy → `0`).

For HA, LibraFlux is designed to run alongside FRR/keepalived VRRP: it becomes active and reconciles only when it detects the VIP is present on the node (VIP assignment is handled by VRRP, not by `lbctl`).

## Why LibraFlux over Nginx/HAProxy?

LibraFlux is fundamentally different. Nginx and HAProxy are **User-Space Reverse Proxies**. LibraFlux is a **Kernel-Space Controller** for IPVS.

### Direct Server Return (DSR) / High Throughput

This is the biggest differentiator.

- **Nginx/HAProxy**: `Client → LB → Server → LB → Client`. The LB processes all return traffic and becomes a bottleneck.
- **LibraFlux (IPVS)**: In **DR mode**, `Client → LB → Server → Client` (return path bypasses the LB). In **NAT mode**, return traffic still traverses the LB.

DR mode requires backend host preparation (VIP on loopback + ARP suppression, usually same L2). For local validation inside Docker, the provided functional harness uses NAT mode by default.

### Self-Healing Reconciliation Loop

Nginx and HAProxy are static daemons—load config and run. 

LibraFlux works like a Kubernetes Controller:
- Constantly checks **Current State** (Kernel IPVS tables)
- Compares to **Desired State** (Config)
- Applies changes to converge toward the desired state (and keeps retrying on failures)

This helps recover from drift (e.g., manual changes to IPVS), while surfacing behavior via logs/metrics/audit events.

### Layer 4 Only = Minimal Overhead

- **Nginx/HAProxy**: Excel at Layer 7—URL routing, SSL termination, header manipulation.
- **LibraFlux**: Layer 4 only. IPs and Ports. Cannot see URLs or certificates.

If you're balancing DNS servers, database clusters, or raw TCP/UDP streams, Nginx is often unnecessary complexity. LibraFlux programs the kernel data plane directly and avoids L7 parsing/termination.

### When to Use What

| Criteria | Nginx / HAProxy | LibraFlux |
|----------|-----------------|-----------|
| Traffic Type | HTTP, HTTPS, WebSockets, gRPC | Raw TCP, UDP, High-Volume Data |
| Return Path | Always via LB | DR mode can bypass LB |
| Logic | Smart (URLs, Headers, Cookies) | Fast (IPs and Ports) |
| Architecture | User-Space Proxy | Kernel-Space Controller |

**Use LibraFlux**: A declarative L4 controller for internal infrastructure (databases, K8s clusters, DNS) using Linux IPVS, with optional DR mode for high-throughput return paths.

**Use Nginx/HAProxy**: Websites, SSL termination, URL-based routing, public internet traffic.

## Features

- **IPVS Management** - Direct kernel-level L4 load balancing via netlink
- **Reconciliation Loop** - Kubernetes-style desired-state controller
- **Health Checking** - TCP health probes with configurable thresholds and weight-to-zero behavior on failures
- **HA Aware** - Gated reconciliation based on VIP presence (designed to work with FRR/keepalived VRRP)
- **FRR Integration** - Optional FRR config managed-block patching for VRRP stanzas
- **Interactive Shell** - Network-device-style CLI for configuration and inspection
- **Observability First** - Prometheus metrics, optional InfluxDB push, GELF logging, and a structured audit event stream

## Built Monitoring-First

LibraFlux was built “monitoring first”: structured logging, a metrics registry, and audit events were scaffolded early and used to guide the rest of the implementation. The project milestones and test checkpoints are tracked in `Docs/PROGRESS.md`.

## Quick Start

```bash
# Build
go build -o lbctl ./cmd/lbctl

# Validate config
sudo ./lbctl validate --config /etc/lbctl/config.yaml

# Apply once (one-shot reconciliation)
sudo ./lbctl apply --config /etc/lbctl/config.yaml

# Run daemon mode (monitor VIP + reconcile continuously)
sudo ./lbctl apply --daemon --config /etc/lbctl/config.yaml

# Interactive shell (TTY: starts the shell by default)
sudo ./lbctl --config /etc/lbctl/config.yaml
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                       LibraFlux                             │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Config    │  │   Health    │  │    Reconciler       │  │
│  │   Loader    │──│   Checker   │──│  (Desired→Actual)   │  │
│  └─────────────┘  └─────────────┘  └──────────┬──────────┘  │
│                                                │             │
│  ┌─────────────────────────────────────────────▼──────────┐  │
│  │                    IPVS Manager                        │  │
│  │                  (netlink to kernel)                   │  │
│  └────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Linux Kernel (IPVS)                      │
└─────────────────────────────────────────────────────────────┘
```

## Building

```bash
# Download dependencies
go mod download

# Run tests
go test ./...

# Run tests in AlmaLinux container (recommended for IPVS/netlink)
make docker-test

# Build binary
go build -o lbctl ./cmd/lbctl
```

## Project Structure

```
LibraFlux/
├── cmd/lbctl/           # CLI entry point
├── internal/
│   ├── observability/   # Logging, metrics, audit
│   ├── config/          # Configuration management
│   ├── ipvs/            # IPVS management & reconciler
│   ├── health/          # Health checking scheduler
│   ├── system/          # System integration (sysctl, FRR)
│   ├── shell/           # Interactive network-device-style shell
│   └── daemon/          # Main control loop engine
├── dist/                # Distribution files (systemd, examples)
├── Docs/                # Specification and standards
├── go.mod
└── README.md
```

## CLI Command

LibraFlux ships as a single binary named `lbctl` (load balancer control):

```bash
lbctl apply [--daemon]     # Apply config (one-shot or run daemon)
lbctl validate             # Validate config only
lbctl status               # Print node/VIP/config summary
lbctl show <topic>         # show status|config
lbctl disable              # Remove managed IPVS services for VIP
lbctl doctor               # Run diagnostic checks
```

## Community Project

If you want this to be a shared learning project: open issues for “paper cuts”, add lab notes to `Docs/`, and contribute small PRs that improve correctness, observability, and test coverage. The spec and engineering standards are intentionally written to be readable and evolvable as the community learns.

## License

Apache 2.0
