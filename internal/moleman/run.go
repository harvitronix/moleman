package moleman

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

type RunOptions struct {
	Prompt     string
	PromptFile string
	Workdir    string
	DryRun     bool
	Verbose    bool
}

type RunResult struct {
	RunDir string
}

func Run(cfg *Config, cfgPath string, opts RunOptions) (*RunResult, error) {
	workdir := opts.Workdir
	if workdir == "" {
		workdir = ConfigDir(cfgPath)
		if workdir == "" {
			workdir = "."
		}
	}

	input, err := loadPrompt(opts.Prompt, opts.PromptFile)
	if err != nil {
		return nil, err
	}

	runID := fmt.Sprintf("%s-workflow", time.Now().Format("20060102-150405"))
	runDir := filepath.Join(workdir, ".moleman", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}

	if err := writeArtifactsSkeleton(runDir, input, cfg.Workflow); err != nil {
		return &RunResult{RunDir: runDir}, err
	}

	ctx := &RunContext{
		Input:       input,
		Outputs:     map[string]any{},
		Sessions:    map[string]string{},
		RunDir:      runDir,
		Workdir:     workdir,
		Verbose:     opts.Verbose,
		NodeResults: []NodeResult{},
	}

	if err := ensureAgentCommands(cfg, ctx.Workdir); err != nil {
		writeSummary(runDir, "failed", err, ctx)
		return &RunResult{RunDir: runDir}, err
	}

	log.Info("run started", "nodes", len(cfg.Workflow))
	log.Info("run artifacts", "path", runDir)

	if opts.DryRun {
		if err := writeSummary(runDir, "dry-run", nil, ctx); err != nil {
			return &RunResult{RunDir: runDir}, err
		}
		return &RunResult{RunDir: runDir}, nil
	}

	if err := executeWorkflow(ctx, cfg, cfg.Workflow); err != nil {
		writeSummary(runDir, "failed", err, ctx)
		return &RunResult{RunDir: runDir}, err
	}

	if err := writeSummary(runDir, "success", nil, ctx); err != nil {
		return &RunResult{RunDir: runDir}, err
	}

	return &RunResult{RunDir: runDir}, nil
}

func ensureAgentCommands(cfg *Config, workdir string) error {
	usedAgents := map[string]struct{}{}
	collectAgentNames(cfg.Workflow, usedAgents)
	for name := range usedAgents {
		agent, ok := cfg.Agents[name]
		if !ok {
			continue
		}
		command := resolveAgentCommand(agent)
		if command == "" {
			return fmt.Errorf("agent %s has no command configured", name)
		}
		if err := commandAvailable(command, workdir); err != nil {
			return fmt.Errorf("agent %s command not found: %s (%w)", name, command, err)
		}
	}
	return nil
}

func collectAgentNames(items []WorkflowItem, used map[string]struct{}) {
	for _, item := range items {
		switch item.Type {
		case "agent":
			if item.Agent != "" {
				used[item.Agent] = struct{}{}
			}
		case "loop":
			collectAgentNames(item.Body, used)
		}
	}
}

func resolveAgentCommand(agent AgentConfig) string {
	if agent.Command != "" {
		return agent.Command
	}
	switch agent.Type {
	case "codex":
		return "codex"
	case "claude":
		return "claude"
	default:
		return ""
	}
}

func commandAvailable(command, workdir string) error {
	if filepath.IsAbs(command) {
		if _, err := os.Stat(command); err != nil {
			return err
		}
		return nil
	}
	if strings.Contains(command, "/") {
		path := command
		if !filepath.IsAbs(path) && workdir != "" {
			path = filepath.Join(workdir, path)
		}
		if _, err := os.Stat(path); err != nil {
			return err
		}
		return nil
	}
	_, err := exec.LookPath(command)
	return err
}

func writeArtifactsSkeleton(runDir string, input string, workflow []WorkflowItem) error {
	if err := os.WriteFile(filepath.Join(runDir, "input.md"), []byte(input), 0o644); err != nil {
		return fmt.Errorf("write input.md: %w", err)
	}
	rawPlan, err := json.MarshalIndent(workflow, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal workflow: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "resolved-workflow.json"), rawPlan, 0o644); err != nil {
		return fmt.Errorf("write resolved workflow: %w", err)
	}
	dirs := []string{
		filepath.Join(runDir, "nodes"),
		filepath.Join(runDir, "diffs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func writeSummary(runDir, status string, err error, ctx *RunContext) error {
	summary := struct {
		Status string       `json:"status"`
		Error  string       `json:"error,omitempty"`
		Time   string       `json:"time"`
		Nodes  []NodeResult `json:"nodes"`
	}{
		Status: status,
		Time:   time.Now().Format(time.RFC3339),
		Nodes:  ctx.NodeResults,
	}
	if err != nil {
		summary.Error = err.Error()
	}
	raw, _ := json.MarshalIndent(summary, "", "  ")
	content := "# moleman Run Summary\n\n" + string(raw) + "\n"
	return os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(content), 0o644)
}

func loadPrompt(prompt, promptFile string) (string, error) {
	if prompt != "" && promptFile != "" {
		return "", errors.New("provide only one of --prompt or --prompt-file")
	}
	if promptFile != "" {
		raw, err := os.ReadFile(promptFile)
		if err != nil {
			return "", fmt.Errorf("read prompt file: %w", err)
		}
		return string(raw), nil
	}
	return prompt, nil
}
