# LibraFlux Quick Start Guide

## One-Line Installation

### AlmaLinux 8, 9, 10 (Recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash
```

### With Options

```bash
# Skip FRR (if using keepalived instead)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --skip-frr

# Preview changes without applying (dry-run)
curl -fsSL https://raw.githubusercontent.com/malindarathnayake/LibraFlux/main/Deployment/install.sh | sudo bash -s -- --dry-run
```

---

## Post-Install Configuration (3 Minutes)

### 1. Edit Main Config

```bash
sudo vi /etc/lbctl/config.yaml
```

**Change these values:**

```yaml
node:
  name: lb-node-a        # ← Unique name for this node
  role: primary          # ← primary or secondary

network:
  frontend:
    interface: eth0      # ← Your frontend interface
    vip: 192.168.1.100   # ← Your virtual IP
    cidr: 24             # ← Network prefix
  backend:
    interface: eth1      # ← Your backend interface
```

### 2. Create Service Definition

```bash
sudo vi /etc/lbctl/config.d/web-service.yaml
```

**Example:**

```yaml
services:
  - name: web-cluster
    vip: 192.168.1.100
    port: 80
    protocol: tcp
    scheduler: rr
    backends:
      - ip: 192.168.1.10
        port: 80
        weight: 100
      - ip: 192.168.1.11
        port: 80
        weight: 100
    health_check:
      type: tcp
      interval_ms: 5000
      timeout_ms: 2000
      retries: 3
```

### 3. Validate & Apply

```bash
# Validate configuration
sudo lbctl validate --config /etc/lbctl/config.yaml

# Test one-shot application
sudo lbctl apply --config /etc/lbctl/config.yaml

# Verify IPVS state
sudo ipvsadm -Ln
```

### 4. Enable Daemon Mode

```bash
sudo systemctl enable lbctl
sudo systemctl start lbctl
sudo systemctl status lbctl
```

---

## Essential Commands

### Service Management

```bash
# Start service
sudo systemctl start lbctl

# Stop service
sudo systemctl stop lbctl

# Restart service
sudo systemctl restart lbctl

# View status
sudo systemctl status lbctl

# View logs
sudo journalctl -u lbctl -f
```

### Configuration

```bash
# Validate config
sudo lbctl validate --config /etc/lbctl/config.yaml

# Apply config (one-shot)
sudo lbctl apply --config /etc/lbctl/config.yaml

# Run daemon mode
sudo lbctl apply --daemon --config /etc/lbctl/config.yaml

# Show status
sudo lbctl status

# Run diagnostics
sudo lbctl doctor
```

### IPVS Inspection

```bash
# List all virtual services
sudo ipvsadm -Ln

# Show statistics
sudo ipvsadm -Ln --stats

# Show rate information
sudo ipvsadm -Ln --rate

# Clear all rules (careful!)
sudo ipvsadm -C
```

### Monitoring

```bash
# Check metrics endpoint
curl http://localhost:9090/metrics

# Key metrics:
# - lbctl_health_backend_healthy
# - lbctl_health_backend_weight
# - lbctl_reconcile_runs_total
# - lbctl_vip_is_owner
```

---

## High Availability Setup

### Primary Node

```yaml
node:
  name: lb-primary
  role: primary

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
```

### Secondary Node

```yaml
node:
  name: lb-secondary
  role: secondary

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
```

**Important:** Both nodes need identical service definitions!

---

## Troubleshooting

### Check VIP Presence

```bash
ip addr show | grep -A2 "inet.*192.168.1.100"
```

### Check VRRP Status

```bash
sudo vtysh -c "show vrrp"
```

### Check IPVS Modules

```bash
lsmod | grep ip_vs
```

### Check IP Forwarding

```bash
cat /proc/sys/net/ipv4/ip_forward  # Should be 1
```

### Test Backend Connectivity

```bash
# TCP connection test
nc -zv 192.168.1.10 80

# HTTP test
curl -v http://192.168.1.10:80
```

---

## Operating Modes

### Direct Return (DR) Mode - High Performance

**Config:**

```yaml
mode: dr
```

**Backend Setup (on each backend server):**

```bash
# Add VIP to loopback
sudo ip addr add 192.168.1.100/32 dev lo

# Suppress ARP
sudo sysctl -w net.ipv4.conf.lo.arp_ignore=1
sudo sysctl -w net.ipv4.conf.lo.arp_announce=2
```

**When to use:** High-throughput scenarios, same L2 network

### NAT Mode - Simple Setup

**Config:**

```yaml
mode: nat
```

**No backend configuration needed!**

**When to use:** Different networks, Docker testing, simpler setup

---

## Firewall Configuration

```bash
# Allow VRRP
sudo firewall-cmd --permanent --add-protocol=vrrp

# Allow service traffic (example: HTTP/HTTPS)
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https

# Allow Prometheus metrics (optional)
sudo firewall-cmd --permanent --add-port=9090/tcp

# Reload
sudo firewall-cmd --reload
```

---

## Uninstall

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

# Reload systemd
sudo systemctl daemon-reload
```

---

## Getting Help

- **Full Documentation:** `Deployment/deployment-notes.md`
- **Project Docs:** `Docs/` directory
- **GitHub Issues:** https://github.com/malindarathnayake/LibraFlux/issues

---

**Tip:** Start with NAT mode for testing, then switch to DR mode for production high-throughput scenarios.

