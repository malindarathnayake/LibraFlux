# LibraFlux Deployment Notes

## One-Shot Installation for AlmaLinux 8, 9, 10

### Quick Install (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

Or with options:

```bash
# Skip FRR installation (if using keepalived)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --skip-frr

# Dry-run mode (preview changes without applying)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --dry-run
```

### Manual Installation

If you prefer to review the script before running:

```bash
# Download the script
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh -o install-libraflux.sh

# Review the script
less install-libraflux.sh

# Make it executable
chmod +x install-libraflux.sh

# Run with sudo
sudo ./install-libraflux.sh
```

---

## What the Installer Does

The installation script (`install.sh`) performs the following operations:

### 1. System Detection
- Detects OS type and version (AlmaLinux 8, 9, 10)
- Verifies root privileges
- Checks for container environment

### 2. IPVS Kernel Module Setup
- Configures `/etc/modules-load.d/ipvs.conf` to load:
  - `ip_vs` - Core IPVS module
  - `ip_vs_rr` - Round-robin scheduler
  - `ip_vs_wrr` - Weighted round-robin scheduler
  - `ip_vs_sh` - Source hashing scheduler
- Loads modules immediately (no reboot required)
- Installs `ipvsadm` utility for debugging

### 3. Sysctl Configuration
- Creates `/etc/sysctl.d/90-lbctl.conf` with optimized settings:
  - Enables IP forwarding (`net.ipv4.ip_forward = 1`)
  - Enables IPVS connection tracking for NAT mode
  - Configures connection table size and timeouts
  - Sets ARP behavior for Direct Return (DR) mode
  - Tunes conntrack limits
- Applies settings immediately

### 4. FRR Installation (Optional)
- Installs FRR (Free Range Routing) for VRRP support
- Enables EPEL repository on RHEL-based systems
- Configures `vrrpd` daemon in `/etc/frr/daemons`
- Starts and enables FRR service
- Can be skipped with `--skip-frr` flag

### 5. LibraFlux Binary & Configuration
- Downloads latest `lbctl` binary from GitHub releases
- Installs to `/usr/local/bin/lbctl`
- Creates directory structure:
  - `/etc/lbctl/` - Main configuration directory
  - `/etc/lbctl/config.d/` - Service definitions
  - `/var/lib/lbctl/` - State directory
  - `/var/lib/lbctl/backups/` - Configuration backups
- Installs example configuration files
- Installs systemd service unit

### 6. Verification
- Verifies IPVS modules are loaded
- Checks IP forwarding is enabled
- Confirms `ipvsadm` is available
- Validates FRR installation (if not skipped)
- Tests `lbctl` binary execution
- Verifies configuration files exist

---

## Post-Installation Steps

### 1. Configure LibraFlux

Edit the main configuration file:

```bash
sudo vi /etc/lbctl/config.yaml
```

**Required changes:**
- `node.name` - Unique name for this node (e.g., `lb-node-a`, `lb-node-b`)
- `node.role` - Set to `primary` or `secondary`
- `network.frontend.interface` - Your frontend network interface (e.g., `eth0`, `ens160`)
- `network.frontend.vip` - Virtual IP address for the load balancer
- `network.frontend.cidr` - CIDR prefix length (e.g., `24` for /24)
- `network.backend.interface` - Your backend network interface

**Example configuration:**

```yaml
mode: dr # or nat

node:
  name: lb-node-a
  role: primary

network:
  frontend:
    interface: eth0
    vip: 192.168.1.100
    cidr: 24
  backend:
    interface: eth1

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
  advert_interval_ms: 1000
```

### 2. Define Services

Create service definitions in `/etc/lbctl/config.d/`:

```bash
sudo vi /etc/lbctl/config.d/web-service.yaml
```

**Example service:**

```yaml
services:
  - name: web-cluster
    vip: 192.168.1.100
    port: 80
    protocol: tcp
    scheduler: rr  # rr, wrr, sh
    backends:
      - ip: 192.168.1.10
        port: 80
        weight: 100
      - ip: 192.168.1.11
        port: 80
        weight: 100
      - ip: 192.168.1.12
        port: 80
        weight: 100
    health_check:
      type: tcp
      interval_ms: 5000
      timeout_ms: 2000
      retries: 3
```

