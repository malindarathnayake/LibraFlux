package observability

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// TestInfluxPusher_Config tests InfluxDB configuration validation
func TestInfluxPusher_Config(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	tests := []struct {
		name    string
		cfg     InfluxConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: InfluxConfig{
				URL:      "http://localhost:8086",
				Token:    "mytoken",
				Org:      "myorg",
				Bucket:   "mybucket",
				Interval: 10 * time.Second,
			},
			wantErr: false,
		},
		{
			name: "missing URL",
			cfg: InfluxConfig{
				Token:    "mytoken",
				Org:      "myorg",
				Bucket:   "mybucket",
				Interval: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "url is required",
		},
		{
			name: "missing token",
			cfg: InfluxConfig{
				URL:      "http://localhost:8086",
				Org:      "myorg",
				Bucket:   "mybucket",
				Interval: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "token is required",
		},
		{
			name: "missing org",
			cfg: InfluxConfig{
				URL:      "http://localhost:8086",
				Token:    "mytoken",
				Bucket:   "mybucket",
				Interval: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "org is required",
		},
		{
			name: "missing bucket",
			cfg: InfluxConfig{
				URL:      "http://localhost:8086",
				Token:    "mytoken",
				Org:      "myorg",
				Interval: 10 * time.Second,
			},
			wantErr: true,
			errMsg:  "bucket is required",
		},
		{
			name: "interval too short",
			cfg: InfluxConfig{
				URL:      "http://localhost:8086",
				Token:    "mytoken",
				Org:      "myorg",
				Bucket:   "mybucket",
				Interval: 500 * time.Millisecond,
			},
			wantErr: true,
			errMsg:  "interval must be at least 1 second",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pusher, err := NewInfluxPusher(tt.cfg, registry, logger)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewInfluxPusher() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewInfluxPusher() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("NewInfluxPusher() unexpected error: %v", err)
					return
				}
				if pusher == nil {
					t.Errorf("NewInfluxPusher() returned nil pusher")
					return
				}
				// Clean up
				pusher.Stop()
			}
		})
	}
}

// TestInfluxPusher_ConvertToPoints tests metric conversion to InfluxDB format
func TestInfluxPusher_ConvertToPoints(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	// Create test metrics
	counter := registry.NewCounter("test_counter_total", "Test counter", []string{"label1", "label2"})
	gauge := registry.NewGauge("test_gauge", "Test gauge", []string{"service"})

	// Set values
	counter.With(prometheus.Labels{"label1": "value1", "label2": "value2"}).Add(42)
	gauge.With(prometheus.Labels{"service": "api"}).Set(123.45)

	// Create pusher
	cfg := InfluxConfig{
		URL:      "http://localhost:8086",
		Token:    "test-token",
		Org:      "test-org",
		Bucket:   "test-bucket",
		Interval: 10 * time.Second,
	}
	pusher, err := NewInfluxPusher(cfg, registry, logger)
	if err != nil {
		t.Fatalf("NewInfluxPusher() error: %v", err)
	}
	defer pusher.Stop()

	// Gather metrics
	metricFamilies, err := pusher.GatherMetrics()
	if err != nil {
		t.Fatalf("GatherMetrics() error: %v", err)
	}

	// Convert to points
	points := pusher.convertToPoints(metricFamilies)

	// Verify we have points
	if len(points) == 0 {
		t.Errorf("convertToPoints() returned 0 points, want > 0")
		return
	}

	// Check that points were created for our metrics
	hasCounter := false
	hasGauge := false

	for _, point := range points {
		name := point.Name()
		if name == "test_counter_total" {
			hasCounter = true
		}
		if name == "test_gauge" {
			hasGauge = true
		}
	}

	if !hasCounter {
		t.Errorf("convertToPoints() missing test_counter_total")
	}
	if !hasGauge {
		t.Errorf("convertToPoints() missing test_gauge")
	}
}

