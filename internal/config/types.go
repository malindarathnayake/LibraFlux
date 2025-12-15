package config

// Config represents the global configuration
type Config struct {
	Mode          string        `yaml:"mode"`
	Node          NodeConfig    `yaml:"node"`
	Network       NetworkConfig `yaml:"network"`
	VRRP          VRRPConfig    `yaml:"vrrp"`
	Observability ObsConfig     `yaml:"observability"`
	System        SystemConfig  `yaml:"system"`
	Daemon        DaemonConfig  `yaml:"daemon"`
	Include       string        `yaml:"include"`
	Services      []Service     `yaml:"services"` // Merged from config.d
}

type NodeConfig struct {
	Name string `yaml:"name"`
	Role string `yaml:"role"`
}

type NetworkConfig struct {
	Frontend InterfaceConfig `yaml:"frontend"`
	Backend  InterfaceConfig `yaml:"backend"`
}

type InterfaceConfig struct {
	Interface string `yaml:"interface"`
	VIP       string `yaml:"vip,omitempty"`
	CIDR      int    `yaml:"cidr,omitempty"`
}

type VRRPConfig struct {
	VRID              int `yaml:"vrid"`
	PriorityPrimary   int `yaml:"priority_primary"`
	PrioritySecondary int `yaml:"priority_secondary"`
	AdvertIntervalMS  int `yaml:"advert_interval_ms"`
}

type ObsConfig struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
}

type LoggingConfig struct {
	Console ConsoleLogConfig `yaml:"console"`
	GELF    GELFLogConfig    `yaml:"gelf"`
}

type ConsoleLogConfig struct {
	Enabled bool   `yaml:"enabled"`
	Level   string `yaml:"level"`
}

type GELFLogConfig struct {
	Enabled  bool   `yaml:"enabled"`
	Host     string `yaml:"host"`
	Port     int    `yaml:"port"`
	Protocol string `yaml:"protocol"`
	Facility string `yaml:"facility"`
}

type MetricsConfig struct {
	InfluxDB   InfluxConfig   `yaml:"influxdb"`
	Prometheus PromConfig     `yaml:"prometheus"`
}

type InfluxConfig struct {
	Enabled             bool   `yaml:"enabled"`
	URL                 string `yaml:"url"`
	Token               string `yaml:"token"`
	Org                 string `yaml:"org"`
	Bucket              string `yaml:"bucket"`
	PushIntervalSeconds int    `yaml:"push_interval_seconds"`
}

type PromConfig struct {
	Enabled bool   `yaml:"enabled"`
	Port    int    `yaml:"port"`
	Path    string `yaml:"path"`
	Bind    string `yaml:"bind"` // Bind address (default: "" = all interfaces)
}

type SystemConfig struct {
	StateDir               string       `yaml:"state_dir"`
	FRRConfig              string       `yaml:"frr_config"`
	SysctlFile             string       `yaml:"sysctl_file"`
	TuningProfile          string       `yaml:"tuning_profile"`
	LockIdleTimeoutMinutes int          `yaml:"lock_idle_timeout_minutes"`
}

// DaemonConfig holds runtime daemon settings
type DaemonConfig struct {
	ReconcileIntervalMS int         `yaml:"reconcile_interval_ms"`
	StateCache          CacheConfig `yaml:"state_cache"`
}

// CacheConfig holds settings for the in-memory IPVS state cache
type CacheConfig struct {
	Enabled bool `yaml:"enabled"`
	TTLMS   int  `yaml:"ttl_ms"` // Cache TTL in milliseconds
}

// ServiceConfig is the root struct for service config files
type ServiceConfig struct {
	Services []Service `yaml:"services"`
}

type Service struct {
	Name       string        `yaml:"name"`
	Protocol   string        `yaml:"protocol"`
	Ports      []int         `yaml:"ports"`
	PortRanges []PortRange   `yaml:"port_ranges"`
	Scheduler  string        `yaml:"scheduler"`
	Backends   []Backend     `yaml:"backends"`
	Health     HealthCheck   `yaml:"health"`
}

type PortRange struct {
	Start int `yaml:"start"`
	End   int `yaml:"end"`
}

type Backend struct {
	Address string `yaml:"address"`
	Port    int    `yaml:"port"`
	Weight  int    `yaml:"weight"`
}

type HealthCheck struct {
	Enabled      bool   `yaml:"enabled"`
	Type         string `yaml:"type"`
	Port         int    `yaml:"port"`
	IntervalMS   int    `yaml:"interval_ms"`
	TimeoutMS    int    `yaml:"timeout_ms"`
	FailAfter    int    `yaml:"fail_after"`
	RecoverAfter int    `yaml:"recover_after"`
}
