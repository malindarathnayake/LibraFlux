# LibraFlux Deployment Resources

This directory contains installation scripts and deployment documentation for LibraFlux.

## Files

### Installation Scripts

- **`install.sh`** - One-shot installer for AlmaLinux 8, 9, 10 and compatible systems
  - Downloads latest release from GitHub
  - Configures IPVS kernel modules
  - Installs FRR for VRRP support
  - Sets up systemd service
  - Applies system tuning

### Documentation

- **`deployment-notes.md`** - Comprehensive deployment guide
  - Installation instructions
  - Post-installation configuration
  - High availability setup
  - Operating modes (DR vs NAT)
  - Troubleshooting guide
  - Performance tuning

- **`QUICK-START.md`** - Quick reference card
  - One-line installation
  - Essential commands
  - Common configurations
  - Quick troubleshooting

## Quick Installation

### One-Line Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

### Manual Installation

```bash
# Download script
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh -o install-libraflux.sh

# Review script
less install-libraflux.sh

# Make executable
chmod +x install-libraflux.sh

# Run with sudo
sudo ./install-libraflux.sh
```

## Installation Options

```bash
# Skip FRR installation (if using keepalived)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --skip-frr

# Dry-run mode (preview changes)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --dry-run

# Custom binary location
LBCTL_BIN=/usr/bin/lbctl curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash

# Custom config directory
LBCTL_CONFIG_DIR=/opt/lbctl/config curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

## Supported Platforms

| Platform | Version | Status | Notes |
|----------|---------|--------|-------|
| AlmaLinux | 8 | ✅ Supported | EPEL required for FRR |
| AlmaLinux | 9 | ✅ Supported | Recommended |
| AlmaLinux | 10 | ✅ Supported | Latest |
| Rocky Linux | 8, 9 | ✅ Supported | Same as AlmaLinux |
| RHEL | 8, 9 | ✅ Supported | Requires subscription |
| Ubuntu | 22.04+ | ✅ Supported | |
| Debian | 12+ | ✅ Supported | |

## What Gets Installed

### System Configuration

1. **IPVS Kernel Modules** (`/etc/modules-load.d/ipvs.conf`)
   - `ip_vs` - Core IPVS
   - `ip_vs_rr` - Round-robin scheduler
   - `ip_vs_wrr` - Weighted round-robin
   - `ip_vs_sh` - Source hashing

2. **Sysctl Tuning** (`/etc/sysctl.d/90-lbctl.conf`)
   - IP forwarding enabled
   - IPVS connection tracking
   - Connection table sizing
   - ARP behavior for DR mode
   - Conntrack limits

3. **FRR (Optional)**
   - VRRP daemon for high availability
   - Installed from EPEL (RHEL-based) or official repos (Debian-based)

### LibraFlux Components

1. **Binary** - `/usr/local/bin/lbctl`
2. **Configuration** - `/etc/lbctl/config.yaml`
3. **Service Definitions** - `/etc/lbctl/config.d/*.yaml`
4. **State Directory** - `/var/lib/lbctl/`
5. **Systemd Service** - `/etc/systemd/system/lbctl.service`

## Post-Installation

After installation, you need to:

1. **Configure your node** - Edit `/etc/lbctl/config.yaml`
2. **Define services** - Create files in `/etc/lbctl/config.d/`
3. **Validate config** - Run `sudo lbctl validate`
4. **Test application** - Run `sudo lbctl apply`
5. **Enable daemon** - Run `sudo systemctl enable --now lbctl`

See `QUICK-START.md` for detailed steps.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Installation Script                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Detect OS (AlmaLinux 8/9/10, Rocky, RHEL, etc.)        │
│  2. Install IPVS modules + ipvsadm                          │
│  3. Configure sysctl (IP forwarding, IPVS tuning)           │
│  4. Install FRR (VRRP support)                              │
│  5. Download lbctl binary from GitHub releases              │
│  6. Download config files                                   │
│  7. Install systemd service                                 │
│  8. Verify installation                                     │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Troubleshooting Installation

### Binary Download Fails

If the GitHub release doesn't exist yet:

```bash
# Build locally
git clone https://github.com/malindarathnayake/LibraFlux.git
cd LibraFlux
go build -o lbctl ./cmd/lbctl

# Run existing deploy script
sudo ./scripts/deploy.sh
```

### Module Loading Fails

Some systems may require a reboot for modules to load:

```bash
# Check if modules are configured
cat /etc/modules-load.d/ipvs.conf

# Reboot
sudo reboot

# After reboot, verify
lsmod | grep ip_vs
```

### FRR Installation Issues

If FRR installation fails:

```bash
# Skip FRR and use keepalived instead
curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --skip-frr

# Or install FRR manually
# See: https://docs.frrouting.org/en/latest/installation.html
```

### Permission Issues

Ensure you're running with sudo:

```bash
# Wrong
curl -fsSL https://... | bash

# Correct
curl -fsSL https://... | sudo bash
```

## Environment Variables

Customize installation paths:

| Variable | Default | Description |
|----------|---------|-------------|
| `GITHUB_REPO` | `malindarathnayake/LibraFlux` | GitHub repository |
| `GITHUB_RELEASE` | `latest` | Release tag to download |
| `LBCTL_BIN` | `/usr/local/bin/lbctl` | Binary install path |
| `LBCTL_CONFIG_DIR` | `/etc/lbctl` | Config directory |
| `LBCTL_STATE_DIR` | `/var/lib/lbctl` | State directory |

Example:

```bash
LBCTL_BIN=/usr/bin/lbctl \
LBCTL_CONFIG_DIR=/opt/lbctl \
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

## Security Considerations

1. **Review Before Running** - Always review scripts before piping to bash
2. **HTTPS Only** - Script uses HTTPS for all downloads
3. **Root Required** - Script needs root for system configuration
4. **Firewall** - Configure firewall after installation
5. **Metrics Endpoint** - Restrict Prometheus endpoint to localhost if needed

## Uninstallation

To remove LibraFlux:

```bash
# Stop service
sudo systemctl stop lbctl
sudo systemctl disable lbctl

# Clear IPVS rules
sudo ipvsadm -C

# Remove files
sudo rm -f /usr/local/bin/lbctl
sudo rm -f /etc/systemd/system/lbctl.service
sudo rm -rf /etc/lbctl
sudo rm -rf /var/lib/lbctl

# Optional: Remove system configs
sudo rm -f /etc/sysctl.d/90-lbctl.conf
sudo rm -f /etc/modules-load.d/ipvs.conf

# Reload systemd
sudo systemctl daemon-reload
```

## Development vs Production

### Development/Testing

Use the existing `scripts/deploy.sh` for local development:

```bash
# Build locally
go build -o lbctl ./cmd/lbctl

# Deploy to local system
sudo ./scripts/deploy.sh
```

### Production

Use the one-shot installer from GitHub:

```bash
# Install from latest release
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

## Getting Help

- **Quick Start:** `QUICK-START.md`
- **Full Guide:** `deployment-notes.md`
- **Project Docs:** `../Docs/`
- **GitHub Issues:** https://github.com/malindarathnayake/LibraFlux/issues

## Related Files

- `../scripts/deploy.sh` - Local development deployment script
- `../dist/` - Distribution files (configs, systemd units)
- `../Docs/spec.md` - LibraFlux specification
- `../README.md` - Project overview

---

**Note:** Replace `YOUR_ORG` with your actual GitHub organization/username in all URLs before publishing.

