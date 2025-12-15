package observability

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

// TestLoggerNew verifies logger creation
func TestLoggerNew(t *testing.T) {
	logger := NewLogger(InfoLevel)

	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}

	if logger.level != InfoLevel {
		t.Errorf("expected level %v, got %v", InfoLevel, logger.level)
	}

	if logger.gelfEnabled {
		t.Error("GELF should be disabled by default")
	}

	if logger.hostname == "" {
		t.Error("hostname should be set")
	}
}

// TestLoggerParseLogLevel tests log level string parsing
func TestLoggerParseLogLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected LogLevel
		wantErr  bool
	}{
		{"debug", DebugLevel, false},
		{"info", InfoLevel, false},
		{"warn", WarnLevel, false},
		{"warning", WarnLevel, false},
		{"error", ErrorLevel, false},
		{"DEBUG", DebugLevel, false},
		{"INFO", InfoLevel, false},
		{"invalid", InfoLevel, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			level, err := ParseLogLevel(tt.input)

			if tt.wantErr && err == nil {
				t.Error("expected error, got nil")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if level != tt.expected {
				t.Errorf("expected level %v, got %v", tt.expected, level)
			}
		})
	}
}

// TestLoggerLogLevelString tests log level string representation
func TestLoggerLogLevelString(t *testing.T) {
	tests := []struct {
		level    LogLevel
		expected string
	}{
		{DebugLevel, "DEBUG"},
		{InfoLevel, "INFO"},
		{WarnLevel, "WARN"},
		{ErrorLevel, "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if tt.level.String() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.level.String())
			}
		})
	}
}

// TestLoggerLevelFiltering verifies that messages below the threshold are not logged
func TestLoggerLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(WarnLevel)
	logger.SetConsoleOutput(&buf)

	// These should not be logged
	logger.Debug("debug message")
	logger.Info("info message")

	if buf.Len() > 0 {
		t.Errorf("expected no output, got: %s", buf.String())
	}

	// These should be logged
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if !strings.Contains(output, "[WARN] warn message") {
		t.Errorf("expected warn message in output, got: %s", output)
	}

	if !strings.Contains(output, "[ERROR] error message") {
		t.Errorf("expected error message in output, got: %s", output)
	}
}

// TestLoggerConsoleOutputFormat verifies the console output format
func TestLoggerConsoleOutputFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Info("test message")

	output := buf.String()
	if !strings.HasPrefix(output, "[INFO] ") {
		t.Errorf("expected [INFO] prefix, got: %s", output)
	}

	if !strings.Contains(output, "test message") {
		t.Errorf("expected message text, got: %s", output)
	}

	if !strings.HasSuffix(output, "\n") {
		t.Errorf("expected newline suffix, got: %s", output)
	}
}

// TestLoggerStructuredFields verifies structured field logging
func TestLoggerStructuredFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Info("test message", map[string]interface{}{
		"component": "test",
		"count":     42,
	})

	output := buf.String()

	if !strings.Contains(output, "component=test") {
		t.Errorf("expected component field, got: %s", output)
	}

	if !strings.Contains(output, "count=42") {
		t.Errorf("expected count field, got: %s", output)
	}
}

// TestLoggerStructuredFieldsOrder verifies that fields are output in sorted order
func TestLoggerStructuredFieldsOrder(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Info("test", map[string]interface{}{
		"zebra":  "z",
		"alpha":  "a",
		"middle": "m",
	})

	output := buf.String()

	// Fields should appear in alphabetical order
	alphaPos := strings.Index(output, "alpha=a")
	middlePos := strings.Index(output, "middle=m")
	zebraPos := strings.Index(output, "zebra=z")

	if alphaPos == -1 || middlePos == -1 || zebraPos == -1 {
		t.Fatalf("not all fields found in output: %s", output)
	}

	if !(alphaPos < middlePos && middlePos < zebraPos) {
		t.Errorf("fields not in alphabetical order: %s", output)
	}
}

// TestLoggerFormattedLogging tests the formatted logging methods
func TestLoggerFormattedLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Infof("formatted %s with number %d", "message", 123)

	output := buf.String()

	if !strings.Contains(output, "formatted message with number 123") {
		t.Errorf("formatted message not found, got: %s", output)
	}
}

// TestLoggerSetLevel verifies dynamic level changes
func TestLoggerSetLevel(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Debug("should not appear")

	if buf.Len() > 0 {
		t.Errorf("debug should not be logged at info level")
	}

	// Change to debug level
	logger.SetLevel(DebugLevel)
	logger.Debug("should appear")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG] should appear") {
		t.Errorf("debug message not found after level change, got: %s", output)
	}
}

// TestLoggerSetNodeConfig verifies node configuration
func TestLoggerSetNodeConfig(t *testing.T) {
	logger := NewLogger(InfoLevel)

	logger.SetNodeConfig("test-node", map[string]interface{}{
		"_role": "primary",
	})

	if logger.nodeConfig["_node"] != "test-node" {
		t.Errorf("expected _node=test-node, got %v", logger.nodeConfig["_node"])
	}

	if logger.nodeConfig["_role"] != "primary" {
		t.Errorf("expected _role=primary, got %v", logger.nodeConfig["_role"])
	}
}