### 3. Validate Configuration

```bash
sudo lbctl validate --config /etc/lbctl/config.yaml
```

### 4. Test One-Shot Application

Apply configuration once without daemon mode:

```bash
sudo lbctl apply --config /etc/lbctl/config.yaml
```

Verify IPVS state:

```bash
sudo ipvsadm -Ln
```

### 5. Enable Daemon Mode

Start LibraFlux as a systemd service:

```bash
# Enable service to start on boot
sudo systemctl enable lbctl

# Start the service
sudo systemctl start lbctl

# Check status
sudo systemctl status lbctl

# View logs
sudo journalctl -u lbctl -f
```

### 6. Monitor Metrics

LibraFlux exposes Prometheus metrics on port 9090 by default:

```bash
# Check metrics endpoint
curl http://localhost:9090/metrics

# Key metrics to monitor:
# - lbctl_health_backend_healthy
# - lbctl_health_backend_weight
# - lbctl_reconcile_runs_total
# - lbctl_reconcile_duration_ms
# - lbctl_vip_is_owner
# - lbctl_vip_transitions_total
```

---

## High Availability Setup

For HA deployment, you need at least two nodes running LibraFlux with VRRP:

### Node A (Primary)

```yaml
node:
  name: lb-node-a
  role: primary

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
```

### Node B (Secondary)

```yaml
node:
  name: lb-node-b
  role: secondary

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
```

**Important:**
- Both nodes must have the same `vrid`
- Both nodes must have identical service definitions
- Only the active node (with VIP) will reconcile IPVS state
- VRRP handles VIP failover automatically

---

## Operating Modes

### Direct Return (DR) Mode

**Best for:** High-throughput scenarios where return traffic bypasses the load balancer

**Requirements:**
- Backends must be on the same L2 network as the load balancer
- Backends must have VIP configured on loopback with ARP suppression
- Load balancer rewrites destination MAC address only

**Backend configuration:**

```bash
# On each backend server
sudo ip addr add 192.168.1.100/32 dev lo

# Suppress ARP responses for VIP
sudo sysctl -w net.ipv4.conf.lo.arp_ignore=1
sudo sysctl -w net.ipv4.conf.lo.arp_announce=2
```

### NAT Mode

**Best for:** Backends on different networks, Docker testing, simpler setup

**Requirements:**
- Load balancer performs SNAT and DNAT
- Return traffic must go through the load balancer
- Backends use load balancer as default gateway (or route VIP traffic to LB)

**No special backend configuration required.**

---

## Troubleshooting

### Check IPVS State

```bash
# List all virtual services
sudo ipvsadm -Ln

# Show connection statistics
sudo ipvsadm -Ln --stats

# Show rate statistics
sudo ipvsadm -Ln --rate
```

### Check Module Loading

```bash
# List loaded IPVS modules
lsmod | grep ip_vs

# Load modules manually if needed
sudo modprobe ip_vs
sudo modprobe ip_vs_rr
sudo modprobe ip_vs_wrr
```

### Check IP Forwarding

```bash
# Check current setting
cat /proc/sys/net/ipv4/ip_forward

# Enable temporarily
sudo sysctl -w net.ipv4.ip_forward=1

# Enable permanently (already done by installer)
sudo sysctl -p /etc/sysctl.d/90-lbctl.conf
```

### Check VIP Presence

```bash
# List all IP addresses
ip addr show

# Check specific interface
ip addr show dev eth0
```

### Check FRR/VRRP Status

```bash
# Enter FRR shell
sudo vtysh

# Show VRRP status
show vrrp

# Show running config
show running-config

# Exit
exit
```

### Check LibraFlux Logs

```bash
# Follow logs in real-time
sudo journalctl -u lbctl -f

# Show recent logs
sudo journalctl -u lbctl -n 100

# Show logs since boot
sudo journalctl -u lbctl -b
```

