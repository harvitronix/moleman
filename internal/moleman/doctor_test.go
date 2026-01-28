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
pipelines:
  default:
    plan:
      - type: ref
        id: missing
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	err := Doctor(configPath)
	if err == nil {
		t.Fatalf("expected pipeline error")
	}
	if !strings.Contains(err.Error(), "pipeline default") {
		t.Fatalf("expected pipeline name in error, got %v", err)
	}
}
