# lbctl Docker Testing Harness (Guided Checklist)

This document helps you build and run a Docker-based test harness for `lbctl`.
It is intentionally broken into small tasks with built-in checks and “operator questions” so you can adapt to environments where Docker cannot fully emulate production (e.g., VRRP/L2 failover).

## What this harness can and cannot prove

**What you can test well with Docker**
- Config parsing/validation and regression unit tests (all phases in `Docs/PROGRESS.md`).
- IPVS programming for **TCP and UDP** services (requires privileged container + CAP_NET_ADMIN).
- L4 forwarding behavior in an isolated Docker network (recommended: `mode: nat`).

**What Docker is not a great fit for**
- True VIP failover on a real L2 network (VRRP advertisements, ARP behavior across switches).
- Real “production-like” DR mode semantics unless you also configure backends to own the VIP and handle ARP suppression.

When Docker can’t replicate a behavior, this doc calls out operator-driven alternatives.

---

## Operator Questions (answer before you start)

1) **Where are you running this?**
- Linux host
- Windows + WSL2
- macOS (Docker Desktop)
- CI runner (GitHub Actions/GitLab/etc.)
- Kubernetes node (DaemonSet testing)

2) **Can you run privileged containers?**
- If **no** (rootless Docker / restricted CI), you are limited to unit tests (`make docker-test`).

3) **Do you need to validate data-plane forwarding or just control-plane behavior?**
- Control-plane only: unit tests + `lbctl validate`.
- Data-plane: run the “Functional NAT Harness”.

4) **Do you need VRRP/VIP failover testing?**
- If **yes**, plan for an operator-assisted lab (two nodes on same L2, FRR/keepalived running, packet capture).

---

## Task 0 — Preflight checks

**Goal:** confirm your environment can run the harness.

### 0.1 Check Docker availability
- Command: `docker version`
- Check: prints both Client and Server sections.

### 0.2 Check privileged container capability
- Command: `docker run --rm --privileged alpine:3.20 sh -c 'id && ip link show >/dev/null'`
- Check: exits `0`.
- If fail: you’re likely on rootless Docker or a restricted environment; skip to “Unit Tests Only”.

### 0.3 Check Linux kernel IPVS support
You need `/proc/net/ip_vs` to exist in the network namespace where `lbctl` runs.

- Command (host): `test -e /proc/net/ip_vs || echo "missing"`
- Check: should **not** print `missing` on a Linux host with IPVS enabled.

If `missing`:
- Operator action: load modules on the host (example): `sudo modprobe ip_vs ip_vs_rr ip_vs_wrr ip_vs_sh`.
- If you cannot load modules (restricted kernel), you can still run unit tests, but not functional dataplane tests.

---

## Task 1 — Unit Tests Only (safe, always recommended)

This corresponds to the “repo ✓ PASS” line in `Docs/PROGRESS.md`.

### 1.1 Run all tests in the AlmaLinux test image
- Command: `make docker-test`
- Check: exits `0`.

### 1.2 Phase-by-phase checkpoints (Docker)
These mirror the checkpoints in `Docs/PROGRESS.md` but run in the container.

- Build the test image: `docker build --target lbctl-test -t lbctl-test .`

Then run:
- Phase 1 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/config ./internal/observability -v`
  - Check: exits `0`.
- Phase 2 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/system -v`
  - Check: exits `0`.
- Phase 3 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/ipvs ./internal/health -v`
  - Check: exits `0`.
- Phase 4 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/observability -v`
  - Check: exits `0`.
- Phase 5 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/daemon -v`
  - Check: exits `0`.
- Phase 6 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./internal/shell -v`
  - Check: exits `0`.
- Phase 7 checkpoint:
  - Command: `docker run --rm lbctl-test go test ./cmd/... -v`
  - Check: exits `0`.
- Phase 8/9 repo checkpoint:
  - Command: `docker run --rm lbctl-test make test`
  - Check: exits `0`.

If these pass, your control-plane logic and unit behavior are stable.

---

## Task 2 — Functional NAT Harness (Docker network, TCP+UDP)

**Goal:** prove that `lbctl` can program IPVS and forward real TCP+UDP traffic to backends.

This harness is **intentionally NAT mode** because it is the easiest to validate inside Docker without requiring special backend VIP/ARP setup.

