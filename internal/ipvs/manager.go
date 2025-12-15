package ipvs

// Manager defines IPVS operations
type Manager interface {
	GetServices() ([]*Service, error)
	GetDestinations(svc *Service) ([]*Destination, error)
	CreateService(svc *Service) error
	UpdateService(svc *Service) error
	DeleteService(svc *Service) error
	CreateDestination(svc *Service, dst *Destination) error
	UpdateDestination(svc *Service, dst *Destination) error
	DeleteDestination(svc *Service, dst *Destination) error
}
