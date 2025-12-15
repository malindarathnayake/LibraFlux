package shell

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

type ServiceMode struct {
	Service config.Service
}

func NewServiceMode(svc config.Service) (*ServiceMode, error) {
	if strings.TrimSpace(svc.Name) == "" {
		return nil, errors.New("service name required")
	}
	return &ServiceMode{Service: svc}, nil
}

func (m *ServiceMode) Handle(s *Shell, tokens []string) error {
	switch strings.ToLower(tokens[0]) {
	case "show":
		return m.show(s)
	case "protocol":
		if len(tokens) < 2 {
			return errors.New("usage: protocol <tcp|udp>")
		}
		m.Service.Protocol = strings.ToLower(tokens[1])
		return nil
	case "scheduler":
		if len(tokens) < 2 {
			return errors.New("usage: scheduler <rr|wrr|sh>")
		}
		m.Service.Scheduler = strings.ToLower(tokens[1])
		return nil
	case "ports":
		if len(tokens) < 2 {
			return errors.New("usage: ports <p1,p2,...>")
		}
		ports, err := parseCSVPorts(tokens[1])
		if err != nil {
			return err
		}
		m.Service.Ports = ports
		return nil
	case "port-range":
		if len(tokens) < 2 {
			return errors.New("usage: port-range <start-end>")
		}
		pr, err := parsePortRange(tokens[1])
		if err != nil {
			return err
		}
		m.Service.PortRanges = append(m.Service.PortRanges, pr)
		return nil
	case "backend":
		if len(tokens) < 2 {
			return errors.New("usage: backend <ip> [weight]")
		}
		ip := tokens[1]
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid ip: %s", ip)
		}
		weight := 1
		if len(tokens) >= 3 {
			w, err := strconv.Atoi(tokens[2])
			if err != nil {
				return fmt.Errorf("invalid weight: %w", err)
			}
			weight = w
		}
		m.Service.Backends = append(m.Service.Backends, config.Backend{
			Address: ip,
			Port:    0,
			Weight:  weight,
		})
		return nil
	case "no":
		if len(tokens) < 2 {
			return errors.New("usage: no <subcommand>")
		}
		switch strings.ToLower(tokens[1]) {
		case "backend":
			if len(tokens) < 3 {
				return errors.New("usage: no backend <ip>")
			}
			ip := tokens[2]
			var next []config.Backend
			for _, be := range m.Service.Backends {
				if be.Address != ip {
					next = append(next, be)
				}
			}
			m.Service.Backends = next
			return nil
		case "health":
			m.Service.Health = config.HealthCheck{Enabled: false, Type: "tcp"}
			return nil
		default:
			return fmt.Errorf("unknown no subcommand: %s", tokens[1])
		}
	case "health":
		return m.health(tokens[1:])
	default:
		return fmt.Errorf("unknown service command: %s", tokens[0])
	}
}

func (m *ServiceMode) show(s *Shell) error {
	fmt.Fprintf(s.out, "service %s\n", m.Service.Name)
	fmt.Fprintf(s.out, "  protocol %s\n", m.Service.Protocol)
	if len(m.Service.Ports) > 0 {
		var ps []string
		for _, p := range m.Service.Ports {
			ps = append(ps, strconv.Itoa(p))
		}
		sort.Strings(ps)
		fmt.Fprintf(s.out, "  ports %s\n", strings.Join(ps, ","))
	}
	for _, pr := range m.Service.PortRanges {
		fmt.Fprintf(s.out, "  port-range %d-%d\n", pr.Start, pr.End)
	}
	fmt.Fprintf(s.out, "  scheduler %s\n", m.Service.Scheduler)
	for _, be := range m.Service.Backends {
		fmt.Fprintf(s.out, "  backend %s weight %d\n", be.Address, be.Weight)
	}
	if m.Service.Health.Enabled {
		fmt.Fprintf(s.out, "  health tcp port %d interval %d timeout %d\n", m.Service.Health.Port, m.Service.Health.IntervalMS, m.Service.Health.TimeoutMS)
	}
	return nil
}

func (m *ServiceMode) health(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: health tcp port <p> interval <ms> timeout <ms>")
	}
	if strings.ToLower(args[0]) != "tcp" {
		return errors.New("only tcp health checks supported")
	}
	h := config.HealthCheck{
		Enabled:      true,
		Type:         "tcp",
		FailAfter:    3,
		RecoverAfter: 2,
	}

	i := 1
	for i < len(args) {
		switch strings.ToLower(args[i]) {
		case "port":
			i++
			if i >= len(args) {
				return errors.New("missing health port")
			}
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			h.Port = v
		case "interval":
			i++
			if i >= len(args) {
				return errors.New("missing health interval")
			}
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			h.IntervalMS = v
		case "timeout":
			i++
			if i >= len(args) {
				return errors.New("missing health timeout")
			}
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			h.TimeoutMS = v
		case "fail-after":
			i++
			if i >= len(args) {
				return errors.New("missing fail-after")
			}
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			h.FailAfter = v
		case "recover-after":
			i++
			if i >= len(args) {
				return errors.New("missing recover-after")
			}
			v, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			h.RecoverAfter = v
		default:
			return fmt.Errorf("unknown health field: %s", args[i])
		}
		i++
	}
	m.Service.Health = h
	return nil
}

func parseCSVPorts(s string) ([]int, error) {
	var ports []int
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		v, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		ports = append(ports, v)
	}
	return ports, nil
}

func parsePortRange(s string) (config.PortRange, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return config.PortRange{}, errors.New("usage: port-range <start-end>")
	}
	start, err := strconv.Atoi(parts[0])
	if err != nil {
		return config.PortRange{}, err
	}
	end, err := strconv.Atoi(parts[1])
	if err != nil {
		return config.PortRange{}, err
	}
	return config.PortRange{Start: start, End: end}, nil
}

