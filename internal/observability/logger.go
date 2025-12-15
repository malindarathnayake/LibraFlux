package observability

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Graylog2/go-gelf/gelf"
)

// LogLevel represents the severity of a log message
type LogLevel int

const (
	DebugLevel LogLevel = iota
	InfoLevel
	WarnLevel
	ErrorLevel
)

// String returns the string representation of a log level
func (l LogLevel) String() string {
	switch l {
	case DebugLevel:
		return "DEBUG"
	case InfoLevel:
		return "INFO"
	case WarnLevel:
		return "WARN"
	case ErrorLevel:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// ParseLogLevel converts a string to a LogLevel
func ParseLogLevel(level string) (LogLevel, error) {
	switch strings.ToLower(level) {
	case "debug":
		return DebugLevel, nil
	case "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	default:
		return InfoLevel, fmt.Errorf("invalid log level: %s", level)
	}
}

// Logger provides dual-output logging (console + GELF)
type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	consoleOut  io.Writer
	gelfWriter  gelf.Writer
	gelfEnabled bool
	facility    string
	hostname    string
	nodeConfig  map[string]interface{} // Additional fields from config (node name, etc.)
}

// NewLogger creates a new logger with console output only
func NewLogger(level LogLevel) *Logger {
	hostname, _ := os.Hostname()
	
	return &Logger{
		level:       level,
		consoleOut:  os.Stdout,
		gelfEnabled: false,
		facility:    "lbctl",
		hostname:    hostname,
		nodeConfig:  make(map[string]interface{}),
	}
}

// SetLevel changes the minimum log level
func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// SetConsoleOutput sets the console output writer (useful for testing)
func (l *Logger) SetConsoleOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.consoleOut = w
}

// SetNodeConfig sets additional fields that should be included in all log entries
func (l *Logger) SetNodeConfig(nodeName string, additionalFields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	l.nodeConfig = make(map[string]interface{})
	l.nodeConfig["_node"] = nodeName
	
	for k, v := range additionalFields {
		l.nodeConfig[k] = v
	}
}

// InitGELF initializes GELF output to the specified host
// protocol can be "udp" or "tcp"
func (l *Logger) InitGELF(host string, port int, protocol, facility string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	address := fmt.Sprintf("%s:%d", host, port)
	
	var gw gelf.Writer
	
	if protocol == "tcp" {
		tcpWriter, err := gelf.NewTCPWriter(address)
		if err != nil {
			return fmt.Errorf("failed to create GELF TCP writer: %w", err)
		}
		tcpWriter.Facility = facility
		gw = tcpWriter
	} else {
		udpWriter, err := gelf.NewUDPWriter(address)
		if err != nil {
			return fmt.Errorf("failed to create GELF UDP writer: %w", err)
		}
		udpWriter.Facility = facility
		gw = udpWriter
	}
	
	l.gelfWriter = gw
	l.gelfEnabled = true
	l.facility = facility
	
	return nil
}

// DisableGELF disables GELF output
func (l *Logger) DisableGELF() {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.gelfWriter != nil {
		l.gelfWriter.Close()
		l.gelfWriter = nil
	}
	
	l.gelfEnabled = false
}

// Close closes any open GELF connections
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	if l.gelfWriter != nil {
		return l.gelfWriter.Close()
	}
	
	return nil
}

// log is the internal logging method
func (l *Logger) log(level LogLevel, msg string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	
	// Check if we should log at this level
	if level < l.level {
		return
	}
	
	// Console output
	l.logConsole(level, msg, fields)
	
	// GELF output (if enabled)
	if l.gelfEnabled && l.gelfWriter != nil {
		l.logGELF(level, msg, fields)
	}
}

// logConsole writes to console in format: [LEVEL] message key=value key=value
func (l *Logger) logConsole(level LogLevel, msg string, fields map[string]interface{}) {
	var sb strings.Builder
	
	sb.WriteString("[")
	sb.WriteString(level.String())
	sb.WriteString("] ")
	sb.WriteString(msg)
	
	// Sort keys for consistent output
	if len(fields) > 0 {
		keys := make([]string, 0, len(fields))
		for k := range fields {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		
		for _, k := range keys {
			sb.WriteString(" ")
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprintf("%v", fields[k]))
		}
	}
	
	sb.WriteString("\n")
	
	// Write to console output
	if l.consoleOut != nil {
		l.consoleOut.Write([]byte(sb.String()))
	}
}

