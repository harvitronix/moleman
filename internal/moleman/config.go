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
	baseAgents, err := loadBaseAgents(path)
	if err != nil {
		return nil, err
	}
	if cfg.Version != 1 {
		return nil, fmt.Errorf("unsupported config version: %d", cfg.Version)
	}
	if cfg.Agents == nil {
		cfg.Agents = map[string]AgentConfig{}
	}
	mergedAgents, err := mergeAgents(baseAgents, cfg.Agents)
	if err != nil {
		return nil, err
	}
	cfg.Agents = mergedAgents
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
		if agent.Model != "" && agent.Type == "generic" {
			return fmt.Errorf("agent %s model is only supported for codex or claude", name)
		}
		if agent.Thinking != "" && agent.Type != "codex" {
			return fmt.Errorf("agent %s thinking is only supported for codex", name)
		}
		if agent.Thinking != "" && !isValidCodexThinking(agent.Thinking) {
			return fmt.Errorf("agent %s thinking must be one of minimal, low, medium, high, xhigh", name)
		}
	}
	seenNames := map[string]bool{}
	if err := validateWorkflow(cfg, cfg.Workflow, seenNames); err != nil {
		return err
	}
	return nil
}

func isValidCodexThinking(value string) bool {
	switch value {
	case "minimal", "low", "medium", "high", "xhigh":
		return true
	default:
		return false
	}
}

func loadBaseAgents(configPath string) (map[string]AgentConfig, error) {
	dir := ConfigDir(configPath)
	if dir == "" {
		dir = "."
	}
	agentsPath := filepath.Join(dir, "agents.yaml")
	if _, err := os.Stat(agentsPath); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agents.yaml not found: %s", agentsPath)
		}
		return nil, fmt.Errorf("stat agents.yaml: %w", err)
	}
	raw, err := os.ReadFile(agentsPath)
	if err != nil {
		return nil, fmt.Errorf("read agents.yaml: %w", err)
	}
	var payload struct {
		Agents map[string]AgentConfig `yaml:"agents"`
	}
	if err := yaml.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("parse agents.yaml: %w", err)
	}
	if payload.Agents == nil {
		payload.Agents = map[string]AgentConfig{}
	}
	return payload.Agents, nil
}

func mergeAgents(base, overrides map[string]AgentConfig) (map[string]AgentConfig, error) {
	merged := map[string]AgentConfig{}
	for name, agent := range base {
		merged[name] = agent
	}
	for name, agent := range overrides {
		baseAgent := AgentConfig{}
		if agent.Extends != "" {
			extended, ok := base[agent.Extends]
			if !ok {
				return nil, fmt.Errorf("agent %s extends unknown agent: %s", name, agent.Extends)
			}
			baseAgent = extended
		} else if existing, ok := merged[name]; ok {
			baseAgent = existing
		}
		merged[name] = mergeAgentConfig(baseAgent, agent)
	}
	return merged, nil
}

func mergeAgentConfig(base, override AgentConfig) AgentConfig {
	result := base
	if override.Type != "" {
		result.Type = override.Type
	}
	if override.Command != "" {
		result.Command = override.Command
	}
	if override.Model != "" {
		result.Model = override.Model
	}
	if override.Thinking != "" {
		result.Thinking = override.Thinking
	}
	if override.Args != nil {
		result.Args = override.Args
	}
	if override.OutputSchema != "" {
		result.OutputSchema = override.OutputSchema
	}
	if override.OutputFile != "" {
		result.OutputFile = override.OutputFile
	}
	if override.Env != nil {
		if result.Env == nil {
			result.Env = map[string]string{}
		}
		for key, value := range override.Env {
			result.Env[key] = value
		}
	}
	if override.Timeout != "" {
		result.Timeout = override.Timeout
	}
	if override.Capture != nil {
		result.Capture = override.Capture
	}
	if override.Print != nil {
		result.Print = override.Print
	}
	if override.Session != nil {
		result.Session = override.Session
	}
	result.Extends = ""
	return result
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
