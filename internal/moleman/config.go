package moleman

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	cfg := &Config{}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("parse yaml: %w", err)
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	if cfg.Steps == nil {
		cfg.Steps = map[string]Step{}
	}
	if cfg.Groups == nil {
		cfg.Groups = map[string][]Node{}
	}
	if cfg.Pipelines == nil {
		cfg.Pipelines = map[string]Pipeline{}
	}
	return cfg, nil
}

func PipelineNames(cfg *Config) []string {
	names := make([]string, 0, len(cfg.Pipelines))
	for name := range cfg.Pipelines {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func ConfigDir(path string) string {
	dir := filepath.Dir(path)
	if dir == "." {
		return ""
	}
	return dir
}
