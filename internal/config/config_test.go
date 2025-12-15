package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveEnvVars(t *testing.T) {
	os.Setenv("TEST_HOST", "localhost")
	os.Setenv("TEST_PORT", "8080")
	defer os.Unsetenv("TEST_HOST")
	defer os.Unsetenv("TEST_PORT")

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "basic substitution",
			input: "host: ${TEST_HOST}",
			want:  "host: localhost",
		},
		{
			name:  "multiple substitution",
			input: "url: http://${TEST_HOST}:${TEST_PORT}/api",
			want:  "url: http://localhost:8080/api",
		},
		{
			name:    "missing variable",
			input:   "host: ${MISSING_VAR}",
			wantErr: true,
		},
		{
			name:  "no substitution",
			input: "host: localhost",
			want:  "host: localhost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveEnvVars([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveEnvVars() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && string(got) != tt.want {
				t.Errorf("ResolveEnvVars() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create main config
	mainConfig := `
mode: dr
node:
  name: test-node
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
observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: false
      host: ""
      port: 0
      protocol: ""
      facility: ""
  metrics:
    influxdb:
      enabled: false
      url: ""
      token: ""
      org: ""
      bucket: ""
      push_interval_seconds: 0
    prometheus:
      enabled: true
      port: 9090
      path: /metrics
system:
  state_dir: /var/lib/lbctl
  frr_config: /etc/frr/frr.conf
  sysctl_file: /etc/sysctl.d/99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
include: "conf.d/*.yaml"
`
	mainPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Create included config
	os.Mkdir(filepath.Join(tmpDir, "conf.d"), 0755)
	serviceConfig := `
services:
  - name: test-service
    protocol: tcp
    ports: [80]
    scheduler: rr
    backends:
      - address: 10.0.0.1
        port: 80
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 80
      interval_ms: 1000
      timeout_ms: 1000
      fail_after: 3
      recover_after: 3
`
	servicePath := filepath.Join(tmpDir, "conf.d", "service.yaml")
	if err := os.WriteFile(servicePath, []byte(serviceConfig), 0644); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := LoadConfig(mainPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	// Verify
	if cfg.Node.Name != "test-node" {
		t.Errorf("expected node name test-node, got %s", cfg.Node.Name)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "test-service" {
		t.Errorf("expected service name test-service, got %s", cfg.Services[0].Name)
	}
}

func TestLoadConfigEnvResolutionNumeric(t *testing.T) {
	tmpDir := t.TempDir()

	os.Setenv("GELF_PORT", "12201")
	defer os.Unsetenv("GELF_PORT")

	mainConfig := `
mode: dr
node:
  name: test-node
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
observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: true
      host: graylog.example.com
      port: ${GELF_PORT}
      protocol: udp
      facility: lbctl
  metrics:
    influxdb:
      enabled: false
      url: ""
      token: ""
      org: ""
      bucket: ""
      push_interval_seconds: 0
    prometheus:
      enabled: false
      port: 0
      path: ""
system:
  state_dir: /var/lib/lbctl
  frr_config: /etc/frr/frr.conf
  sysctl_file: /etc/sysctl.d/99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
`
	mainPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(mainPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Observability.Logging.GELF.Port != 12201 {
		t.Fatalf("expected GELF port 12201, got %d", cfg.Observability.Logging.GELF.Port)
	}
}

func TestLoadConfigMergeOrder(t *testing.T) {
	tmpDir := t.TempDir()

	mainConfig := `
mode: dr
node:
  name: test-node
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
observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: false
      host: ""
      port: 0
      protocol: ""
      facility: ""
  metrics:
    influxdb:
      enabled: false
      url: ""
      token: ""
      org: ""
      bucket: ""
      push_interval_seconds: 0
    prometheus:
      enabled: false
      port: 0
      path: ""
system:
  state_dir: /var/lib/lbctl
  frr_config: /etc/frr/frr.conf
  sysctl_file: /etc/sysctl.d/99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
include: "conf.d/*.yaml"
`
	mainPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "conf.d"), 0755); err != nil {
		t.Fatal(err)
	}

	a := `
services:
  - name: a
    protocol: tcp
    ports: [80]
    scheduler: rr
    backends:
      - address: 10.0.0.1
        port: 80
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 80
      interval_ms: 1000
      timeout_ms: 300
      fail_after: 3
      recover_after: 2
`
	b := `
services:
  - name: b
    protocol: tcp
    ports: [81]
    scheduler: rr
    backends:
      - address: 10.0.0.2
        port: 81
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 81
      interval_ms: 1000
      timeout_ms: 300
      fail_after: 3
      recover_after: 2
`
	if err := os.WriteFile(filepath.Join(tmpDir, "conf.d", "a.yaml"), []byte(a), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "conf.d", "b.yaml"), []byte(b), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(mainPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := len(cfg.Services); got != 2 {
		t.Fatalf("expected 2 services, got %d", got)
	}
	if cfg.Services[0].Name != "a" || cfg.Services[1].Name != "b" {
		t.Fatalf("expected order [a, b], got [%s, %s]", cfg.Services[0].Name, cfg.Services[1].Name)
	}
}

func TestLoadConfigRejectsMainServices(t *testing.T) {
	tmpDir := t.TempDir()

	mainConfig := `
mode: dr
node:
  name: test-node
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
services:
  - name: should-not-be-here
    protocol: tcp
    ports: [80]
    scheduler: rr
    backends:
      - address: 10.0.0.1
        port: 80
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 80
      interval_ms: 1000
      timeout_ms: 300
      fail_after: 3
      recover_after: 2
`
	mainPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(mainPath); err == nil {
		t.Fatalf("expected LoadConfig to fail when main config defines services")
	}
}

func TestLoadConfigRejectsConfigDGlobals(t *testing.T) {
	tmpDir := t.TempDir()

	mainConfig := `
mode: dr
node:
  name: test-node
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
observability:
  logging:
    console:
      enabled: true
      level: info
    gelf:
      enabled: false
      host: ""
      port: 0
      protocol: ""
      facility: ""
  metrics:
    influxdb:
      enabled: false
      url: ""
      token: ""
      org: ""
      bucket: ""
      push_interval_seconds: 0
    prometheus:
      enabled: false
      port: 0
      path: ""
system:
  state_dir: /var/lib/lbctl
  frr_config: /etc/frr/frr.conf
  sysctl_file: /etc/sysctl.d/99-lbctl.conf
  tuning_profile: balanced
  lock_idle_timeout_minutes: 10
include: "conf.d/*.yaml"
`
	mainPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(mainPath, []byte(mainConfig), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(tmpDir, "conf.d"), 0755); err != nil {
		t.Fatal(err)
	}

	badServiceConfig := `
node:
  name: not-allowed
services:
  - name: svc
    protocol: tcp
    ports: [80]
    scheduler: rr
    backends:
      - address: 10.0.0.1
        port: 80
        weight: 1
    health:
      enabled: true
      type: tcp
      port: 80
      interval_ms: 1000
      timeout_ms: 300
      fail_after: 3
      recover_after: 2
`
	if err := os.WriteFile(filepath.Join(tmpDir, "conf.d", "bad.yaml"), []byte(badServiceConfig), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := LoadConfig(mainPath); err == nil {
		t.Fatalf("expected LoadConfig to fail when config.d file contains globals")
	}
}

func TestValidate(t *testing.T) {
	validConfig := &Config{
		Mode: "dr",
		Node: NodeConfig{Name: "valid-node", Role: "primary"},
		Network: NetworkConfig{
			Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
			Backend:  InterfaceConfig{Interface: "eth1"},
		},
		VRRP: VRRPConfig{VRID: 10, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
		Services: []Service{
			{
				Name:      "valid-service",
				Protocol:  "tcp",
				Ports:     []int{80},
				Scheduler: "rr",
				Backends: []Backend{
					{Address: "10.0.0.1", Port: 80, Weight: 1},
				},
				Health: HealthCheck{
					Enabled:      true,
					Type:         "tcp",
					Port:         80,
					IntervalMS:   1000,
					TimeoutMS:    300,
					FailAfter:    3,
					RecoverAfter: 2,
				},
			},
		},
	}

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "valid config",
			config:  validConfig,
			wantErr: false,
		},
		{
			name: "invalid node name",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "invalid name", Role: "primary"}, // Space not allowed
			},
			wantErr: true,
		},
		{
			name: "invalid vip",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "invalid-ip", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
			},
			wantErr: true,
		},
		{
			name: "duplicate service",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
				Services: []Service{
					{
						Name: "svc1", Protocol: "tcp", Ports: []int{80}, Scheduler: "rr",
						Backends: []Backend{{Address: "1.1.1.1", Port: 80, Weight: 1}},
						Health:   HealthCheck{Enabled: true, Type: "tcp", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
					{
						Name: "svc1", Protocol: "tcp", Ports: []int{80}, Scheduler: "rr",
						Backends: []Backend{{Address: "1.1.1.1", Port: 80, Weight: 1}},
						Health:   HealthCheck{Enabled: true, Type: "tcp", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "injection chars",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node;rm -rf", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
			},
			wantErr: true,
		},
		{
			name: "invalid scheduler",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
				Services: []Service{
					{
						Name:      "svc",
						Protocol:  "tcp",
						Ports:     []int{80},
						Scheduler: "lc",
						Backends:  []Backend{{Address: "10.0.0.1", Port: 80, Weight: 1}},
						Health:    HealthCheck{Enabled: true, Type: "tcp", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid port range",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
				Services: []Service{
					{
						Name:       "svc",
						Protocol:   "tcp",
						PortRanges: []PortRange{{Start: 100, End: 50}},
						Scheduler:  "rr",
						Backends:   []Backend{{Address: "10.0.0.1", Port: 0, Weight: 1}},
						Health:     HealthCheck{Enabled: true, Type: "tcp", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid backend weight",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
				Services: []Service{
					{
						Name:      "svc",
						Protocol:  "tcp",
						Ports:     []int{80},
						Scheduler: "rr",
						Backends:  []Backend{{Address: "10.0.0.1", Port: 80, Weight: 0}},
						Health:    HealthCheck{Enabled: true, Type: "tcp", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid health type",
			config: &Config{
				Mode: "dr",
				Node: NodeConfig{Name: "node", Role: "primary"},
				Network: NetworkConfig{
					Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
					Backend:  InterfaceConfig{Interface: "eth1"},
				},
				VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
				Services: []Service{
					{
						Name:      "svc",
						Protocol:  "tcp",
						Ports:     []int{80},
						Scheduler: "rr",
						Backends:  []Backend{{Address: "10.0.0.1", Port: 80, Weight: 1}},
						Health:    HealthCheck{Enabled: true, Type: "http", Port: 80, IntervalMS: 1000, TimeoutMS: 300, FailAfter: 3, RecoverAfter: 2},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidate_DaemonDefaultsAndBounds(t *testing.T) {
	base := &Config{
		Mode: "dr",
		Node: NodeConfig{Name: "node", Role: "primary"},
		Network: NetworkConfig{
			Frontend: InterfaceConfig{Interface: "eth0", VIP: "192.168.1.1", CIDR: 24},
			Backend:  InterfaceConfig{Interface: "eth1"},
		},
		VRRP: VRRPConfig{VRID: 1, PriorityPrimary: 150, PrioritySecondary: 100, AdvertIntervalMS: 1000},
	}

	t.Run("defaults reconcile interval when unset", func(t *testing.T) {
		cfg := *base
		if err := Validate(&cfg); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if cfg.Daemon.ReconcileIntervalMS != 1000 {
			t.Fatalf("expected default reconcile_interval_ms=1000, got %d", cfg.Daemon.ReconcileIntervalMS)
		}
	})

	t.Run("rejects reconcile interval below minimum", func(t *testing.T) {
		cfg := *base
		cfg.Daemon.ReconcileIntervalMS = 99
		if err := Validate(&cfg); err == nil {
			t.Fatalf("expected error")
		}
	})

	t.Run("defaults state_cache.ttl_ms when enabled and unset", func(t *testing.T) {
		cfg := *base
		cfg.Daemon.StateCache.Enabled = true
		cfg.Daemon.StateCache.TTLMS = 0
		if err := Validate(&cfg); err != nil {
			t.Fatalf("Validate() error = %v", err)
		}
		if cfg.Daemon.StateCache.TTLMS != 500 {
			t.Fatalf("expected default state_cache.ttl_ms=500, got %d", cfg.Daemon.StateCache.TTLMS)
		}
	})

	t.Run("rejects negative ttl_ms", func(t *testing.T) {
		cfg := *base
		cfg.Daemon.StateCache.Enabled = true
		cfg.Daemon.StateCache.TTLMS = -1
		if err := Validate(&cfg); err == nil {
			t.Fatalf("expected error")
		}
	})
}

func TestWriteServiceConfig(t *testing.T) {
	tmpDir := t.TempDir()

	outDir := filepath.Join(tmpDir, "config.d")

	svc := Service{
		Name:      "test-write",
		Protocol:  "tcp",
		Ports:     []int{80},
		Scheduler: "rr",
		Backends: []Backend{
			{Address: "10.0.0.1", Port: 80, Weight: 1},
		},
		Health: HealthCheck{
			Enabled:      true,
			Type:         "tcp",
			Port:         80,
			IntervalMS:   1000,
			TimeoutMS:    300,
			FailAfter:    3,
			RecoverAfter: 2,
		},
	}

	if err := WriteServiceConfig(outDir, svc); err != nil {
		t.Fatalf("WriteServiceConfig() error = %v", err)
	}

	// Verify file exists and content
	path := filepath.Join(outDir, "test-write.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Quick check content
	content := string(data)
	if !strings.Contains(content, "name: test-write") {
		t.Error("file content missing name")
	}
}