// TestLoggerContext verifies context-based logging
func TestLoggerContext(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	ctx := logger.With(map[string]interface{}{
		"component": "reconciler",
		"service":   "test-svc",
	})

	ctx.Info("operation complete")

	output := buf.String()

	if !strings.Contains(output, "component=reconciler") {
		t.Errorf("expected component field, got: %s", output)
	}

	if !strings.Contains(output, "service=test-svc") {
		t.Errorf("expected service field, got: %s", output)
	}

	if !strings.Contains(output, "operation complete") {
		t.Errorf("expected message, got: %s", output)
	}
}

// TestLoggerContextAdditionalFields verifies context with additional fields
func TestLoggerContextAdditionalFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	ctx := logger.With(map[string]interface{}{
		"component": "health",
	})

	ctx.Info("check passed", map[string]interface{}{
		"backend": "192.168.1.10",
		"latency": "5ms",
	})

	output := buf.String()

	if !strings.Contains(output, "component=health") {
		t.Errorf("expected context field, got: %s", output)
	}

	if !strings.Contains(output, "backend=192.168.1.10") {
		t.Errorf("expected backend field, got: %s", output)
	}

	if !strings.Contains(output, "latency=5ms") {
		t.Errorf("expected latency field, got: %s", output)
	}
}

// TestLoggerConcurrentLogging verifies thread safety
func TestLoggerConcurrentLogging(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	var wg sync.WaitGroup
	numGoroutines := 100
	messagesPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				logger.Info("concurrent message", map[string]interface{}{
					"goroutine": id,
					"message":   j,
				})
			}
		}(i)
	}

	wg.Wait()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	expectedLines := numGoroutines * messagesPerGoroutine
	if len(lines) != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, len(lines))
	}

	// Verify all lines are properly formatted
	for i, line := range lines {
		if !strings.HasPrefix(line, "[INFO] ") {
			t.Errorf("line %d not properly formatted: %s", i, line)
		}
	}
}

// TestLoggerMultipleLevels verifies all log levels work correctly
func TestLoggerMultipleLevels(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(DebugLevel)
	logger.SetConsoleOutput(&buf)

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()

	expectedMessages := []string{
		"[DEBUG] debug message",
		"[INFO] info message",
		"[WARN] warn message",
		"[ERROR] error message",
	}

	for _, expected := range expectedMessages {
		if !strings.Contains(output, expected) {
			t.Errorf("expected %s in output, got: %s", expected, output)
		}
	}
}

// TestLoggerGELFDisabledByDefault verifies GELF is disabled by default
func TestLoggerGELFDisabledByDefault(t *testing.T) {
	logger := NewLogger(InfoLevel)

	if logger.gelfEnabled {
		t.Error("GELF should be disabled by default")
	}

	if logger.gelfWriter != nil {
		t.Error("GELF writer should be nil by default")
	}
}

// TestLoggerDisableGELF verifies GELF can be disabled
func TestLoggerDisableGELF(t *testing.T) {
	logger := NewLogger(InfoLevel)

	// Even without initializing GELF, DisableGELF should not panic
	logger.DisableGELF()

	if logger.gelfEnabled {
		t.Error("GELF should be disabled")
	}
}

// TestLoggerClose verifies logger can be closed safely
func TestLoggerClose(t *testing.T) {
	logger := NewLogger(InfoLevel)

	// Close without GELF initialized should not error
	err := logger.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestLoggerMergeFields tests field merging
func TestLoggerMergeFields(t *testing.T) {
	result := mergeFields(
		map[string]interface{}{"a": 1, "b": 2},
		map[string]interface{}{"c": 3},
		map[string]interface{}{"b": 4}, // Override
	)

	if result["a"] != 1 {
		t.Errorf("expected a=1, got %v", result["a"])
	}

	if result["b"] != 4 {
		t.Errorf("expected b=4 (overridden), got %v", result["b"])
	}

	if result["c"] != 3 {
		t.Errorf("expected c=3, got %v", result["c"])
	}
}

// TestLoggerEmptyFields verifies logging with no fields
func TestLoggerEmptyFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	logger.Info("message with no fields")

	output := buf.String()
	expected := "[INFO] message with no fields\n"

	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

// BenchmarkLogging benchmarks logging performance
func BenchmarkLogging(b *testing.B) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message", map[string]interface{}{
			"iteration": i,
		})
	}
}

// BenchmarkLoggingNoFields benchmarks logging without fields
func BenchmarkLoggingNoFields(b *testing.B) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("benchmark message")
	}
}

// BenchmarkLoggingFiltered benchmarks filtered logging
func BenchmarkLoggingFiltered(b *testing.B) {
	var buf bytes.Buffer
	logger := NewLogger(ErrorLevel)
	logger.SetConsoleOutput(&buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug("this will be filtered")
	}
}
