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

agents:
  echo:
    type: generic
    command: "printf"
    args: []
    capture: [stdout, stderr, exitCode]

workflow:
  - type: agent
    name: first
    agent: echo
    input:
      prompt: "hello"
    output:
      toNext: true
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	result, err := Run(cfg, configPath, RunOptions{})
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

	metaPath := filepath.Join(result.RunDir, "nodes", "first", "meta.json")
	if _, err := os.Stat(metaPath); err != nil {
		t.Fatalf("missing step meta: %v", err)
	}
}

func TestRunLoopStopsOnCondition(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "moleman.yaml")
	config := `version: 1

agents:
  fail:
    type: generic
    command: "false"
    capture: [stdout, stderr, exitCode]

workflow:
  - type: loop
    maxIters: 2
    until: "outputs.__previous__ == \"ok\""
    body:
      - type: agent
        name: fail_once
        agent: fail
        input:
          prompt: "ignored"
        output:
          toNext: true
`
	if err := os.WriteFile(configPath, []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if _, err := Run(cfg, configPath, RunOptions{}); err == nil {
		t.Fatalf("expected loop exhaustion error")
	}
}