// TestInfluxPusher_Lifecycle tests start and stop
func TestInfluxPusher_Lifecycle(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	cfg := InfluxConfig{
		URL:      "http://localhost:8086",
		Token:    "test-token",
		Org:      "test-org",
		Bucket:   "test-bucket",
		Interval: 1 * time.Second,
	}

	pusher, err := NewInfluxPusher(cfg, registry, logger)
	if err != nil {
		t.Fatalf("NewInfluxPusher() error: %v", err)
	}

	// Start in background
	ctx, cancel := context.WithCancel(context.Background())
	go pusher.Start(ctx)

	// Let it run briefly
	time.Sleep(100 * time.Millisecond)

	// Stop via context
	cancel()

	// Give it time to clean up
	time.Sleep(100 * time.Millisecond)

	// Should be able to stop again safely
	pusher.Stop()
}

// TestPrometheusServer_Config tests Prometheus configuration validation
func TestPrometheusServer_Config(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	tests := []struct {
		name    string
		cfg     PrometheusConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			cfg: PrometheusConfig{
				Port: 9090,
				Path: "/metrics",
			},
			wantErr: false,
		},
		{
			name: "valid config without leading slash",
			cfg: PrometheusConfig{
				Port: 9090,
				Path: "metrics",
			},
			wantErr: false,
		},
		{
			name: "port too low",
			cfg: PrometheusConfig{
				Port: 0,
				Path: "/metrics",
			},
			wantErr: true,
			errMsg:  "port must be between",
		},
		{
			name: "port too high",
			cfg: PrometheusConfig{
				Port: 70000,
				Path: "/metrics",
			},
			wantErr: true,
			errMsg:  "port must be between",
		},
		{
			name: "empty path gets default",
			cfg: PrometheusConfig{
				Port: 9090,
				Path: "",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewPrometheusServer(tt.cfg, registry, logger)
			if tt.wantErr {
				if err == nil {
					t.Errorf("NewPrometheusServer() expected error, got nil")
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("NewPrometheusServer() error = %v, want error containing %q", err, tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("NewPrometheusServer() unexpected error: %v", err)
					return
				}
				if server == nil {
					t.Errorf("NewPrometheusServer() returned nil server")
					return
				}

				// Verify path has leading slash
				if server.path[0] != '/' {
					t.Errorf("NewPrometheusServer() path = %q, want leading slash", server.path)
				}

				// If empty path was provided, verify default
				if tt.cfg.Path == "" && server.path != "/metrics" {
					t.Errorf("NewPrometheusServer() path = %q, want %q for empty input", server.path, "/metrics")
				}
			}
		})
	}
}

// TestPrometheusServer_Endpoints tests HTTP endpoints
func TestPrometheusServer_Endpoints(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	// Create test metrics
	counter := registry.NewCounter("http_requests_total", "Total requests", []string{"method"})
	counter.With(prometheus.Labels{"method": "GET"}).Add(100)

	// Create server with random available port
	cfg := PrometheusConfig{
		Port: 19090, // Use high port to avoid conflicts
		Path: "/metrics",
	}

	server, err := NewPrometheusServer(cfg, registry, logger)
	if err != nil {
		t.Fatalf("NewPrometheusServer() error: %v", err)
	}

	// Start server in background
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		if err := server.Start(ctx); err != nil {
			t.Logf("Server.Start() error: %v", err)
		}
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Test metrics endpoint
	t.Run("metrics endpoint", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%d/metrics", cfg.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s error: %v", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", url, resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}

		bodyStr := string(body)

		// Check for our metric
		if !strings.Contains(bodyStr, "http_requests_total") {
			t.Errorf("Metrics response missing http_requests_total")
		}
	})

	// Test health endpoint
	t.Run("health endpoint", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%d/health", cfg.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s error: %v", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", url, resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}

		if string(body) != "ok" {
			t.Errorf("Health response = %q, want %q", string(body), "ok")
		}
	})

	// Test root endpoint
	t.Run("root endpoint", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%d/", cfg.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s error: %v", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s status = %d, want %d", url, resp.StatusCode, http.StatusOK)
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("ReadAll() error: %v", err)
		}

		bodyStr := string(body)
		if !strings.Contains(bodyStr, "lbctl Prometheus Metrics") {
			t.Errorf("Root response missing expected HTML content")
		}
	})

	// Test 404 for unknown path
	t.Run("404 for unknown path", func(t *testing.T) {
		url := fmt.Sprintf("http://localhost:%d/unknown", cfg.Port)
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("GET %s error: %v", url, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s status = %d, want %d", url, resp.StatusCode, http.StatusNotFound)
		}
	})

	// Stop server
	cancel()
	time.Sleep(100 * time.Millisecond)
}

