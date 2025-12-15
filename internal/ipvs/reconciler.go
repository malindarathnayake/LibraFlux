package ipvs

import (
	"fmt"
	"net"
	"syscall"

	"github.com/malindarathnayake/LibraFlux/internal/config"
	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

type Reconciler struct {
	manager Manager
	logger  *observability.Logger
}

func NewReconciler(manager Manager, logger *observability.Logger) *Reconciler {
	return &Reconciler{
		manager: manager,
		logger:  logger,
	}
}

type DesiredState struct {
	Service      *Service
	Destinations []*Destination
}

// Apply reconciles the desired state with the actual IPVS state
func (r *Reconciler) Apply(desired []config.Service, vip string) error {
	// 1. Expand desired config into flat list of IPVS services
	desiredState, err := r.expandConfig(desired, vip)
	if err != nil {
		return err
	}

	// 2. Get current state
	currentServices, err := r.manager.GetServices()
	if err != nil {
		return fmt.Errorf("failed to get current IPVS services: %w", err)
	}

	// 3. Reconcile
	return r.reconcile(desiredState, currentServices, vip)
}

func (r *Reconciler) reconcile(desired map[string]*DesiredState, current []*Service, managedVIP string) error {
	currentMap := make(map[string]*Service)
	for _, svc := range current {
		currentMap[svc.Key()] = svc
	}

	// Add/Update
	for key, state := range desired {
		currentSvc, exists := currentMap[key]
		if !exists {
			// Add
			r.logger.Infof("Creating IPVS service: %s", key)
			if err := r.manager.CreateService(state.Service); err != nil {
				r.logger.Errorf("Failed to create service %s: %v", key, err)
				continue
			}
			// Add destinations
			if err := r.reconcileDestinations(state.Service, state.Destinations, nil); err != nil {
				r.logger.Errorf("Failed to reconcile destinations for %s: %v", key, err)
			}
		} else {
			// Update if changed
			if currentSvc.Scheduler != state.Service.Scheduler {
				r.logger.Infof("Updating IPVS service: %s", key)
				currentSvc.Scheduler = state.Service.Scheduler
				if err := r.manager.UpdateService(currentSvc); err != nil {
					r.logger.Errorf("Failed to update service %s: %v", key, err)
				}
			}

			// Reconcile destinations
			currentDests, err := r.manager.GetDestinations(currentSvc)
			if err != nil {
				r.logger.Errorf("Failed to get destinations for %s: %v", key, err)
				continue
			}
			if err := r.reconcileDestinations(currentSvc, state.Destinations, currentDests); err != nil {
				r.logger.Errorf("Failed to reconcile destinations for %s: %v", key, err)
			}
		}
	}

	// Delete
	for key, svc := range currentMap {
		// Only delete if it belongs to our managed VIP
		if svc.Address.String() != managedVIP {
			continue
		}

		if _, exists := desired[key]; !exists {
			r.logger.Infof("Deleting IPVS service: %s", key)
			if err := r.manager.DeleteService(svc); err != nil {
				r.logger.Errorf("Failed to delete service %s: %v", key, err)
			}
		}
	}

	return nil
}

func (r *Reconciler) reconcileDestinations(svc *Service, desired []*Destination, current []*Destination) error {
	currentMap := make(map[string]*Destination)
	for _, dest := range current {
		currentMap[dest.Key()] = dest
	}

	for _, dest := range desired {
		key := dest.Key()
		currDest, exists := currentMap[key]
		if !exists {
			if err := r.manager.CreateDestination(svc, dest); err != nil {
				return err
			}
		} else {
			if currDest.Weight != dest.Weight {
				// Update
				currDest.Weight = dest.Weight
				if err := r.manager.UpdateDestination(svc, currDest); err != nil {
					return err
				}
			}
		}
	}

	for key, dest := range currentMap {
		// Check if exists in desired
		found := false
		for _, d := range desired {
			if d.Key() == key {
				found = true
				break
			}
		}
		if !found {
			if err := r.manager.DeleteDestination(svc, dest); err != nil {
				return err
			}
		}
	}

	return nil
}

func (r *Reconciler) expandConfig(services []config.Service, vip string) (map[string]*DesiredState, error) {
	result := make(map[string]*DesiredState)
	parsedVIP := net.ParseIP(vip)
	if parsedVIP == nil {
		return nil, fmt.Errorf("invalid VIP: %s", vip)
	}

	for _, svc := range services {
		proto := ProtocolToUint16(svc.Protocol)
		protoStr := "tcp"
		if proto == syscall.IPPROTO_UDP {
			protoStr = "udp"
		}

		// Collect ports
		ports := make([]uint16, 0)
		for _, p := range svc.Ports {
			ports = append(ports, uint16(p))
		}
		for _, pr := range svc.PortRanges {
			for p := pr.Start; p <= pr.End; p++ {
				ports = append(ports, uint16(p))
			}
		}

		// Pre-process backends info
		type backendInfo struct {
			address net.IP
			port    uint16
			weight  int
		}
		backends := make([]backendInfo, 0, len(svc.Backends))
		for _, be := range svc.Backends {
			backends = append(backends, backendInfo{
				address: net.ParseIP(be.Address),
				port:    uint16(be.Port),
				weight:  be.Weight,
			})
		}

		for _, port := range ports {
			ipvsSvc := &Service{
				Address:   parsedVIP,
				Protocol:  protoStr,
				Port:      port,
				Scheduler: svc.Scheduler,
			}

			// Resolve destination ports
			resolvedDests := make([]*Destination, len(backends))
			for i, be := range backends {
				portToUse := be.port
				if portToUse == 0 {
					portToUse = port
				}
				resolvedDests[i] = &Destination{
					Address: be.address,
					Port:    portToUse,
					Weight:  be.weight,
				}
			}

			key := ipvsSvc.Key()
			result[key] = &DesiredState{
				Service:      ipvsSvc,
				Destinations: resolvedDests,
			}
		}
	}

	return result, nil
}