### 2.1 Choose a test subnet and VIP
Answer:
- `SUBNET` (example): `172.28.0.0/16`
- `VIP` (example): `172.28.0.250`
- Backend IPs (examples): `172.28.0.11`, `172.28.0.12`

Check:
- Ensure the chosen subnet does not overlap your existing Docker networks: `docker network ls` then inspect as needed.

### 2.2 Create an isolated Docker network
- Command: `docker network create --subnet 172.28.0.0/16 lbctl-harness`
- Check: `docker network inspect lbctl-harness` shows the subnet.

### 2.3 Start TCP and UDP backend servers
You need two backends that respond distinctly.

Option A (recommended): use `alpine` + `socat`.

Backend 1:
- Command:
  - `docker run -d --name be1 --network lbctl-harness --ip 172.28.0.11 alpine:3.20 sh -lc 'apk add --no-cache socat >/dev/null && \
      socat -T1 -v TCP-LISTEN:8080,fork,reuseaddr SYSTEM:"echo backend1-tcp" & \
      socat -T1 -v UDP-RECVFROM:8081,fork,reuseaddr SYSTEM:"echo backend1-udp" && wait'`
- Check: `docker logs be1 | head` shows socat startup; container stays running.

Backend 2:
- Command:
  - `docker run -d --name be2 --network lbctl-harness --ip 172.28.0.12 alpine:3.20 sh -lc 'apk add --no-cache socat >/dev/null && \
      socat -T1 -v TCP-LISTEN:8080,fork,reuseaddr SYSTEM:"echo backend2-tcp" & \
      socat -T1 -v UDP-RECVFROM:8081,fork,reuseaddr SYSTEM:"echo backend2-udp" && wait'`
- Check: `docker logs be2 | head`.

If UDP responses don’t show up in your environment:
- Operator note: some tooling combinations drop replies; you can still validate UDP balancing via IPVS stats (Task 2.8).

### 2.4 Create harness config (operator creates local files)
Create a temporary harness directory on your host:
- `mkdir -p ./tmp/harness/etc/lbctl/config.d`

Create `./tmp/harness/etc/lbctl/config.yaml`:
```yaml
mode: nat

node:
  name: docker-harness
  role: primary

network:
  frontend:
    interface: eth0
    vip: 172.28.0.250
    cidr: 16
  backend:
    interface: eth0

vrrp:
  vrid: 50
  priority_primary: 150
  priority_secondary: 100
  advert_interval_ms: 1000

include: /etc/lbctl/config.d/*.yaml

observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: false
  metrics:
    influxdb:
      enabled: false
    prometheus:
      enabled: false

system:
  state_dir: /var/lib/lbctl
  frr_config: /etc/frr/frr.conf
  sysctl_file: /etc/sysctl.d/99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
```

Create `./tmp/harness/etc/lbctl/config.d/services.yaml`:
```yaml
services:
  - name: tcp-demo
    protocol: tcp
    ports: [8080]
    port_ranges: []
    scheduler: rr
    backends:
      - address: 172.28.0.11
        port: 8080
        weight: 1
      - address: 172.28.0.12
        port: 8080
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 8080
      interval_ms: 500
      timeout_ms: 200
      fail_after: 2
      recover_after: 1

  - name: udp-demo
    protocol: udp
    ports: [8081]
    port_ranges: []
    scheduler: rr
    backends:
      - address: 172.28.0.11
        port: 8081
        weight: 1
      - address: 172.28.0.12
        port: 8081
        weight: 1
    health:
      enabled: false
      type: tcp
      port: 8081
      interval_ms: 1000
      timeout_ms: 200
      fail_after: 2
      recover_after: 1
```

Checks:
- `lbctl validate` should accept `protocol: tcp|udp`.
- Health checks are currently TCP-only; UDP services typically run with health disabled (or use a TCP health port).

### 2.5 Build the runtime image
- Command: `make docker-build`
- Check: image exists: `docker images | rg 'lbctl-alma' || true`

If you don’t have `make`:
- Command: `docker build --target lbctl-runtime -t lbctl-alma .`

### 2.6 Start the `lbctl` container (privileged)
Run it attached to the harness network with a fixed IP.

- Command:
  - `docker run -d --name lbctl --privileged --network lbctl-harness --ip 172.28.0.10 \
      -v "$PWD/tmp/harness/etc/lbctl:/etc/lbctl:ro" \
      lbctl-alma sleep infinity`

