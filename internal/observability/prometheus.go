package observability

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// PrometheusServer exposes metrics via HTTP endpoint
type PrometheusServer struct {
	registry *MetricsRegistry
	server   *http.Server
	logger   *Logger
	port     int
	path     string
	bind     string
}

// PrometheusConfig holds Prometheus server parameters
type PrometheusConfig struct {
	Port int
	Path string
	Bind string // Bind address (empty = all interfaces, "127.0.0.1" = localhost only)
}

// NewPrometheusServer creates a new Prometheus HTTP server
func NewPrometheusServer(cfg PrometheusConfig, registry *MetricsRegistry, logger *Logger) (*PrometheusServer, error) {
	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("prometheus port must be between 1-65535")
	}
	if cfg.Path == "" {
		cfg.Path = "/metrics"
	}
	if cfg.Path[0] != '/' {
		cfg.Path = "/" + cfg.Path
	}
	// Validate bind address if provided
	if cfg.Bind != "" && cfg.Bind != "0.0.0.0" && cfg.Bind != "::" {
		if ip := net.ParseIP(cfg.Bind); ip == nil {
			return nil, fmt.Errorf("prometheus bind must be a valid IP address: %s", cfg.Bind)
		}
	}

	return &PrometheusServer{
		registry: registry,
		logger:   logger,
		port:     cfg.Port,
		path:     cfg.Path,
		bind:     cfg.Bind,
	}, nil
}

// Start starts the HTTP server
func (s *PrometheusServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	
	// Prometheus metrics endpoint
	mux.Handle(s.path, promhttp.HandlerFor(
		s.registry.Registry,
		promhttp.HandlerOpts{
			EnableOpenMetrics: true,
		},
	))

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Root endpoint with helpful info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>lbctl Metrics</title>
	<style>
		body { font-family: sans-serif; margin: 40px; }
		.endpoint { margin: 20px 0; }
		a { color: #0066cc; text-decoration: none; }
		a:hover { text-decoration: underline; }
	</style>
</head>
<body>
	<h1>lbctl Prometheus Metrics</h1>
	<div class="endpoint">
		<h2>Endpoints:</h2>
		<ul>
			<li><a href="%s">%s</a> - Prometheus metrics</li>
			<li><a href="/health">/health</a> - Health check</li>
		</ul>
	</div>
</body>
</html>`, s.path, s.path)
		w.Write([]byte(html))
	})

	addr := fmt.Sprintf("%s:%d", s.bind, s.port)
	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.logger.Info("Prometheus server starting", map[string]interface{}{
		"addr": addr,
		"path": s.path,
	})

	// Start server in goroutine
	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("Prometheus server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Wait for context cancellation
	<-ctx.Done()
	return s.Stop()
}

// Stop gracefully shuts down the HTTP server
func (s *PrometheusServer) Stop() error {
	if s.server == nil {
		return nil
	}

	s.logger.Info("Prometheus server stopping", nil)

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("prometheus server shutdown error: %w", err)
	}

	s.logger.Info("Prometheus server stopped", nil)
	return nil
}

// TestConnection verifies the Prometheus endpoint is accessible
func (s *PrometheusServer) TestConnection() error {
	url := fmt.Sprintf("http://localhost:%d%s", s.port, s.path)
	
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("prometheus endpoint not accessible: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("prometheus endpoint returned status %d", resp.StatusCode)
	}

	return nil
}

// GetURL returns the full URL for the metrics endpoint
func (s *PrometheusServer) GetURL() string {
	host := s.bind
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "localhost"
	}
	return fmt.Sprintf("http://%s:%d%s", host, s.port, s.path)
}
