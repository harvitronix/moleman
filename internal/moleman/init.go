package moleman

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `version: 1

agents:
  codex:
    type: codex
    args: ["--full-auto"]
    timeout: 45m
    capture: [stdout, stderr, exitCode]

workflow:
  - type: agent
    name: write
    agent: codex
    input:
      from: input
    output:
      toNext: true
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

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(defaultConfig), 0o644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}
