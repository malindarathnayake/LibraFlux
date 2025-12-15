package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/malindarathnayake/LibraFlux/internal/observability"
)

var notifySignals = signal.Notify
var stopSignals = signal.Stop

// ContextWithSignals returns a derived context that is canceled on SIGTERM/SIGINT,
// plus a reload channel that is notified (coalesced) on SIGHUP.
func ContextWithSignals(parent context.Context, logger *observability.Logger) (context.Context, <-chan struct{}, func()) {
	ctx, cancel := context.WithCancel(parent)
	reloadCh := make(chan struct{}, 1)

	sigCh := make(chan os.Signal, 2)
	notifySignals(sigCh, syscall.SIGHUP, syscall.SIGTERM, os.Interrupt)

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			stopSignals(sigCh)
			cancel()
		})
	}

	go func() {
		defer func() {
			stopSignals(sigCh)
			close(reloadCh)
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case sig := <-sigCh:
				switch sig {
				case syscall.SIGHUP:
					select {
					case reloadCh <- struct{}{}:
					default:
					}
				case syscall.SIGTERM, os.Interrupt:
					if logger != nil {
						logger.Info("Termination signal received", map[string]interface{}{
							"signal": fmt.Sprintf("%v", sig),
						})
					}
					stop()
					return
				default:
				}
			}
		}
	}()

	return ctx, reloadCh, stop
}

