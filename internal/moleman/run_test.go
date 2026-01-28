package moleman

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRunExecutesStepsAndWritesArtifacts(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "moleman.yaml")
	config := `version: 1

steps:
  first:
    type: run
    run: "printf 'hello'"
    capture: [stdout, stderr, exitCode]

pipelines:
  default:
    plan:
      - type: ref
        id: first
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	result, err := Run(cfg, configPath, RunOptions{Pipeline: "default"})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if result.RunDir == "" {
		t.Fatalf("missing run dir")
	}

	summaryPath := filepath.Join(result.RunDir, "summary.md")
	if _, err := os.Stat(summaryPath); err != nil {
		t.Fatalf("missing summary: %v", err)
	}

	metaPath := filepath.Join(result.RunDir, "steps", "first", "01", "meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("missing step meta: %v", err)
	}
}

func TestRunLoopStopsOnCondition(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "moleman.yaml")
	config := `version: 1

steps:
  fail:
    type: run
    run: "exit 1"
    capture: [stdout, stderr, exitCode]

pipelines:
  default:
    plan:
      - type: loop
        maxIters: 2
        until: "steps.fail.exitCode == 0"
        body:
          - type: ref
            id: fail
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, err := Run(cfg, configPath, RunOptions{Pipeline: "default"}); err == nil {
		t.Fatalf("expected loop exhaustion error")
	}
}
