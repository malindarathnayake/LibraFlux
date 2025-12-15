package observability

import (
	"bytes"
	"strings"
	"testing"
)

func TestAuditEmit(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	auditor := NewAuditor(logger).WithComponent("config")

	auditor.Emit(AuditConfigLoaded, map[string]interface{}{
		"config_hash":    "a1b2c3d4",
		"services_count": 3,
		"backends_count": 6,
	})

	output := buf.String()

	expectedStrings := []string{
		"[INFO] AUDIT",
		"_event_type=audit",
		"_audit_event=config_loaded",
		"_component=config",
		"config_hash=a1b2c3d4",
		"services_count=3",
		"backends_count=6",
	}

	for _, expected := range expectedStrings {
		if !strings.Contains(output, expected) {
			t.Errorf("expected output to contain %q, got %q", expected, output)
		}
	}
}

func TestAuditReservedFieldsCannotBeOverridden(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	auditor := NewAuditor(logger).WithComponent("vip")

	auditor.Emit(AuditVIPAcquired, map[string]interface{}{
		"_event_type":  "not_audit",
		"_audit_event": "not_vip_acquired",
		"_component":   "not_vip",
		"vip":          "192.168.94.250",
	})

	output := buf.String()

	if strings.Contains(output, "not_audit") || strings.Contains(output, "not_vip_acquired") || strings.Contains(output, "not_vip") {
		t.Fatalf("expected reserved fields to be overwritten, got %q", output)
	}
	if !strings.Contains(output, "_event_type=audit") || !strings.Contains(output, "_audit_event=vip_acquired") || !strings.Contains(output, "_component=vip") {
		t.Fatalf("expected reserved fields to be set, got %q", output)
	}
	if !strings.Contains(output, "vip=192.168.94.250") {
		t.Fatalf("expected custom field to be preserved, got %q", output)
	}
}

func TestAuditNilFields(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLogger(InfoLevel)
	logger.SetConsoleOutput(&buf)

	auditor := NewAuditor(logger).WithComponent("vip")
	auditor.Emit(AuditVIPReleased, nil)

	output := buf.String()
	if !strings.Contains(output, "_event_type=audit") || !strings.Contains(output, "_audit_event=vip_released") {
		t.Fatalf("expected audit fields, got %q", output)
	}
}
