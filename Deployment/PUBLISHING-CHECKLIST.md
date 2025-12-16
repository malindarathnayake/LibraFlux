# Publishing Checklist for LibraFlux Deployment

## Pre-Publishing Tasks

### 1. Update GitHub Organization/Username

Replace `YOUR_ORG` with your actual GitHub organization/username in:

- [ ] `Deployment/install.sh`
  - [ ] Line 12: `GITHUB_REPO` variable
  - [ ] Line 566-580: Usage examples in help text
  
- [ ] `Deployment/deployment-notes.md`
  - [ ] Lines 7-15: Installation commands
  - [ ] Line 489: Additional Resources URL
  
- [ ] `Deployment/QUICK-START.md`
  - [ ] Lines 7-15: Installation commands
  - [ ] Line 283: GitHub Issues URL
  
- [ ] `Deployment/README.md`
  - [ ] Lines 11-26: Installation commands
  - [ ] All GitHub URLs throughout document

### 2. Create GitHub Release

- [ ] Build binaries for all architectures:
  ```bash
  # AMD64 (x86_64)
  GOOS=linux GOARCH=amd64 go build -o lbctl-linux-amd64 ./cmd/lbctl
  
  # ARM64 (aarch64)
  GOOS=linux GOARCH=arm64 go build -o lbctl-linux-arm64 ./cmd/lbctl
  ```

- [ ] Create a new release on GitHub:
  - [ ] Tag: `v1.0.0` (or appropriate version)
  - [ ] Title: `LibraFlux v1.0.0`
  - [ ] Description: Release notes
  
- [ ] Upload release assets:
  - [ ] `lbctl-linux-amd64`
  - [ ] `lbctl-linux-arm64`
  - [ ] Checksums file (optional but recommended)

### 3. Test Installation Script

Test on all supported platforms:

- [ ] **AlmaLinux 8**
  ```bash
  docker run -it --privileged almalinux:8 bash
  curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | bash
  ```

- [ ] **AlmaLinux 9**
  ```bash
  docker run -it --privileged almalinux:9 bash
  curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | bash
  ```

- [ ] **Rocky Linux 9**
  ```bash
  docker run -it --privileged rockylinux:9 bash
  curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | bash
  ```

- [ ] **Ubuntu 22.04**
  ```bash
  docker run -it --privileged ubuntu:22.04 bash
  curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | bash
  ```

### 4. Test Installation Options

- [ ] Test `--dry-run` mode
  ```bash
  curl -fsSL https://... | bash -s -- --dry-run
  ```

- [ ] Test `--skip-frr` option
  ```bash
  curl -fsSL https://... | bash -s -- --skip-frr
  ```

- [ ] Test `--skip-frr-start` option
  ```bash
  curl -fsSL https://... | bash -s -- --skip-frr-start
  ```

- [ ] Test custom paths
  ```bash
  LBCTL_BIN=/usr/bin/lbctl curl -fsSL https://... | bash
  ```

### 5. Verify Post-Installation

After installation, verify:

- [ ] Binary is executable
  ```bash
  lbctl --version
  ```

- [ ] Config files exist
  ```bash
  ls -la /etc/lbctl/config.yaml
  ls -la /etc/lbctl/config.d/
  ```

- [ ] Systemd service is installed
  ```bash
  systemctl status lbctl
  ```

- [ ] IPVS modules are loaded
  ```bash
  lsmod | grep ip_vs
  ```

- [ ] Sysctl settings are applied
  ```bash
  sysctl net.ipv4.ip_forward
  ```

- [ ] FRR is installed (if not skipped)
  ```bash
  vtysh --version
  ```

### 6. Update Main Project Documentation

- [ ] Update `README.md` to reference Deployment folder
  ```markdown
  ## Installation
  
  See [Deployment/](Deployment/) for installation instructions.
  
  ### Quick Install
  
  ```bash
  curl -fsSL https://raw.githubusercontent.com/YOUR_ORG/LibraFlux/main/Deployment/install.sh | sudo bash
  ```
  ```

- [ ] Update `Docs/PROGRESS.md` to mark deployment as complete

- [ ] Add deployment section to `Docs/spec.md` if needed

### 7. Create CI/CD Pipeline (Optional)

- [ ] Create `.github/workflows/release.yml` to automate releases
  - [ ] Build binaries on tag push
  - [ ] Create GitHub release
  - [ ] Upload artifacts
  - [ ] Generate checksums

Example workflow:
```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - uses: actions/setup-go@v4
        with:
          go-version: '1.21'
      
      - name: Build binaries
        run: |
          GOOS=linux GOARCH=amd64 go build -o lbctl-linux-amd64 ./cmd/lbctl
          GOOS=linux GOARCH=arm64 go build -o lbctl-linux-arm64 ./cmd/lbctl
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            lbctl-linux-amd64
            lbctl-linux-arm64
```

---

