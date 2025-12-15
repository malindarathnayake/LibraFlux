package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// MetricsRegistry manages Prometheus metrics
type MetricsRegistry struct {
	Registry *prometheus.Registry
	counters map[string]*prometheus.CounterVec
	gauges   map[string]*prometheus.GaugeVec
	mu       sync.RWMutex
}

// NewMetricsRegistry creates a new metrics registry with a custom Prometheus registry
func NewMetricsRegistry() *MetricsRegistry {
	// Include Go runtime metrics and process metrics by default?
	// For a custom registry, they are not included by default.
	// We'll keep it clean for now.
	return &MetricsRegistry{
		Registry: prometheus.NewRegistry(),
		counters: make(map[string]*prometheus.CounterVec),
		gauges:   make(map[string]*prometheus.GaugeVec),
	}
}

// NewCounter creates or retrieves a counter metric
func (m *MetricsRegistry) NewCounter(name, help string, labels []string) *prometheus.CounterVec {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c, exists := m.counters[name]; exists {
		return c
	}

	c := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: name,
		Help: help,
	}, labels)

	m.Registry.MustRegister(c)
	m.counters[name] = c
	return c
}

// NewGauge creates or retrieves a gauge metric
func (m *MetricsRegistry) NewGauge(name, help string, labels []string) *prometheus.GaugeVec {
	m.mu.Lock()
	defer m.mu.Unlock()

	if g, exists := m.gauges[name]; exists {
		return g
	}

	g := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: name,
		Help: help,
	}, labels)

	m.Registry.MustRegister(g)
	m.gauges[name] = g
	return g
}

// Counter is a helper to increment a counter with labels
func (m *MetricsRegistry) Counter(name string, labels prometheus.Labels) prometheus.Counter {
	m.mu.RLock()
	c, ok := m.counters[name]
	m.mu.RUnlock()

	if !ok {
		return noopCounter{}
	}

	return c.With(labels)
}

// Gauge is a helper to access a gauge with labels
func (m *MetricsRegistry) Gauge(name string, labels prometheus.Labels) prometheus.Gauge {
	m.mu.RLock()
	g, ok := m.gauges[name]
	m.mu.RUnlock()

	if !ok {
		return noopGauge{}
	}

	return g.With(labels)
}

type noopCounter struct{}

func (noopCounter) Desc() *prometheus.Desc {
	return prometheus.NewDesc("lbctl_noop_counter", "noop counter", nil, nil)
}

func (noopCounter) Write(metric *dto.Metric) error {
	if metric == nil {
		return nil
	}
	zero := float64(0)
	metric.Counter = &dto.Counter{Value: &zero}
	return nil
}

func (noopCounter) Describe(_ chan<- *prometheus.Desc) {}
func (noopCounter) Collect(_ chan<- prometheus.Metric) {}
func (noopCounter) Inc()                               {}
func (noopCounter) Add(_ float64)                      {}

type noopGauge struct{}

func (noopGauge) Desc() *prometheus.Desc {
	return prometheus.NewDesc("lbctl_noop_gauge", "noop gauge", nil, nil)
}

func (noopGauge) Write(metric *dto.Metric) error {
	if metric == nil {
		return nil
	}
	zero := float64(0)
	metric.Gauge = &dto.Gauge{Value: &zero}
	return nil
}

func (noopGauge) Describe(_ chan<- *prometheus.Desc) {}
func (noopGauge) Collect(_ chan<- prometheus.Metric) {}
func (noopGauge) Set(_ float64)                      {}
func (noopGauge) Inc()                               {}
func (noopGauge) Dec()                               {}
func (noopGauge) Add(_ float64)                      {}
func (noopGauge) Sub(_ float64)                      {}
func (noopGauge) SetToCurrentTime()                  {}