### Run Diagnostics

```bash
# Run built-in diagnostic checks
sudo lbctl doctor
```

### Test Backend Connectivity

```bash
# Test TCP connection to backend
nc -zv 192.168.1.10 80

# Test HTTP response
curl -v http://192.168.1.10:80
```

---

## Firewall Configuration

If firewall is enabled, allow necessary traffic:

```bash
# Allow VRRP (protocol 112)
sudo firewall-cmd --permanent --add-protocol=vrrp
sudo firewall-cmd --permanent --add-protocol=112

# Allow VIP traffic (example for HTTP/HTTPS)
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https

# Allow Prometheus metrics (if external monitoring)
sudo firewall-cmd --permanent --add-port=9090/tcp

# Reload firewall
sudo firewall-cmd --reload
```

---

## Uninstallation

To remove LibraFlux:

```bash
# Stop and disable service
sudo systemctl stop lbctl
sudo systemctl disable lbctl

# Remove IPVS rules
sudo ipvsadm -C

# Remove files
sudo rm -f /usr/local/bin/lbctl
sudo rm -f /etc/systemd/system/lbctl.service
sudo rm -rf /etc/lbctl
sudo rm -rf /var/lib/lbctl

# Remove sysctl config (optional)
sudo rm -f /etc/sysctl.d/90-lbctl.conf

# Remove modules config (optional)
sudo rm -f /etc/modules-load.d/ipvs.conf

# Reload systemd
sudo systemctl daemon-reload
```

---

## Supported Platforms

| Platform | Version | Status | Notes |
|----------|---------|--------|-------|
| AlmaLinux | 8 | ✅ Supported | EPEL required for FRR |
| AlmaLinux | 9 | ✅ Supported | Recommended |
| AlmaLinux | 10 | ✅ Supported | Latest |
| Rocky Linux | 8, 9 | ✅ Supported | Same as AlmaLinux |
| RHEL | 8, 9 | ✅ Supported | Requires active subscription |
| Ubuntu | 22.04+ | ✅ Supported | |
| Debian | 12+ | ✅ Supported | |

---

## Security Considerations

1. **Run as Root:** LibraFlux requires root privileges to manage IPVS and network interfaces
2. **Firewall:** Configure firewall rules to restrict access to management interfaces
3. **Metrics Endpoint:** By default, Prometheus metrics are exposed on all interfaces. Restrict with:
   ```yaml
   observability:
     metrics:
       prometheus:
         bind: 127.0.0.1  # Localhost only
   ```
4. **Configuration Files:** Protect configuration files from unauthorized access:
   ```bash
   sudo chmod 600 /etc/lbctl/config.yaml
   sudo chown root:root /etc/lbctl/config.yaml
   ```

---

## Performance Tuning

For high-throughput scenarios, consider tuning:

### Connection Tracking

```bash
# Increase conntrack table size
sudo sysctl -w net.netfilter.nf_conntrack_max=2097152
sudo sysctl -w net.ipv4.vs.conn_tab_bits=13
```

### IPVS Timeouts

```bash
# Adjust timeouts for your workload
sudo sysctl -w net.ipv4.vs.timeout_tcp=300
sudo sysctl -w net.ipv4.vs.timeout_tcpfin=30
sudo sysctl -w net.ipv4.vs.timeout_udp=60
```

### Reconciliation Interval

Edit `/etc/lbctl/config.yaml`:

```yaml
daemon:
  reconcile_interval_ms: 500  # Faster convergence (default: 1000)
```

---

## Additional Resources

- **Project Repository:** https://github.com/malindarathnayake/LibraFlux
- **Documentation:** `Docs/` directory in repository
- **Specification:** `Docs/spec.md`
- **Progress Tracking:** `Docs/PROGRESS.md`
- **Testing Harness:** `Docs/testing-harness.md`

---

## Support

For issues, questions, or contributions:
- Open an issue on GitHub
- Review `Docs/engineering-standards.md` for contribution guidelines
- Check `testing-notes.md` for testing procedures

---

**Last Updated:** December 2025

