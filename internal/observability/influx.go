package observability

import (
	"context"
	"fmt"
	"time"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
	dto "github.com/prometheus/client_model/go"
)

// InfluxPusher periodically pushes metrics to InfluxDB
type InfluxPusher struct {
	client   influxdb2.Client
	writeAPI api.WriteAPIBlocking
	registry *MetricsRegistry
	org      string
	bucket   string
	interval time.Duration
	logger   *Logger
	stopCh   chan struct{}
	doneCh   chan struct{}
}

// InfluxConfig holds InfluxDB connection parameters
type InfluxConfig struct {
	URL      string
	Token    string
	Org      string
	Bucket   string
	Interval time.Duration
}

// NewInfluxPusher creates a new InfluxDB pusher
func NewInfluxPusher(cfg InfluxConfig, registry *MetricsRegistry, logger *Logger) (*InfluxPusher, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("influxdb url is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("influxdb token is required")
	}
	if cfg.Org == "" {
		return nil, fmt.Errorf("influxdb org is required")
	}
	if cfg.Bucket == "" {
		return nil, fmt.Errorf("influxdb bucket is required")
	}
	if cfg.Interval < time.Second {
		return nil, fmt.Errorf("influxdb interval must be at least 1 second")
	}

	client := influxdb2.NewClient(cfg.URL, cfg.Token)
	writeAPI := client.WriteAPIBlocking(cfg.Org, cfg.Bucket)

	return &InfluxPusher{
		client:   client,
		writeAPI: writeAPI,
		registry: registry,
		org:      cfg.Org,
		bucket:   cfg.Bucket,
		interval: cfg.Interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}, nil
}

// Start begins the periodic push loop
func (p *InfluxPusher) Start(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()
	defer close(p.doneCh)

	p.logger.Info("InfluxDB pusher started", map[string]interface{}{
		"interval": p.interval.String(),
		"org":      p.org,
		"bucket":   p.bucket,
	})

	for {
		select {
		case <-ctx.Done():
			p.logger.Info("InfluxDB pusher stopped (context)", nil)
			return
		case <-p.stopCh:
			p.logger.Info("InfluxDB pusher stopped", nil)
			return
		case <-ticker.C:
			if err := p.push(ctx); err != nil {
				p.logger.Warn("Failed to push metrics to InfluxDB", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}
	}
}

// Stop stops the pusher
func (p *InfluxPusher) Stop() {
	select {
	case <-p.stopCh:
		// Already stopped
		return
	default:
		close(p.stopCh)
	}
	
	// Only wait for doneCh if Start() is running
	// This prevents deadlock when Stop() is called without Start()
	select {
	case <-p.doneCh:
		// Start() has finished
	default:
		// Start() was never called or hasn't started yet
	}
	
	p.client.Close()
}

// push collects metrics from registry and pushes to InfluxDB
func (p *InfluxPusher) push(ctx context.Context) error {
	// Gather metrics from Prometheus registry
	metricFamilies, err := p.registry.Registry.Gather()
	if err != nil {
		return fmt.Errorf("failed to gather metrics: %w", err)
	}

	// Convert to InfluxDB line protocol points
	points := p.convertToPoints(metricFamilies)
	if len(points) == 0 {
		return nil
	}

	// Write to InfluxDB
	if err := p.writeAPI.WritePoint(ctx, points...); err != nil {
		return fmt.Errorf("failed to write points: %w", err)
	}

	return nil
}

// convertToPoints converts Prometheus metrics to InfluxDB points
func (p *InfluxPusher) convertToPoints(metricFamilies []*dto.MetricFamily) []*write.Point {
	var points []*write.Point
	now := time.Now()

	for _, mf := range metricFamilies {
		metricName := mf.GetName()
		metricType := mf.GetType()

		for _, m := range mf.GetMetric() {
			// Extract labels as tags
			tags := make(map[string]string)
			for _, lp := range m.GetLabel() {
				tags[lp.GetName()] = lp.GetValue()
			}

			// Extract value based on metric type
			var value float64
			var hasValue bool

			switch metricType {
			case dto.MetricType_COUNTER:
				if m.Counter != nil {
					value = m.Counter.GetValue()
					hasValue = true
				}
			case dto.MetricType_GAUGE:
				if m.Gauge != nil {
					value = m.Gauge.GetValue()
					hasValue = true
				}
			case dto.MetricType_UNTYPED:
				if m.Untyped != nil {
					value = m.Untyped.GetValue()
					hasValue = true
				}
			// HISTOGRAM and SUMMARY are more complex - skip for now
			}

			if hasValue {
				// Create InfluxDB point
				// Measurement name is the metric name
				// Tags include all labels
				// Field is "value" with the metric value
				point := write.NewPoint(
					metricName,
					tags,
					map[string]interface{}{"value": value},
					now,
				)
				points = append(points, point)
			}
		}
	}

	return points
}

// TestConnection verifies InfluxDB connectivity
func (p *InfluxPusher) TestConnection(ctx context.Context) error {
	// Try to ping the server
	health, err := p.client.Health(ctx)
	if err != nil {
		return fmt.Errorf("influxdb health check failed: %w", err)
	}

	if health.Status != "pass" {
		return fmt.Errorf("influxdb health status: %s (message: %s)", health.Status, *health.Message)
	}

	return nil
}

// GatherMetrics is a helper to manually gather current metrics (useful for testing)
func (p *InfluxPusher) GatherMetrics() ([]*dto.MetricFamily, error) {
	return p.registry.Registry.Gather()
}
