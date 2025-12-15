//go:build linux

package ipvs

import (
	"fmt"
	"syscall"

	libipvs "github.com/moby/ipvs"
)

// RealManager implements Manager using moby/ipvs
type RealManager struct {
	handle *libipvs.Handle
}

func NewManager() (*RealManager, error) {
	handle, err := libipvs.New("")
	if err != nil {
		return nil, fmt.Errorf("failed to create IPVS handle: %w", err)
	}
	return &RealManager{handle: handle}, nil
}

func (m *RealManager) Close() {
	if m.handle != nil {
		m.handle.Close()
	}
}

func (m *RealManager) GetServices() ([]*Service, error) {
	svcs, err := m.handle.GetServices()
	if err != nil {
		return nil, err
	}
	result := make([]*Service, len(svcs))
	for i, s := range svcs {
		result[i] = toService(s)
	}
	return result, nil
}

func (m *RealManager) GetDestinations(svc *Service) ([]*Destination, error) {
	libSvc := fromService(svc)
	dests, err := m.handle.GetDestinations(libSvc)
	if err != nil {
		return nil, err
	}
	result := make([]*Destination, len(dests))
	for i, d := range dests {
		result[i] = toDestination(d)
	}
	return result, nil
}

func (m *RealManager) CreateService(svc *Service) error {
	return m.handle.NewService(fromService(svc))
}

func (m *RealManager) UpdateService(svc *Service) error {
	return m.handle.UpdateService(fromService(svc))
}

func (m *RealManager) DeleteService(svc *Service) error {
	return m.handle.DelService(fromService(svc))
}

func (m *RealManager) CreateDestination(svc *Service, dst *Destination) error {
	return m.handle.NewDestination(fromService(svc), fromDestination(dst))
}

func (m *RealManager) UpdateDestination(svc *Service, dst *Destination) error {
	return m.handle.UpdateDestination(fromService(svc), fromDestination(dst))
}

func (m *RealManager) DeleteDestination(svc *Service, dst *Destination) error {
	return m.handle.DelDestination(fromService(svc), fromDestination(dst))
}

func toService(s *libipvs.Service) *Service {
	proto := "tcp"
	if s.Protocol == syscall.IPPROTO_UDP {
		proto = "udp"
	}
	return &Service{
		Address:   s.Address,
		Protocol:  proto,
		Port:      s.Port,
		Scheduler: s.SchedName,
	}
}

func fromService(s *Service) *libipvs.Service {
	proto := syscall.IPPROTO_TCP
	if s.Protocol == "udp" {
		proto = syscall.IPPROTO_UDP
	}
	return &libipvs.Service{
		Address:       s.Address,
		Protocol:      uint16(proto),
		Port:          s.Port,
		SchedName:     s.Scheduler,
		AddressFamily: syscall.AF_INET,
		Netmask:       0xFFFFFFFF,
	}
}

func toDestination(d *libipvs.Destination) *Destination {
	return &Destination{
		Address: d.Address,
		Port:    d.Port,
		Weight:  d.Weight,
	}
}

func fromDestination(d *Destination) *libipvs.Destination {
	return &libipvs.Destination{
		Address:       d.Address,
		Port:          d.Port,
		Weight:        d.Weight,
		AddressFamily: syscall.AF_INET,
	}
}
