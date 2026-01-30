package moleman

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultAgents = `agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
    capture: [stdout, stderr, exitCode]
`

const defaultConfig = `version: 1

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      stdout: true
`

func Init(path string, force bool) error {
	if path == "" {
		return fmt.Errorf("config path is empty")
	}
	if !force {
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("config already exists: %s", path)
		}
	}

	configDir := filepath.Dir(path)
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	agentsPath := filepath.Join(configDir, "agents.yaml")
	if _, err := os.Stat(agentsPath); err == nil && !force {
		// Keep existing agents.yaml unless forced.
	} else {
		if err := os.WriteFile(agentsPath, []byte(defaultAgents), 0o644); err != nil {
			return fmt.Errorf("write agents.yaml: %w", err)
		}
	}
	if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
