package health

import (
	"fmt"
	"net"
	"time"
)

type Dialer interface {
	DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)
}

type NetDialer struct{}

func (NetDialer) DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(network, address, timeout)
}

type Checker interface {
	Check(address string, port int, timeout time.Duration) error
}

type TCPChecker struct {
	Dialer Dialer
}

func (c *TCPChecker) Check(address string, port int, timeout time.Duration) error {
	if c == nil || c.Dialer == nil {
		return fmt.Errorf("missing dialer")
	}
	if net.ParseIP(address) == nil {
		return fmt.Errorf("invalid address: %s", address)
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	if timeout <= 0 {
		return fmt.Errorf("invalid timeout: %s", timeout)
	}

	conn, err := c.Dialer.DialTimeout("tcp", fmt.Sprintf("%s:%d", address, port), timeout)
	if err != nil {
		return err
	}
	_ = conn.Close()
	return nil
}
