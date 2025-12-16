# Testing Notes

Local development and testing guide for LibraFlux/lbctl.

## ⚠️ Container Limitations

**lbctl cannot function in containers** - IPVS is kernel-level. Containers can only test:

| What | Container | Real Linux Host |
|------|-----------|-----------------|
| Unit tests (code logic) | ✓ | ✓ |
| Config parsing/validation | ✓ | ✓ |
| Deploy script (file installation) | ✓ | ✓ |
| **IPVS service creation** | ✗ | ✓ |
| **Health checks (real TCP)** | ✗ | ✓ |
| **VIP management** | ✗ | ✓ |
| **Daemon mode** | ✗ | ✓ |

For functional testing, deploy to a real Linux VM/host (AlmaLinux 9, Rocky 9, Ubuntu 22.04+).

---

## Quick Reference

```powershell
# Windows (PowerShell) - all commands use Docker, no local Go required
.\build.ps1 test              # Run all tests
.\build.ps1 build             # Build binary to ./bin/lbctl
.\build.ps1 deploy-test       # Test deploy script (dry-run)
.\build.ps1 deploy-test-full  # Full deploy test with binary
.\build.ps1 interact          # Interactive shell in test environment
.\build.ps1                   # Run test + build (default)
```

```bash
# Linux/macOS (with make)
make test                     # Run tests locally
make docker-test              # Run tests in AlmaLinux container
make docker-artifact          # Build binary via Docker → ./lbctl
make deploy-test              # Test deploy script (dry-run)
make deploy-test-full         # Full deploy test
```

---

## Testing Workflow

### 1. Run Tests

Tests run inside an AlmaLinux 9 container (matches production target):

```powershell
.\build.ps1 test
```

This builds a test image and runs `go test -v ./...` inside it.

### 2. Build Binary

Build the Linux binary (outputs to `./bin/lbctl`, gitignored):

```powershell
.\build.ps1 build
```

The binary is built inside Docker using a multi-stage build. Copy it to your Linux host for deployment.

### 3. Test Deploy Script

The deploy script (`scripts/deploy.sh`) installs:
- IPVS kernel modules
- Sysctl configuration
- FRR with VRRP enabled
- lbctl binary + systemd service

**Dry-run** (no changes, shows what would happen):

```powershell
.\build.ps1 deploy-test
```

**Full install test** (runs in privileged container):

```powershell
.\build.ps1 deploy-test-full
```

### 4. Interactive Testing

Drop into an interactive shell with the built binary and deploy environment:

```powershell
.\build.ps1 interact
```

Inside the container:

```bash
# Run the deploy script
/scripts/deploy.sh --skip-frr-start

# Test the binary (before deploy)
/app/lbctl --help
/app/lbctl doctor

# After deploy, binary is installed to PATH
lbctl --help
lbctl validate --config /etc/lbctl/config.yaml

# Explore the installed config
cat /etc/lbctl/config.yaml
ls /etc/lbctl/config.d/

# Check FRR config
cat /etc/frr/daemons | grep vrrp

# Exit
exit
```

---

## Deploy Script Options

```bash
./scripts/deploy.sh [OPTIONS]

Options:
  --dry-run         Print commands without executing
  --skip-frr        Skip FRR installation (use if you have keepalived)
  --skip-frr-start  Install FRR but don't start it (for container testing)
  -h, --help        Show help

Environment variables:
  LBCTL_BIN         Binary install path (default: /usr/local/bin/lbctl)
  LBCTL_CONFIG_DIR  Config directory (default: /etc/lbctl)
```

---

## CI Pipeline

GitHub Actions workflow (`.github/workflows/ci.yml`) runs on push/PR:

1. **test** - Run all tests in container
2. **build** - Build binary, upload as artifact
3. **deploy-script-test** - Validate deploy.sh works

---

## Container Environment Notes

When running in Docker, some checks are automatically skipped:

| Check | Real Host | Container |
|-------|-----------|-----------|
| Kernel modules (ip_vs) | ✓ Verified | ⊘ Skipped |
| IP forwarding | ✓ Verified | ⊘ Skipped |
| ipvsadm | ✓ Verified | ✓ Verified |
| FRR vtysh | ✓ Verified | ✓ Verified |
| lbctl binary | ✓ Verified | ⊘ Skipped |
| Config files | ✓ Verified | ✓ Verified |
| Systemd service | ✓ Verified | ✓ Verified |

The deploy script detects container environment via `/.dockerenv` or cgroup inspection.

---

## Files

| File | Purpose |
|------|---------|
| `build.ps1` | Windows PowerShell build script |
| `Makefile` | Linux/macOS build targets |
| `Dockerfile` | Main multi-stage build (test, build, runtime) |
| `scripts/Dockerfile.deploy-test` | Deploy script test environment |
| `scripts/deploy.sh` | Production deployment script |
| `.github/workflows/ci.yml` | GitHub Actions CI pipeline |

---

## Functional Testing (Real Host)

For actual IPVS functionality, deploy to a Linux VM:

```bash
# On AlmaLinux/Rocky 9 VM
# 1. Copy binary and scripts
scp ./bin/lbctl user@vm:/tmp/
scp -r ./scripts ./dist user@vm:/tmp/

# 2. SSH and run deploy
ssh user@vm
cd /tmp
sudo ./scripts/deploy.sh

# 3. Test the binary
sudo lbctl doctor
sudo lbctl validate --config /etc/lbctl/config.yaml

# 4. Create a test service and check IPVS
sudo lbctl apply --config /etc/lbctl/config.yaml
sudo ipvsadm -Ln

# 5. Run daemon mode
sudo systemctl start lbctl
sudo journalctl -fu lbctl
```

### Minimal Test VM Setup

```bash
# Vagrant (quick local VM)
vagrant init almalinux/9
vagrant up
vagrant ssh

# Or use any cloud VM with AlmaLinux 9 / Rocky 9 / Ubuntu 22.04+
```

---

## Troubleshooting

### "make is not recognized" (Windows)

Use `.\build.ps1` instead - it wraps all Docker commands.

### "-race requires cgo"

Race detector needs CGO/gcc. Container tests run without `-race` flag.

### "Module not loaded: ip_vs"

Expected in containers. Kernel modules are host-level, not container-level.

### "frr-pythontools not available"

Optional package, not available in all repos. FRR still works without it.