// TestPrometheusServer_GetURL tests URL generation
func TestPrometheusServer_GetURL(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	tests := []struct {
		name     string
		port     int
		path     string
		bind     string
		wantURL  string
	}{
		{
			name:    "default metrics path",
			port:    9090,
			path:    "/metrics",
			bind:    "",
			wantURL: "http://localhost:9090/metrics",
		},
		{
			name:    "custom path",
			port:    8080,
			path:    "/prometheus/metrics",
			bind:    "",
			wantURL: "http://localhost:8080/prometheus/metrics",
		},
		{
			name:    "localhost bind",
			port:    9090,
			path:    "/metrics",
			bind:    "127.0.0.1",
			wantURL: "http://127.0.0.1:9090/metrics",
		},
		{
			name:    "0.0.0.0 bind shows localhost",
			port:    9090,
			path:    "/metrics",
			bind:    "0.0.0.0",
			wantURL: "http://localhost:9090/metrics",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := PrometheusConfig{
				Port: tt.port,
				Path: tt.path,
				Bind: tt.bind,
			}
			server, err := NewPrometheusServer(cfg, registry, logger)
			if err != nil {
				t.Fatalf("NewPrometheusServer() error: %v", err)
			}

			url := server.GetURL()
			if url != tt.wantURL {
				t.Errorf("GetURL() = %q, want %q", url, tt.wantURL)
			}
		})
	}
}

// TestPrometheusServer_BindValidation tests bind address validation
func TestPrometheusServer_BindValidation(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	tests := []struct {
		name    string
		bind    string
		wantErr bool
	}{
		{name: "empty bind (all interfaces)", bind: "", wantErr: false},
		{name: "0.0.0.0 (all interfaces)", bind: "0.0.0.0", wantErr: false},
		{name: ":: (all interfaces IPv6)", bind: "::", wantErr: false},
		{name: "localhost IPv4", bind: "127.0.0.1", wantErr: false},
		{name: "specific IP", bind: "192.168.1.1", wantErr: false},
		{name: "invalid IP", bind: "not-an-ip", wantErr: true},
		{name: "invalid format", bind: "256.256.256.256", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := PrometheusConfig{
				Port: 9090,
				Path: "/metrics",
				Bind: tt.bind,
			}
			_, err := NewPrometheusServer(cfg, registry, logger)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewPrometheusServer() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPrometheusServer_Lifecycle tests start and stop
func TestPrometheusServer_Lifecycle(t *testing.T) {
	logger := NewLogger(InfoLevel)
	registry := NewMetricsRegistry()

	cfg := PrometheusConfig{
		Port: 19091,
		Path: "/metrics",
	}

	server, err := NewPrometheusServer(cfg, registry, logger)
	if err != nil {
		t.Fatalf("NewPrometheusServer() error: %v", err)
	}

	// Start in background
	ctx, cancel := context.WithCancel(context.Background())
	
	done := make(chan error, 1)
	go func() {
		done <- server.Start(ctx)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify it's running
	url := fmt.Sprintf("http://localhost:%d/health", cfg.Port)
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Server not running: %v", err)
	}
	resp.Body.Close()

	// Stop via context
	cancel()

	// Wait for shutdown
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("Start() returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Error("Server did not shut down within timeout")
	}

	// Should be able to stop again safely
	if err := server.Stop(); err != nil {
		t.Errorf("Second Stop() returned error: %v", err)
	}
}
