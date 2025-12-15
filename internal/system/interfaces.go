package system

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

// NetworkManager defines network operations
type NetworkManager interface {
	CheckVIPPresent(vip string) (bool, error)
	GetInterfaceStatus(iface string) (bool, error)
}

// RealNetworkManager implements NetworkManager using netlink
type RealNetworkManager struct{}

// NewNetworkManager creates a new network manager
func NewNetworkManager() *RealNetworkManager {
	return &RealNetworkManager{}
}

// CheckVIPPresent checks if the VIP exists on any interface
func (n *RealNetworkManager) CheckVIPPresent(vip string) (bool, error) {
	// Parse VIP
	parsedVIP := net.ParseIP(vip)
	if parsedVIP == nil {
		return false, fmt.Errorf("invalid VIP: %s", vip)
	}

	// List all addresses (family 0 = all)
	addrs, err := netlink.AddrList(nil, 0)
	if err != nil {
		return false, fmt.Errorf("failed to list addresses: %w", err)
	}

	for _, addr := range addrs {
		if addr.IP.Equal(parsedVIP) {
			return true, nil
		}
	}

	return false, nil
}

// GetInterfaceStatus checks if an interface is up
func (n *RealNetworkManager) GetInterfaceStatus(iface string) (bool, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return false, fmt.Errorf("interface %s not found: %w", iface, err)
	}

	attrs := link.Attrs()
	return attrs.Flags&net.FlagUp != 0, nil
}
