package moleman

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/log"
)

func executeNodes(ctx *RunContext, nodes []Node, inLoop bool) error {
	for _, node := range nodes {
		switch node.Type {
		case "run":
			if err := executeRunNode(ctx, node, inLoop); err != nil {
				return err
			}
		case "loop":
			if err := executeLoop(ctx, node); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unexpected node type at execution: %s", node.Type)
		}
	}
	return nil
}

func executeLoop(ctx *RunContext, node Node) error {
	for i := 0; i < node.MaxIters; i++ {
		if ctx.Verbose {
			log.Debugf("loop iteration %d/%d", i+1, node.MaxIters)
		}
		if err := executeNodes(ctx, node.Body, true); err != nil {
			return err
		}

		data := ctx.TemplateData()
		cond, err := EvalCondition(node.Until, data)
		if err != nil {
			return fmt.Errorf("loop condition: %w", err)
		}
		if cond {
			log.Info("loop condition met", "iteration", i+1, "max", node.MaxIters)
			return nil
		}
	}
	return fmt.Errorf("loop exhausted without meeting condition")
}

func executeRunNode(ctx *RunContext, node Node, inLoop bool) error {
	id := node.ID
	if id == "" {
		id = fmt.Sprintf("inline-%d", len(ctx.StepsHistory)+1)
	}

	templated, err := renderRunNode(ctx, node)
	if err != nil {
		return err
	}

	iteration := len(ctx.StepsHistory[id]) + 1
	command := strings.TrimSpace(templated.Run)
	if command == "" {
		command = "<empty>"
	}
	log.Info("step start", "step", id, "iteration", fmt.Sprintf("%02d", iteration), "command", command)

	result, err := runCommand(ctx, id, templated)
	if err != nil {
		return err
	}

	if result.ExitCode != 0 && !inLoop {
		return fmt.Errorf("step failed: %s (exit %d)", id, result.ExitCode)
	}
	return nil
}

func renderRunNode(ctx *RunContext, node Node) (Node, error) {
	data := ctx.TemplateData()
	out := node
	var err error

	out.Run, err = RenderTemplate(node.Run, data)
	if err != nil {
		return node, err
	}
	out.Stdin, err = RenderTemplate(node.Stdin, data)
	if err != nil {
		return node, err
	}
	out.StdinFile, err = RenderTemplate(node.StdinFile, data)
	if err != nil {
		return node, err
	}
	out.Until, err = RenderTemplate(node.Until, data)
	if err != nil {
		return node, err
	}

	if node.Env != nil {
		out.Env = map[string]string{}
		for key, value := range node.Env {
			rendered, err := RenderTemplate(value, data)
			if err != nil {
				return node, err
			}
			out.Env[key] = rendered
		}
	}

	return out, nil
}

func runCommand(ctx *RunContext, id string, node Node) (StepResult, error) {
	if node.Run == "" {
		return StepResult{}, fmt.Errorf("run node missing command")
	}
	if node.Stdin != "" && node.StdinFile != "" {
		return StepResult{}, fmt.Errorf("step %s: both stdin and stdinFile provided", id)
	}

	shell := node.Shell
	if shell == "" {
		shell = os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/sh"
		}
	}

	var timeout time.Duration
	if node.Timeout != "" {
		parsed, err := time.ParseDuration(node.Timeout)
		if err != nil {
			return StepResult{}, fmt.Errorf("step %s timeout: %w", id, err)
		}
		timeout = parsed
	}

	ctxExec := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctxExec, cancel = context.WithTimeout(ctxExec, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctxExec, shell, "-c", node.Run)
	cmd.Dir = ctx.Workdir
	cmd.Env = buildEnv(node.Env)

	stdin, err := resolveStdin(node.Stdin, node.StdinFile)
	if err != nil {
		return StepResult{}, err
	}
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}

	stepDir, err := stepRunDir(ctx.RunDir, id, len(ctx.StepsHistory[id])+1)
	if err != nil {
		return StepResult{}, err
	}

	stdoutPath := filepath.Join(stepDir, "stdout.log")
	stderrPath := filepath.Join(stepDir, "stderr.log")
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return StepResult{}, fmt.Errorf("create stdout log: %w", err)
	}
	defer stdoutFile.Close()
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return StepResult{}, fmt.Errorf("create stderr log: %w", err)
	}
	defer stderrFile.Close()

	captureStdout := shouldCapture(node.Capture, "stdout")
	captureStderr := shouldCapture(node.Capture, "stderr")

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer

	cmd.Stdout = writerFor(stdoutFile, &stdoutBuf, captureStdout)
	cmd.Stderr = writerFor(stderrFile, &stderrBuf, captureStderr)

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	exitCode := 0
	if runErr != nil {
		exitCode = exitCodeFromErr(runErr)
	}

	if ctxExec.Err() == context.DeadlineExceeded {
		exitCode = 124
	}

	result := StepResult{
		Stdout:   stdoutBuf.String(),
		Stderr:   stderrBuf.String(),
		ExitCode: exitCode,
		Duration: duration.String(),
		Command:  node.Run,
	}

	ctx.Steps[id] = result
	ctx.StepsHistory[id] = append(ctx.StepsHistory[id], result)
	ctx.StepOrder = append(ctx.StepOrder, StepExecution{ID: id, Iteration: len(ctx.StepsHistory[id])})

	if err := writeMeta(stepDir, result); err != nil {
		return result, err
	}
	if node.Parse != nil && node.Parse.Kind == "json" {
		if err := parseJSONInto(ctx, node.Parse.Into, stdoutBuf.Bytes(), stdoutPath); err != nil {
			return result, err
		}
	}

	log.Info("step done", "step", id, "iteration", fmt.Sprintf("%02d", len(ctx.StepsHistory[id])), "exit", exitCode, "duration", duration.String())

	return result, nil
}

func resolveStdin(stdin, stdinFile string) (string, error) {
	if stdinFile == "" {
		return stdin, nil
	}
	raw, err := os.ReadFile(stdinFile)
	if err != nil {
		return "", fmt.Errorf("read stdin file: %w", err)
	}
	return string(raw), nil
}

func writerFor(file *os.File, buf *bytes.Buffer, capture bool) io.Writer {
	if capture {
		return io.MultiWriter(file, buf)
	}
	return file
}

func shouldCapture(capture []string, value string) bool {
	if len(capture) == 0 {
		return true
	}
	for _, item := range capture {
		if item == value {
			return true
		}
	}
	return false
}

func stepRunDir(runDir, id string, iteration int) (string, error) {
	dir := filepath.Join(runDir, "steps", id, fmt.Sprintf("%02d", iteration))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create step dir: %w", err)
	}
	return dir, nil
}

func writeMeta(stepDir string, result StepResult) error {
	raw, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(filepath.Join(stepDir, "meta.json"), raw, 0o644)
}

func parseJSONInto(ctx *RunContext, key string, stdout []byte, stdoutPath string) error {
	if key == "" {
		return fmt.Errorf("parse config missing 'into'")
	}
	if len(stdout) == 0 {
		loaded, err := os.ReadFile(stdoutPath)
		if err != nil {
			return fmt.Errorf("read stdout: %w", err)
		}
		stdout = loaded
	}
	var value any
	if err := json.Unmarshal(stdout, &value); err != nil {
		return fmt.Errorf("parse json: %w", err)
	}
	ctx.Vars[key] = value
	return nil
}

func buildEnv(extra map[string]string) []string {
	env := os.Environ()
	for key, value := range extra {
		env = append(env, fmt.Sprintf("%s=%s", key, value))
	}
	return env
}

func exitCodeFromErr(err error) int {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}
