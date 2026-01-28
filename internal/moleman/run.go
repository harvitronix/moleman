package moleman

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/log"
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
		StepOrder:    []StepExecution{},
	}

	log.Info("run started", "pipeline", opts.Pipeline)
	log.Info("run artifacts", "path", runDir)

	if opts.DryRun {
		if err := writeSummary(runDir, opts.Pipeline, "dry-run", nil, ctx); err != nil {
			return &RunResult{RunDir: runDir}, err
		}
		printSummary(os.Stdout, opts.Pipeline, "dry-run", nil, ctx)
		return &RunResult{RunDir: runDir}, nil
	}

	err = executeNodes(ctx, plan, false)
	if err != nil {
		writeSummary(runDir, opts.Pipeline, "failed", err, ctx)
		printSummary(os.Stderr, opts.Pipeline, "failed", err, ctx)
		return &RunResult{RunDir: runDir}, err
	}

	if err := writeSummary(runDir, opts.Pipeline, "success", nil, ctx); err != nil {
		return &RunResult{RunDir: runDir}, err
	}
	printSummary(os.Stdout, opts.Pipeline, "success", nil, ctx)

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

type StepIterationSummary struct {
	Iteration int    `json:"iteration"`
	ExitCode  int    `json:"exitCode"`
	Duration  string `json:"duration"`
	Command   string `json:"command"`
	StdoutLog string `json:"stdoutLog"`
	StderrLog string `json:"stderrLog"`
}

type StepSummary struct {
	ID         string                 `json:"id"`
	Iterations []StepIterationSummary `json:"iterations"`
}

type RunSummary struct {
	Pipeline string        `json:"pipeline"`
	Status   string        `json:"status"`
	Error    string        `json:"error,omitempty"`
	Time     string        `json:"time"`
	RunDir   string        `json:"runDir"`
	Steps    []StepSummary `json:"steps,omitempty"`
}

func writeSummary(runDir, pipeline, status string, err error, ctx *RunContext) error {
	summary := buildSummary(runDir, pipeline, status, err, ctx)
	raw, _ := json.MarshalIndent(summary, "", "  ")
	content := "# moleman Run Summary\n\n" + string(raw) + "\n"
	return os.WriteFile(filepath.Join(runDir, "summary.md"), []byte(content), 0o644)
}

func buildSummary(runDir, pipeline, status string, err error, ctx *RunContext) RunSummary {
	summary := RunSummary{
		Pipeline: pipeline,
		Status:   status,
		Time:     time.Now().Format(time.RFC3339),
		RunDir:   runDir,
	}
	if err != nil {
		summary.Error = err.Error()
	}
	if ctx == nil {
		return summary
	}
	index := map[string]int{}
	for _, exec := range ctx.StepOrder {
		stepIdx, ok := index[exec.ID]
		if !ok {
			stepIdx = len(summary.Steps)
			index[exec.ID] = stepIdx
			summary.Steps = append(summary.Steps, StepSummary{ID: exec.ID})
		}
		history := ctx.StepsHistory[exec.ID]
		if exec.Iteration-1 >= len(history) || exec.Iteration <= 0 {
			continue
		}
		result := history[exec.Iteration-1]
		iterSummary := StepIterationSummary{
			Iteration: exec.Iteration,
			ExitCode:  result.ExitCode,
			Duration:  result.Duration,
			Command:   result.Command,
			StdoutLog: filepath.Join("steps", exec.ID, fmt.Sprintf("%02d", exec.Iteration), "stdout.log"),
			StderrLog: filepath.Join("steps", exec.ID, fmt.Sprintf("%02d", exec.Iteration), "stderr.log"),
		}
		summary.Steps[stepIdx].Iterations = append(summary.Steps[stepIdx].Iterations, iterSummary)
	}
	return summary
}

func printSummary(out *os.File, pipeline, status string, err error, ctx *RunContext) {
	fmt.Fprintln(out, "run summary:")
	fmt.Fprintf(out, "  pipeline: %s\n", pipeline)
	fmt.Fprintf(out, "  status: %s\n", status)
	if err != nil {
		fmt.Fprintf(out, "  error: %s\n", err.Error())
	}
	if ctx == nil || len(ctx.StepOrder) == 0 {
		fmt.Fprintln(out, "  steps: none")
		return
	}
	fmt.Fprintln(out, "  steps:")
	for _, exec := range ctx.StepOrder {
		history := ctx.StepsHistory[exec.ID]
		if exec.Iteration-1 >= len(history) || exec.Iteration <= 0 {
			continue
		}
		result := history[exec.Iteration-1]
		fmt.Fprintf(out, "    - %s.%02d exit %d (%s)\n", exec.ID, exec.Iteration, result.ExitCode, result.Duration)
	}
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