## Publishing Tasks

### 1. Commit and Push

- [ ] Commit all deployment files
  ```bash
  git add Deployment/
  git commit -m "Add production deployment scripts and documentation"
  git push origin main
  ```

### 2. Create and Push Tag

- [ ] Create version tag
  ```bash
  git tag -a v1.0.0 -m "Release v1.0.0"
  git push origin v1.0.0
  ```

### 3. Verify GitHub Release

- [ ] Check release page: `https://github.com/YOUR_ORG/LibraFlux/releases`
- [ ] Verify binaries are downloadable
- [ ] Test download URLs manually

### 4. Update Documentation Links

- [ ] Ensure all raw.githubusercontent.com links work
- [ ] Test installation command from documentation

---

## Post-Publishing Tasks

### 1. Announce Release

- [ ] Update project README with installation instructions
- [ ] Create announcement (if applicable)
- [ ] Update any external documentation

### 2. Monitor Issues

- [ ] Watch for installation issues
- [ ] Respond to bug reports
- [ ] Update documentation based on feedback

### 3. Create Examples

- [ ] Create example configurations for common use cases
- [ ] Add to `dist/config.d/` directory
- [ ] Document in `deployment-notes.md`

### 4. Performance Testing

- [ ] Test on production-like environment
- [ ] Verify high-availability failover
- [ ] Load test IPVS performance
- [ ] Monitor metrics during testing

---

## Rollback Plan

If issues are discovered after publishing:

### Option 1: Quick Fix

- [ ] Fix the issue
- [ ] Create new patch release (v1.0.1)
- [ ] Update documentation

### Option 2: Rollback Release

- [ ] Mark release as pre-release
- [ ] Add warning to release notes
- [ ] Create fixed release

### Option 3: Update Documentation

- [ ] Add known issues section
- [ ] Provide workarounds
- [ ] Update troubleshooting guide

---

## Testing Checklist

### Functional Tests

- [ ] Fresh installation on clean system
- [ ] Installation over existing installation (idempotency)
- [ ] Installation with `--skip-frr`
- [ ] Installation with custom paths
- [ ] Dry-run mode doesn't make changes
- [ ] Uninstallation removes all components

### Integration Tests

- [ ] Service starts successfully
- [ ] Configuration validation works
- [ ] IPVS rules are applied
- [ ] Health checks function
- [ ] Metrics endpoint responds
- [ ] VRRP failover works (2-node setup)

### Platform Tests

- [ ] AlmaLinux 8
- [ ] AlmaLinux 9
- [ ] AlmaLinux 10 (if available)
- [ ] Rocky Linux 8
- [ ] Rocky Linux 9
- [ ] RHEL 8 (if access available)
- [ ] RHEL 9 (if access available)
- [ ] Ubuntu 22.04
- [ ] Ubuntu 24.04
- [ ] Debian 12

### Edge Cases

- [ ] Installation without internet access (should fail gracefully)
- [ ] Installation with missing dependencies
- [ ] Installation in container environment
- [ ] Installation on system with existing FRR
- [ ] Installation on system with existing IPVS rules

---

## Documentation Review

### Content Review

- [ ] All commands are correct and tested
- [ ] All file paths are accurate
- [ ] All URLs are valid
- [ ] Examples are complete and working
- [ ] Troubleshooting covers common issues

### Formatting Review

- [ ] Markdown renders correctly on GitHub
- [ ] Code blocks have correct syntax highlighting
- [ ] Tables are properly formatted
- [ ] Links work (internal and external)
- [ ] No typos or grammatical errors

### Consistency Review

- [ ] Terminology is consistent across all docs
- [ ] Command examples use consistent format
- [ ] File paths are consistent
- [ ] Version numbers match

---

## Security Review

- [ ] Script validates downloads (HTTPS only)
- [ ] No hardcoded credentials
- [ ] Proper file permissions set
- [ ] No unsafe shell operations
- [ ] Input validation for user-provided values
- [ ] Secure defaults in configuration

---

## Accessibility Review

- [ ] Documentation is clear for beginners
- [ ] Advanced options are documented
- [ ] Troubleshooting is comprehensive
- [ ] Error messages are helpful
- [ ] Examples cover common use cases

---

## Final Checks

- [ ] All checklist items completed
- [ ] Installation tested on at least 3 platforms
- [ ] Documentation reviewed by at least one other person
- [ ] No breaking changes from previous version (if applicable)
- [ ] Changelog updated
- [ ] Version numbers consistent across all files

---

## Sign-Off

- [ ] **Developer:** Installation script tested and working
- [ ] **Reviewer:** Documentation reviewed and approved
- [ ] **QA:** All platforms tested successfully
- [ ] **Release Manager:** Ready for production release

---

**Date:** _______________
**Version:** _______________
**Released By:** _______________

---

## Notes

Use this space for any additional notes or observations during the publishing process:

```
[Your notes here]
```

