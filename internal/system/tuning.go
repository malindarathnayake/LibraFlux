package system

// TuningProfile defines a set of sysctl values
type TuningProfile map[string]string

// Default profiles
var (
	ProfileMinimal = TuningProfile{
		"net.ipv4.vs.conn_tab_bits":       "12", // Default
		"net.core.somaxconn":              "4096",
		"net.ipv4.tcp_max_syn_backlog":    "4096",
	}

	ProfileBalanced = TuningProfile{
		"net.ipv4.vs.conn_tab_bits":       "18", // 256k entries
		"net.core.rmem_max":               "16777216",
		"net.core.wmem_max":               "16777216",
		"net.core.somaxconn":              "32768",
		"net.ipv4.tcp_max_syn_backlog":    "32768",
		"net.ipv4.tcp_tw_reuse":           "1",
	}

	ProfileAggressive = TuningProfile{
		"net.ipv4.vs.conn_tab_bits":       "20", // 1M entries
		"net.core.rmem_max":               "134217728",
		"net.core.wmem_max":               "134217728",
		"net.core.netdev_max_backlog":     "250000",
		"net.core.somaxconn":              "65535",
		"net.ipv4.tcp_max_syn_backlog":    "65535",
		"net.ipv4.tcp_tw_reuse":           "1",
		"net.ipv4.tcp_window_scaling":     "1",
	}
)

// GetTuningProfile returns the requested profile or Balanced if unknown
func GetTuningProfile(name string) TuningProfile {
	switch name {
	case "minimal":
		return ProfileMinimal
	case "aggressive":
		return ProfileAggressive
	case "balanced":
		return ProfileBalanced
	default:
		return ProfileBalanced
	}
}
