package moleman

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoctorMissingConfig(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.yaml")
	if err := Doctor(missingPath); err == nil || !strings.Contains(err.Error(), "config not found") {
		t.Fatalf("expected missing config error, got %v", err)
	}
}

func TestDoctorAnnotatesPipelineErrors(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "moleman.yaml")
	config := `version: 1
agents: {}
workflow: []
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := Doctor(configPath)
	if err == nil {
		t.Fatalf("expected validation error")
	}
	if !strings.Contains(err.Error(), "agents map is empty") {
		t.Fatalf("expected agents validation error, got %v", err)
	}
}
