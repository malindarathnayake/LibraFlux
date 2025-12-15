package observability

import (
	"sync"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// Helper to get value from a metric
func getMetricValue(m prometheus.Metric) float64 {
	var metric dto.Metric
	m.Write(&metric)
	if metric.Counter != nil {
		return *metric.Counter.Value
	}
	if metric.Gauge != nil {
		return *metric.Gauge.Value
	}
	return 0
}

func TestMetricsNewRegistry(t *testing.T) {
	registry := NewMetricsRegistry()
	if registry == nil {
		t.Fatal("NewMetricsRegistry returned nil")
	}
	if registry.Registry == nil {
		t.Fatal("Registry inside MetricsRegistry is nil")
	}
}

func TestMetricsRegisterCounter(t *testing.T) {
	registry := NewMetricsRegistry()
	name := "test_counter"
	help := "test counter help"
	labels := []string{"tag"}

	c1 := registry.NewCounter(name, help, labels)
	if c1 == nil {
		t.Fatal("NewCounter returned nil")
	}

	// Verify idempotency
	c2 := registry.NewCounter(name, help, labels)
	if c1 != c2 {
		t.Error("NewCounter should return existing metric if already registered")
	}
}

func TestMetricsRegisterGauge(t *testing.T) {
	registry := NewMetricsRegistry()
	name := "test_gauge"
	help := "test gauge help"
	labels := []string{"tag"}

	g1 := registry.NewGauge(name, help, labels)
	if g1 == nil {
		t.Fatal("NewGauge returned nil")
	}

	// Verify idempotency
	g2 := registry.NewGauge(name, help, labels)
	if g1 != g2 {
		t.Error("NewGauge should return existing metric if already registered")
	}
}

func TestMetricsCounterOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	name := "requests_total"
	registry.NewCounter(name, "total requests", []string{"status"})

	// Increment
	c := registry.Counter(name, prometheus.Labels{"status": "200"})
	if c == nil {
		t.Fatal("Counter returned nil")
	}

	c.Inc()
	if val := getMetricValue(c); val != 1 {
		t.Errorf("expected 1, got %f", val)
	}

	c.Add(5)
	if val := getMetricValue(c); val != 6 {
		t.Errorf("expected 6, got %f", val)
	}
}

func TestMetricsGaugeOperations(t *testing.T) {
	registry := NewMetricsRegistry()
	name := "active_connections"
	registry.NewGauge(name, "active connections", []string{"service"})

	g := registry.Gauge(name, prometheus.Labels{"service": "web"})
	if g == nil {
		t.Fatal("Gauge returned nil")
	}

	g.Set(10)
	if val := getMetricValue(g); val != 10 {
		t.Errorf("expected 10, got %f", val)
	}

	g.Inc()
	if val := getMetricValue(g); val != 11 {
		t.Errorf("expected 11, got %f", val)
	}

	g.Dec()
	if val := getMetricValue(g); val != 10 {
		t.Errorf("expected 10, got %f", val)
	}

	g.Sub(5)
	if val := getMetricValue(g); val != 5 {
		t.Errorf("expected 5, got %f", val)
	}
}

func TestMetricsMissingMetrics(t *testing.T) {
	registry := NewMetricsRegistry()

	c := registry.Counter("non_existent", prometheus.Labels{})
	c.Inc()

	g := registry.Gauge("non_existent", prometheus.Labels{})
	g.Inc()
}

func TestMetricsConcurrency(t *testing.T) {
	registry := NewMetricsRegistry()
	name := "concurrent_test"
	registry.NewCounter(name, "help", []string{"id"})

	var wg sync.WaitGroup
	count := 100

	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			registry.NewCounter(name, "help", []string{"id"}) // Should be safe
			registry.Counter(name, prometheus.Labels{"id": "1"}).Inc()
		}()
	}

	wg.Wait()

	val := getMetricValue(registry.Counter(name, prometheus.Labels{"id": "1"}))
	if val != float64(count) {
		t.Errorf("expected %d, got %f", count, val)
	}
}
