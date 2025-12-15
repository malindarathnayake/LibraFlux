//go:build !linux

package ipvs

import "fmt"

type RealManager struct {}

func NewManager() (*RealManager, error) {
	return nil, fmt.Errorf("ipvs only supported on linux")
}

func (m *RealManager) Close() {}

func (m *RealManager) GetServices() ([]*Service, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *RealManager) GetDestinations(svc *Service) ([]*Destination, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *RealManager) CreateService(svc *Service) error {
	return fmt.Errorf("not implemented")
}

func (m *RealManager) UpdateService(svc *Service) error {
	return fmt.Errorf("not implemented")
}

func (m *RealManager) DeleteService(svc *Service) error {
	return fmt.Errorf("not implemented")
}

func (m *RealManager) CreateDestination(svc *Service, dst *Destination) error {
	return fmt.Errorf("not implemented")
}

func (m *RealManager) UpdateDestination(svc *Service, dst *Destination) error {
	return fmt.Errorf("not implemented")
}

func (m *RealManager) DeleteDestination(svc *Service, dst *Destination) error {
	return fmt.Errorf("not implemented")
}
