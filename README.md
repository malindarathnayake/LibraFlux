<p align="center">
  <img src="_logo/logo.svg" alt="LibraFlux Logo" width="600">
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/License-Apache_2.0-blue?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/Linux-FCC624?style=for-the-badge&logo=linux&logoColor=black" alt="Linux">
</p>

**Kernel-level L4 load balancer using Linux IPVS for data plane and FRR VRRP for VIP failover.**

LibraFlux uses a Kubernetes-style reconciliation loop to continuously ensure your desired load balancer state matches the actual IPVS kernel tables. It self-heals from drift, handles HA failover seamlessly, and integrates health checking with automatic weight management.

## Features

- **IPVS Management** - Direct kernel-level L4 load balancing via netlink
- **Reconciliation Loop** - Kubernetes-style desired-state controller
- **Health Checking** - TCP health probes with configurable thresholds
- **HA Aware** - Reacts to VIP transitions from Keepalived/FRR VRRP
- **Interactive Shell** - Cisco-style CLI for configuration and inspection
- **Observability** - Prometheus metrics, GELF logging, audit trail

## Quick Start

```bash
# Build
go build -o lbctl ./cmd/lbctl

# Run daemon mode
sudo ./lbctl daemon --config /etc/libraflux/config.yaml

# Interactive shell
sudo ./lbctl shell
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
│   ├── shell/           # Interactive Cisco-style shell
│   └── daemon/          # Main control loop engine
├── dist/                # Distribution files (systemd, examples)
├── Docs/                # Specification and standards
├── go.mod
└── README.md
```

## Documentation

- [spec.md](Docs/spec.md) - Complete project specification
- [engineering-standards.md](Docs/engineering-standards.md) - Implementation patterns
- [PROGRESS.md](Docs/PROGRESS.md) - Implementation progress tracker

## CLI Command

The binary is named `lbctl` (load balancer control) for ergonomic daily use:

```bash
lbctl daemon    # Run as daemon
lbctl shell     # Interactive shell
lbctl doctor    # System health check
lbctl version   # Show version
```

## License

Apache 2.0
