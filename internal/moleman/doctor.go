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
	if err := ValidateConfig(cfg); err != nil {
		return err
	}
	return nil
}