// logGELF writes structured log to GELF
func (l *Logger) logGELF(level LogLevel, msg string, fields map[string]interface{}) {
	if l.gelfWriter == nil {
		return
	}
	
	// Map log level to GELF level
	var gelfLevel int32
	switch level {
	case DebugLevel:
		gelfLevel = 7 // Debug
	case InfoLevel:
		gelfLevel = 6 // Informational
	case WarnLevel:
		gelfLevel = 4 // Warning
	case ErrorLevel:
		gelfLevel = 3 // Error
	default:
		gelfLevel = 6
	}
	
	// Create GELF message
	gelfMsg := &gelf.Message{
		Version:  "1.1",
		Host:     l.hostname,
		Short:    msg,
		TimeUnix: float64(time.Now().Unix()),
		Level:    gelfLevel,
		Facility: l.facility,
		Extra:    make(map[string]interface{}),
	}
	
	// Add node config fields
	for k, v := range l.nodeConfig {
		gelfMsg.Extra[k] = v
	}
	
	// Add custom fields
	for k, v := range fields {
		// GELF requires custom fields to start with underscore
		if !strings.HasPrefix(k, "_") {
			k = "_" + k
		}
		gelfMsg.Extra[k] = v
	}
	
	// Send to GELF (ignore errors to not block logging)
	l.gelfWriter.WriteMessage(gelfMsg)
}

// Debug logs a debug message with optional structured fields
func (l *Logger) Debug(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(fields...)
	l.log(DebugLevel, msg, mergedFields)
}

// Info logs an info message with optional structured fields
func (l *Logger) Info(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(fields...)
	l.log(InfoLevel, msg, mergedFields)
}

// Warn logs a warning message with optional structured fields
func (l *Logger) Warn(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(fields...)
	l.log(WarnLevel, msg, mergedFields)
}

// Error logs an error message with optional structured fields
func (l *Logger) Error(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(fields...)
	l.log(ErrorLevel, msg, mergedFields)
}

// Debugf logs a formatted debug message
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.log(DebugLevel, fmt.Sprintf(format, args...), nil)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.log(InfoLevel, fmt.Sprintf(format, args...), nil)
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.log(WarnLevel, fmt.Sprintf(format, args...), nil)
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.log(ErrorLevel, fmt.Sprintf(format, args...), nil)
}

// With returns a new logger with additional context fields that will be included in all subsequent logs
func (l *Logger) With(fields map[string]interface{}) *LoggerContext {
	return &LoggerContext{
		logger: l,
		fields: fields,
	}
}

// mergeFields combines multiple field maps into one
func mergeFields(fieldMaps ...map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	
	for _, fm := range fieldMaps {
		for k, v := range fm {
			result[k] = v
		}
	}
	
	return result
}

// LoggerContext provides a logger with pre-set context fields
type LoggerContext struct {
	logger *Logger
	fields map[string]interface{}
}

// Debug logs a debug message with context fields
func (lc *LoggerContext) Debug(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(lc.fields)
	for _, f := range fields {
		for k, v := range f {
			mergedFields[k] = v
		}
	}
	lc.logger.log(DebugLevel, msg, mergedFields)
}

// Info logs an info message with context fields
func (lc *LoggerContext) Info(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(lc.fields)
	for _, f := range fields {
		for k, v := range f {
			mergedFields[k] = v
		}
	}
	lc.logger.log(InfoLevel, msg, mergedFields)
}

// Warn logs a warning message with context fields
func (lc *LoggerContext) Warn(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(lc.fields)
	for _, f := range fields {
		for k, v := range f {
			mergedFields[k] = v
		}
	}
	lc.logger.log(WarnLevel, msg, mergedFields)
}

// Error logs an error message with context fields
func (lc *LoggerContext) Error(msg string, fields ...map[string]interface{}) {
	mergedFields := mergeFields(lc.fields)
	for _, f := range fields {
		for k, v := range f {
			mergedFields[k] = v
		}
	}
	lc.logger.log(ErrorLevel, msg, mergedFields)
}
