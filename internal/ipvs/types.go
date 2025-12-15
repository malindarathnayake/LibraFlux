package ipvs

import (
	"fmt"
	"net"
	"strings"
	"syscall"
)

// Service represents an IPVS service (high level abstraction)
// Note: moby/ipvs has its own Service struct, but we define one for internal use
// to decouple config types from ipvs types if needed, though mostly we map directly.
// For Reconciler we will likely use moby/ipvs types directly or map to them.
type Service struct {
	Address   net.IP
	Protocol  string // tcp, udp
	Port      uint16
	Scheduler string // rr, wrr, lc, etc.
}

// Destination represents an IPVS destination (backend)
type Destination struct {
	Address net.IP
	Port    uint16
	Weight  int
}

// ServiceKey uniquely identifies a service
func (s Service) Key() string {
	return fmt.Sprintf("%s:%s:%d", s.Protocol, s.Address.String(), s.Port)
}

// DestinationKey uniquely identifies a destination
func (d Destination) Key() string {
	return fmt.Sprintf("%s:%d", d.Address.String(), d.Port)
}

// String returns a string representation
func (s Service) String() string {
	return fmt.Sprintf("%s %s:%d (%s)", s.Protocol, s.Address, s.Port, s.Scheduler)
}

func ProtocolToUint16(proto string) uint16 {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "udp":
		return syscall.IPPROTO_UDP
	case "tcp":
		fallthrough
	default:
		return syscall.IPPROTO_TCP
	}
}
