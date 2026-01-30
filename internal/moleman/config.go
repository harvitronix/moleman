package moleman

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

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
	if cfg.Agents == nil {
		cfg.Agents = map[string]AgentConfig{}
	}
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func AgentNames(cfg *Config) []string {
	names := make([]string, 0, len(cfg.Agents))
	for name := range cfg.Agents {
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

func PrintWorkflow(workflow []WorkflowItem) error {
	raw, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow: %w", err)
	}
	_, err = os.Stdout.Write(raw)
	if err != nil {
		return fmt.Errorf("write workflow: %w", err)
	}
	_, _ = os.Stdout.Write([]byte("\n"))
	return nil
}

func ValidateConfig(cfg *Config) error {
	if len(cfg.Agents) == 0 {
		return fmt.Errorf("agents map is empty")
	}
	if len(cfg.Workflow) == 0 {
		return fmt.Errorf("workflow is empty")
	}
	for name, agent := range cfg.Agents {
		if agent.Type == "" {
			return fmt.Errorf("agent %s missing type", name)
		}
		switch agent.Type {
		case "codex", "claude", "generic":
		default:
			return fmt.Errorf("agent %s has unsupported type: %s", name, agent.Type)
		}
		if agent.Type == "generic" && agent.Command == "" {
			return fmt.Errorf("agent %s type generic requires command", name)
		}
	}
	seenNames := map[string]bool{}
	if err := validateWorkflow(cfg, cfg.Workflow, seenNames); err != nil {
		return err
	}
	return nil
}

func validateWorkflow(cfg *Config, items []WorkflowItem, seenNames map[string]bool) error {
	for idx, item := range items {
		switch item.Type {
		case "agent":
			if item.Agent == "" {
				return fmt.Errorf("workflow[%d] agent is required", idx)
			}
			if _, ok := cfg.Agents[item.Agent]; !ok {
				return fmt.Errorf("workflow[%d] references unknown agent: %s", idx, item.Agent)
			}
			if item.Name == "" {
				return fmt.Errorf("workflow[%d] name is required", idx)
			}
			if seenNames[item.Name] {
				return fmt.Errorf("duplicate workflow name: %s", item.Name)
			}
			seenNames[item.Name] = true
			if err := validateInput(item.Input, idx); err != nil {
				return err
			}
			if err := validateOutput(item.Output, idx); err != nil {
				return err
			}
		case "loop":
			if item.MaxIters <= 0 {
				return fmt.Errorf("workflow[%d] loop maxIters must be > 0", idx)
			}
			if strings.TrimSpace(item.Until) == "" {
				return fmt.Errorf("workflow[%d] loop until is required", idx)
			}
			if len(item.Body) == 0 {
				return fmt.Errorf("workflow[%d] loop body is empty", idx)
			}
			if err := validateWorkflow(cfg, item.Body, seenNames); err != nil {
				return err
			}
		default:
			return fmt.Errorf("workflow[%d] unknown type: %s", idx, item.Type)
		}
	}
	return nil
}

func validateInput(input InputSpec, idx int) error {
	count := 0
	if input.Prompt != "" {
		count++
	}
	if input.File != "" {
		count++
	}
	if input.From != "" {
		count++
	}
	if count == 0 {
		return fmt.Errorf("workflow[%d] input requires one of prompt, file, or from", idx)
	}
	if count > 1 {
		return fmt.Errorf("workflow[%d] input must specify only one of prompt, file, or from", idx)
	}
	return nil
}

func validateOutput(output OutputSpec, idx int) error {
	count := 0
	if output.ToNext {
		count++
	}
	if output.File != "" {
		count++
	}
	if output.Stdout {
		count++
	}
	if count == 0 {
		return fmt.Errorf("workflow[%d] output requires one of toNext, file, or stdout", idx)
	}
	if count > 1 {
		return fmt.Errorf("workflow[%d] output must specify only one of toNext, file, or stdout", idx)
	}
	return nil
}
