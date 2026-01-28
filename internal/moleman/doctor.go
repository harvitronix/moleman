package moleman

import (
	"fmt"
	"os"
)

func Doctor(configPath string) error {
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config not found: %s", configPath)
	}
	cfg, err := LoadConfig(configPath)
	if err != nil {
		return err
	}
	for name := range cfg.Pipelines {
		if _, err := ResolvePlan(cfg, name); err != nil {
			return fmt.Errorf("pipeline %s: %w", name, err)
		}
	}
	if _, err := os.Stat("/bin/sh"); err != nil && os.Getenv("SHELL") == "" {
		return fmt.Errorf("shell not found; set SHELL or ensure /bin/sh exists")
	}
	return nil
}