Checks:
- `docker exec lbctl sh -lc 'id && ip -br addr'` works.
- `docker exec lbctl sh -lc 'test -e /proc/net/ip_vs && echo ipvs-ok'` prints `ipvs-ok`.

### 2.7 Assign the VIP inside the `lbctl` container namespace
`lbctl` currently does not assign the VIP address itself; it expects the system (FRR/keepalived/operator) to manage it.
For the Docker harness, you can add it manually.

- Command:
  - `docker exec lbctl sh -lc 'ip addr add 172.28.0.250/32 dev lo && ip addr show dev lo | rg 172.28.0.250'`
- Check: VIP appears in `ip addr` output.

If this fails:
- Operator action: ensure the container is privileged and has CAP_NET_ADMIN.

### 2.8 Apply config and verify IPVS state
- Command (validate): `docker exec lbctl lbctl validate --config /etc/lbctl/config.yaml`
- Check: exits `0`.

- Command (one-shot apply): `docker exec lbctl lbctl apply --config /etc/lbctl/config.yaml`
- Check: exits `0`.

Verify IPVS state using procfs:
- Command: `docker exec lbctl sh -lc 'cat /proc/net/ip_vs || true'`
- Check: output contains services for `172.28.0.250:8080` (TCP) and `172.28.0.250:8081` (UDP).

### 2.9 Generate TCP traffic and observe balancing
- Command:
  - `docker run --rm --network lbctl-harness alpine:3.20 sh -lc 'apk add --no-cache netcat-openbsd >/dev/null && \
      for i in $(seq 1 10); do echo ping | nc -w1 172.28.0.250 8080; done | sort | uniq -c'`

Check:
- You should see both `backend1-tcp` and `backend2-tcp` appear across multiple requests.

### 2.10 Generate UDP traffic and observe balancing
Option A (response-based):
- Command:
  - `docker run --rm --network lbctl-harness alpine:3.20 sh -lc 'apk add --no-cache socat >/dev/null && \
      for i in $(seq 1 20); do echo ping | socat -T1 - UDP:172.28.0.250:8081; done | sort | uniq -c'`

Check:
- Ideally you see both `backend1-udp` and `backend2-udp`.

Option B (stats-based if replies are unreliable):
- Command: `docker exec lbctl sh -lc 'cat /proc/net/ip_vs_stats | head'`
- Check: counters should increase as you send packets.

### 2.11 Teardown
- Command:
  - `docker rm -f lbctl be1 be2 >/dev/null 2>&1 || true`
  - `docker network rm lbctl-harness >/dev/null 2>&1 || true`

---

## Task 3 — Operator-assisted edge cases

### Rootless Docker / restricted CI
Symptoms:
- `--privileged` fails or CAP_NET_ADMIN is blocked.

Action:
- Run unit tests only (`make docker-test`).
- For dataplane tests, move to a Linux VM/runner with privileged Docker.

### Docker Desktop (macOS/Windows)
Notes:
- “Host networking” is not the real host; it is the Linux VM used by Docker Desktop.
- The functional NAT harness (Task 2) can still work because it stays inside the Docker VM.

### DR mode testing
DR mode generally requires:
- VIP present on backends (typically on `lo`).
- ARP suppression / sysctls to avoid backends answering ARP for VIP.

Recommendation:
- Do DR validation in a real lab or Kubernetes with CNI/L2 visibility.

### VRRP failover testing
This requires at least:
- Two instances of `lbctl` (or two nodes) with FRR/keepalived configured.
- A shared L2 segment and packet capture (`tcpdump`) to validate VRRP advertisements.

Operator-driven approach:
- Use the Docker unit tests + functional NAT harness for baseline.
- Do VRRP on real nodes/VMs where you can observe VIP movement.

---

## Quick reference: `Docs/PROGRESS.md` Docker step testing

- Always-green gate: `make docker-test`
- Per-package gates (inside container):
  - `docker run --rm lbctl-test go test ./internal/config -v`
  - `docker run --rm lbctl-test go test ./internal/observability -v`
  - `docker run --rm lbctl-test go test ./internal/ipvs -v`
  - `docker run --rm lbctl-test go test ./internal/health -v`
  - `docker run --rm lbctl-test go test ./internal/system -v`
  - `docker run --rm lbctl-test go test ./internal/daemon -v`
  - `docker run --rm lbctl-test go test ./internal/shell -v`
  - `docker run --rm lbctl-test go test ./cmd/... -v`
