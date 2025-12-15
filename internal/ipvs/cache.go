package ipvs

import (
	"sync"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

// CachedManager wraps a Manager with an in-memory cache to reduce netlink calls.
// It implements the Manager interface transparently.
//
// Cache behavior:
//   - Read operations (GetServices, GetDestinations) check cache first
//   - Write operations (Create/Update/Delete) invalidate the cache
//   - Cache expires after TTL, triggering a fresh fetch on next read
//   - Thread-safe via RWMutex
type CachedManager struct {
	inner   Manager
	ttl     time.Duration
	enabled bool

	mu            sync.RWMutex
	services      []*Service
	destCache     map[string][]*Destination // keyed by service.Key()
	fetchedAt     time.Time                 // services cache timestamp
	destFetchedAt map[string]time.Time      // per-service destination cache timestamps
	hits          uint64
	misses        uint64
}

// CacheConfig holds configuration for the state cache
type CacheConfig struct {
	Enabled bool
	TTL     time.Duration
}

// DefaultCacheConfig returns sensible defaults
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		Enabled: true,
		TTL:     500 * time.Millisecond, // Half the typical 1s reconcile interval
	}
}

// CacheConfigFromDaemonConfig converts config.CacheConfig (daemon.state_cache) into an ipvs.CacheConfig.
func CacheConfigFromDaemonConfig(cfg config.CacheConfig) CacheConfig {
	ttl := time.Duration(cfg.TTLMS) * time.Millisecond
	if cfg.Enabled && cfg.TTLMS == 0 {
		ttl = DefaultCacheConfig().TTL
	}
	return CacheConfig{
		Enabled: cfg.Enabled,
		TTL:     ttl,
	}
}

// NewCachedManager creates a new cached manager wrapping the given Manager.
// If enabled is false, all operations pass through directly to the inner manager.
func NewCachedManager(inner Manager, cfg CacheConfig) *CachedManager {
	return &CachedManager{
		inner:         inner,
		ttl:           cfg.TTL,
		enabled:       cfg.Enabled,
		destCache:     make(map[string][]*Destination),
		destFetchedAt: make(map[string]time.Time),
	}
}

// GetServices returns cached services if valid, otherwise fetches from kernel.
func (c *CachedManager) GetServices() ([]*Service, error) {
	if !c.enabled {
		return c.inner.GetServices()
	}

	// Fast path: check if cache is valid (read lock)
	c.mu.RLock()
	if c.isValidLocked() {
		services := c.copyServicesLocked()
		c.hits++
		c.mu.RUnlock()
		return services, nil
	}
	c.mu.RUnlock()

	// Slow path: fetch and update cache (write lock)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring write lock
	if c.isValidLocked() {
		c.hits++
		return c.copyServicesLocked(), nil
	}

	// Cache miss - fetch from kernel
	services, err := c.inner.GetServices()
	if err != nil {
		c.misses++
		return nil, err
	}

	// Update cache (clear destinations since services changed)
	c.services = services
	c.destCache = make(map[string][]*Destination)
	c.destFetchedAt = make(map[string]time.Time)
	c.fetchedAt = time.Now()
	c.misses++

	return c.copyServicesLocked(), nil
}

// GetDestinations returns cached destinations if valid, otherwise fetches.
func (c *CachedManager) GetDestinations(svc *Service) ([]*Destination, error) {
	if !c.enabled {
		return c.inner.GetDestinations(svc)
	}

	key := svc.Key()

	// Fast path: read lock
	c.mu.RLock()
	if c.isDestValidLocked(key) {
		result := c.copyDestinationsLocked(c.destCache[key])
		c.hits++
		c.mu.RUnlock()
		return result, nil
	}
	c.mu.RUnlock()

	// Slow path: fetch and cache
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check
	if c.isDestValidLocked(key) {
		c.hits++
		return c.copyDestinationsLocked(c.destCache[key]), nil
	}

	// Fetch destinations
	dests, err := c.inner.GetDestinations(svc)
	if err != nil {
		c.misses++
		return nil, err
	}

	// Cache them
	c.destCache[key] = dests
	c.destFetchedAt[key] = time.Now()
	c.misses++

	return c.copyDestinationsLocked(dests), nil
}

// CreateService creates a service and invalidates the cache.
func (c *CachedManager) CreateService(svc *Service) error {
	err := c.inner.CreateService(svc)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// UpdateService updates a service and invalidates the cache.
func (c *CachedManager) UpdateService(svc *Service) error {
	err := c.inner.UpdateService(svc)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// DeleteService deletes a service and invalidates the cache.
func (c *CachedManager) DeleteService(svc *Service) error {
	err := c.inner.DeleteService(svc)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// CreateDestination creates a destination and invalidates the cache.
func (c *CachedManager) CreateDestination(svc *Service, dst *Destination) error {
	err := c.inner.CreateDestination(svc, dst)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// UpdateDestination updates a destination and invalidates the cache.
func (c *CachedManager) UpdateDestination(svc *Service, dst *Destination) error {
	err := c.inner.UpdateDestination(svc, dst)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// DeleteDestination deletes a destination and invalidates the cache.
func (c *CachedManager) DeleteDestination(svc *Service, dst *Destination) error {
	err := c.inner.DeleteDestination(svc, dst)
	if err == nil {
		c.Invalidate()
	}
	return err
}

// Invalidate clears the cache, forcing the next read to fetch fresh data.
func (c *CachedManager) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services = nil
	c.destCache = make(map[string][]*Destination)
	c.destFetchedAt = make(map[string]time.Time)
	c.fetchedAt = time.Time{}
}

// Stats returns cache hit/miss statistics.
func (c *CachedManager) Stats() (hits, misses uint64) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.hits, c.misses
}

// isValidLocked checks if services cache is valid. Must be called with at least read lock held.
func (c *CachedManager) isValidLocked() bool {
	if c.services == nil {
		return false
	}
	return time.Since(c.fetchedAt) < c.ttl
}

// isDestValidLocked checks if destinations cache for a key is valid. Must be called with lock held.
func (c *CachedManager) isDestValidLocked(key string) bool {
	fetchedAt, exists := c.destFetchedAt[key]
	if !exists {
		return false
	}
	if _, ok := c.destCache[key]; !ok {
		return false
	}
	return time.Since(fetchedAt) < c.ttl
}

// copyServicesLocked returns a copy of cached services. Must be called with lock held.
func (c *CachedManager) copyServicesLocked() []*Service {
	if c.services == nil {
		return nil
	}
	result := make([]*Service, len(c.services))
	for i, svc := range c.services {
		copied := *svc
		result[i] = &copied
	}
	return result
}

// copyDestinationsLocked returns a copy of destinations. Must be called with lock held.
func (c *CachedManager) copyDestinationsLocked(dests []*Destination) []*Destination {
	if dests == nil {
		return nil
	}
	result := make([]*Destination, len(dests))
	for i, dst := range dests {
		copied := *dst
		result[i] = &copied
	}
	return result
}


// Inner returns the underlying manager (useful for testing).
func (c *CachedManager) Inner() Manager {
	return c.inner
}

// Enabled returns whether caching is enabled.
func (c *CachedManager) Enabled() bool {
	return c.enabled
}

