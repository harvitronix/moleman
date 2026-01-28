package moleman

import (
	"fmt"
	"os"
	"path/filepath"
)

const defaultConfig = `version: 1

steps:
  lint:
    type: run
    run: "pnpm lint"
    timeout: 10m
    capture: [stdout, stderr, exitCode]

  tests:
    type: run
    run: "pnpm test"
    timeout: 20m

groups:
  core:
    - type: ref
      id: lint
    - type: ref
      id: tests

pipelines:
  default:
    plan:
      - type: ref
        id: core
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
