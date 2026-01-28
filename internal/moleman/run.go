package moleman

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type RunOptions struct {
	Pipeline   string
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
	plan, err := ResolvePlan(cfg, opts.Pipeline)
	if err != nil {
		return nil, err
	}

	workdir := opts.Workdir
	if workdir == "" {
		workdir = ConfigDir(cfgPath)
		if workdir == "" {
			workdir = "."
		}
	}

	prompt, err := loadPrompt(opts.Prompt, opts.PromptFile)
	if err != nil {
		return nil, err
	}

	runID := fmt.Sprintf("%s-%s", time.Now().Format("20060102-150405"), opts.Pipeline)
	runDir := filepath.Join(workdir, ".moleman", "runs", runID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, fmt.Errorf("create run dir: %w", err)
	}

	if err := writeArtifactsSkeleton(runDir, prompt, plan); err != nil {
		return &RunResult{RunDir: runDir}, err
	}

	ctx := &RunContext{
		Input: InputData{
			Prompt: prompt,
		},
		Git:          LoadGitData(workdir),
		Steps:        map[string]StepResult{},
		StepsHistory: map[string][]StepResult{},
		Vars:         map[string]any{},
		RunDir:       runDir,
		Workdir:      workdir,
		Verbose:      opts.Verbose,
	}

	if opts.DryRun {
		if err := writeSummary(runDir, opts.Pipeline, "dry-run", nil); err != nil {
			return &RunResult{RunDir: runDir}, err
		}
		return &RunResult{RunDir: runDir}, nil
	}

	err = executeNodes(ctx, plan, false)
	if err != nil {
		writeSummary(runDir, opts.Pipeline, "failed", err)
		return &RunResult{RunDir: runDir}, err
	}

	if err := writeSummary(runDir, opts.Pipeline, "success", nil); err != nil {
		return &RunResult{RunDir: runDir}, err
	}

	return &RunResult{RunDir: runDir}, nil
}

func writeArtifactsSkeleton(runDir string, prompt string, plan []Node) error {
	if err := os.WriteFile(filepath.Join(runDir, "input.md"), []byte(prompt), 0o644); err != nil {
		return fmt.Errorf("write input.md: %w", err)
	}
	rawPlan, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "resolved-plan.json"), rawPlan, 0o644); err != nil {
		return fmt.Errorf("write resolved plan: %w", err)
	}
	dirs := []string{
		filepath.Join(runDir, "steps"),
		filepath.Join(runDir, "diffs"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}
	return nil
}

func writeSummary(runDir, pipeline, status string, err error) error {
	summary := struct {
		Pipeline string `json:"pipeline"`
		Status   string `json:"status"`
		Error    string `json:"error,omitempty"`
		Time     string `json:"time"`
	}{
		Pipeline: pipeline,
		Status:   status,
		Time:     time.Now().Format(time.RFC3339),
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
