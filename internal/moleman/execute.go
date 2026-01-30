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

func executeWorkflow(ctx *RunContext, cfg *Config, items []WorkflowItem) error {
	for _, item := range items {
		switch item.Type {
		case "agent":
			if err := executeAgentNode(ctx, cfg, item); err != nil {
				return err
			}
		case "loop":
			if err := executeLoop(ctx, cfg, item); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown workflow type: %s", item.Type)
		}
	}
	return nil
}

func executeLoop(ctx *RunContext, cfg *Config, item WorkflowItem) error {
	for i := 0; i < item.MaxIters; i++ {
		if ctx.Verbose {
			log.Debugf("loop iteration %d/%d", i+1, item.MaxIters)
		}
		if err := executeWorkflow(ctx, cfg, item.Body); err != nil {
			return err
		}
		cond, err := EvalCondition(item.Until, ctx.TemplateData())
		if err != nil {
			return fmt.Errorf("loop condition: %w", err)
		}
		if cond {
			log.Info("loop condition met", "iteration", i+1, "max", item.MaxIters)
			return nil
		}
	}
	return fmt.Errorf("loop exhausted without meeting condition")
}

func executeAgentNode(ctx *RunContext, cfg *Config, item WorkflowItem) error {
	agent, ok := cfg.Agents[item.Agent]
	if !ok {
		return fmt.Errorf("unknown agent: %s", item.Agent)
	}

	input, err := resolveInput(ctx, item.Input)
	if err != nil {
		return err
	}

	command, args, err := buildAgentCommand(ctx, agent, item, input)
	if err != nil {
		return err
	}

	stepDir, err := nodeRunDir(ctx.RunDir, item.Name)
	if err != nil {
		return err
	}

	stdoutBuf, stderrBuf, exitCode, duration, err := runCommand(ctx, item.Name, item.Agent, command, args, agent, stepDir, input)
	if err != nil {
		return err
	}

	if err := handleOutput(ctx, item, stdoutBuf.Bytes()); err != nil {
		return err
	}

	if agent.Type == "claude" {
		updateClaudeSession(ctx, stdoutBuf.Bytes())
	}

	ctx.NodeResults = append(ctx.NodeResults, NodeResult{
		Name:     item.Name,
		Agent:    item.Agent,
		ExitCode: exitCode,
		Duration: duration,
		Command:  strings.Join(append([]string{command}, args...), " "),
	})

	if exitCode != 0 {
		stderrSummary := summarizeStderr(stderrBuf)
		stderrPath := filepath.Join(stepDir, "stderr.log")
		if stderrSummary != "" {
			return fmt.Errorf("node failed: %s (exit %d). stderr: %s (see %s)", item.Name, exitCode, stderrSummary, stderrPath)
		}
		return fmt.Errorf("node failed: %s (exit %d). see %s", item.Name, exitCode, stderrPath)
	}

	log.Info("node done", "name", item.Name, "agent", item.Agent, "exit", exitCode, "duration", duration)
	return nil
}

func resolveInput(ctx *RunContext, input InputSpec) (string, error) {
	data := ctx.TemplateData()
	if input.Prompt != "" {
		return RenderTemplate(input.Prompt, data)
	}
	if input.File != "" {
		path, err := RenderTemplate(input.File, data)
		if err != nil {
			return "", err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read input file: %w", err)
		}
		return string(raw), nil
	}
	if input.From != "" {
		switch input.From {
		case "previous", "prev":
			return outputAsString(ctx.Outputs["__previous__"])
		case "input":
			return ctx.Input, nil
		default:
			value, ok := ctx.Outputs[input.From]
			if !ok {
				return "", fmt.Errorf("input from unknown node: %s", input.From)
			}
			return outputAsString(value)
		}
	}
	return "", fmt.Errorf("input is empty")
}

func buildAgentCommand(ctx *RunContext, agent AgentConfig, item WorkflowItem, input string) (string, []string, error) {
	command := agent.Command
	if command == "" {
		switch agent.Type {
		case "codex":
			command = "codex"
		case "claude":
			command = "claude"
		default:
			return "", nil, fmt.Errorf("unsupported agent type: %s", agent.Type)
		}
	}

	session := effectiveSession(agent.Session, item.Session)
	args := []string{}
	templateData := ctx.TemplateData()
	modelArgs := buildModelArgs(agent)
	outputSchema := agent.OutputSchema
	if outputSchema != "" {
		resolved, err := RenderTemplate(outputSchema, templateData)
		if err != nil {
			return "", nil, err
		}
		outputSchema = resolved
	}
	outputFile := agent.OutputFile
	if outputFile != "" {
		resolved, err := RenderTemplate(outputFile, templateData)
		if err != nil {
			return "", nil, err
		}
		outputFile = resolved
	}

	switch agent.Type {
	case "codex":
		if session.Resume == "last" {
			args = append(args, "exec", "resume", "--last")
		} else {
			args = append(args, "exec")
		}
		args = append(args, modelArgs...)
		args = append(args, agent.Args...)
		if outputSchema != "" {
			args = append(args, "--output-schema", outputSchema)
		}
		if outputFile != "" {
			args = append(args, "--output-last-message", outputFile)
		}
		args = append(args, input)
	case "claude":
		args = append(args, "-p", input)
		args = append(args, modelArgs...)
		args = append(args, agent.Args...)
		if session.Resume == "last" {
			sessionID := ctx.Sessions["claude"]
			if sessionID == "" {
				return "", nil, fmt.Errorf("claude resume requested but no session_id is available")
			}
			args = append(args, "--resume", sessionID)
		}
	case "generic":
		if command == "" {
			return "", nil, fmt.Errorf("generic agent requires command")
		}
		args = append(args, agent.Args...)
		if input != "" {
			args = append(args, input)
		}
	default:
		return "", nil, fmt.Errorf("unsupported agent type: %s", agent.Type)
	}

	return command, args, nil
}

func buildModelArgs(agent AgentConfig) []string {
	args := []string{}
	switch agent.Type {
	case "codex":
		if agent.Model != "" {
			args = append(args, "--model", agent.Model)
		}
		if agent.Thinking != "" {
			args = append(args, "-c", fmt.Sprintf("model_reasoning_effort=%s", agent.Thinking))
		}
	case "claude":
		if agent.Model != "" {
			args = append(args, "--model", agent.Model)
		}
	}
	return args
}

func effectiveSession(agentSession *SessionSpec, nodeSession SessionSpec) SessionSpec {
	if nodeSession.Resume != "" {
		return nodeSession
	}
	if agentSession != nil {
		return *agentSession
	}
	return SessionSpec{Resume: "new"}
}

func runCommand(ctx *RunContext, nodeName, agentName, command string, args []string, agent AgentConfig, stepDir string, input string) (*bytes.Buffer, *bytes.Buffer, int, string, error) {
	log.Info("node start", "command", command, "args", strings.Join(args, " "))

	var timeout time.Duration
	if agent.Timeout != "" {
		parsed, err := time.ParseDuration(agent.Timeout)
		if err != nil {
			return nil, nil, 1, "", fmt.Errorf("timeout: %w", err)
		}
		timeout = parsed
	}

	ctxExec := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctxExec, cancel = context.WithTimeout(ctxExec, timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctxExec, command, args...)
	cmd.Dir = ctx.Workdir
	cmd.Env = buildEnv(agent.Env)

	if input != "" && agent.Type == "claude" && !strings.Contains(strings.Join(args, " "), "-p") {
		cmd.Stdin = strings.NewReader(input)
	}

	stdoutPath := filepath.Join(stepDir, "stdout.log")
	stderrPath := filepath.Join(stepDir, "stderr.log")
	stdoutFile, err := os.Create(stdoutPath)
	if err != nil {
		return nil, nil, 1, "", fmt.Errorf("create stdout log: %w", err)
	}
	defer stdoutFile.Close()
	stderrFile, err := os.Create(stderrPath)
	if err != nil {
		return nil, nil, 1, "", fmt.Errorf("create stderr log: %w", err)
	}
	defer stderrFile.Close()

	captureStdout := shouldCapture(agent.Capture, "stdout")
	captureStderr := shouldCapture(agent.Capture, "stderr")
	printStdout := shouldPrint(agent.Print, "stdout")
	printStderr := shouldPrint(agent.Print, "stderr")

	var stdoutBuf bytes.Buffer
	var stderrBuf bytes.Buffer
	stdoutTracker := &outputTracker{}
	stderrTracker := &outputTracker{}

	cmd.Stdout = writerFor(stdoutFile, &stdoutBuf, captureStdout, pickWriter(printStdout, os.Stdout), stdoutTracker)
	cmd.Stderr = writerFor(stderrFile, &stderrBuf, captureStderr, pickWriter(printStderr, os.Stderr), stderrTracker)

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start).String()

	exitCode := 0
	if runErr != nil {
		if errors.Is(runErr, exec.ErrNotFound) {
			return &stdoutBuf, &stderrBuf, 127, duration, fmt.Errorf("command not found: %s", command)
		}
		exitCode = exitCodeFromErr(runErr)
	}

	if ctxExec.Err() == context.DeadlineExceeded {
		exitCode = 124
	}

	if printStdout && stdoutTracker.wrote {
		ensureTrailingNewline(os.Stdout, stdoutTracker)
	}
	if printStderr && stderrTracker.wrote {
		ensureTrailingNewline(os.Stderr, stderrTracker)
	}

	meta := NodeResult{
		Name:     nodeName,
		Agent:    agentName,
		ExitCode: exitCode,
		Duration: duration,
		Command:  strings.Join(append([]string{command}, args...), " "),
	}
	if err := writeMeta(stepDir, meta, stdoutPath, stderrPath); err != nil {
		return &stdoutBuf, &stderrBuf, exitCode, duration, err
	}

	return &stdoutBuf, &stderrBuf, exitCode, duration, nil
}

func handleOutput(ctx *RunContext, item WorkflowItem, stdout []byte) error {
	output := string(stdout)
	if item.Output.ToNext {
		ctx.LastOutput = output
		ctx.Outputs["__previous__"] = output
		if item.Name != "" {
			ctx.Outputs[item.Name] = output
		}
		if parsed := parseJSONOutput(stdout); parsed != nil {
			ctx.Outputs["__previous_json__"] = parsed
			if item.Name != "" {
				ctx.Outputs[item.Name+"_json"] = parsed
			}
		}
	}
	if item.Output.File != "" {
		path, err := RenderTemplate(item.Output.File, ctx.TemplateData())
		if err != nil {
			return err
		}
		if err := os.WriteFile(path, stdout, 0o644); err != nil {
			return fmt.Errorf("write output file: %w", err)
		}
	}
	if item.Output.Stdout {
		if _, err := os.Stdout.Write([]byte("\n")); err != nil {
			return err
		}
		if _, err := os.Stdout.Write(stdout); err != nil {
			return err
		}
		if len(stdout) > 0 && stdout[len(stdout)-1] != '\n' {
			_, _ = os.Stdout.Write([]byte("\n"))
		}
	}
	return nil
}

func updateClaudeSession(ctx *RunContext, stdout []byte) {
	var payload struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(stdout, &payload); err != nil {
		return
	}
	if payload.SessionID != "" {
		ctx.Sessions["claude"] = payload.SessionID
	}
}

func parseJSONOutput(stdout []byte) any {
	var value any
	if err := json.Unmarshal(stdout, &value); err != nil {
		return nil
	}
	return value
}

func outputAsString(value any) (string, error) {
	switch v := value.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		raw, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("marshal output: %w", err)
		}
		return string(raw), nil
	}
}

func summarizeStderr(buf *bytes.Buffer) string {
	if buf == nil || buf.Len() == 0 {
		return ""
	}
	text := strings.TrimSpace(buf.String())
	if text == "" {
		return ""
	}
	const maxLen = 4000
	if len(text) <= maxLen {
		return text
	}
	return "...(truncated)...\n" + text[len(text)-maxLen:]
}

func nodeRunDir(runDir, name string) (string, error) {
	if name == "" {
		name = "node"
	}
	dir := filepath.Join(runDir, "nodes", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create node dir: %w", err)
	}
	return dir, nil
}

func writeMeta(stepDir string, meta NodeResult, stdoutPath, stderrPath string) error {
	raw, err := json.MarshalIndent(struct {
		NodeResult
		StdoutLog string `json:"stdoutLog"`
		StderrLog string `json:"stderrLog"`
	}{
		NodeResult: meta,
		StdoutLog:  stdoutPath,
		StderrLog:  stderrPath,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return os.WriteFile(filepath.Join(stepDir, "meta.json"), raw, 0o644)
}

func writerFor(file *os.File, buf *bytes.Buffer, capture bool, printTo io.Writer, tracker *outputTracker) io.Writer {
	writers := []io.Writer{file}
	if capture {
		writers = append(writers, buf)
	}
	if printTo != nil {
		writers = append(writers, wrapPrintWriter(printTo))
	}
	if tracker != nil {
		writers = append(writers, tracker)
	}
	if len(writers) == 1 {
		return writers[0]
	}
	return io.MultiWriter(writers...)
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

func shouldPrint(print []string, value string) bool {
	if len(print) == 0 {
		return false
	}
	for _, item := range print {
		if item == value {
			return true
		}
	}
	return false
}

func pickWriter(enabled bool, writer io.Writer) io.Writer {
	if enabled {
		return writer
	}
	return nil
}

type printWrapper struct {
	writer  io.Writer
	wrapped bool
}

func wrapPrintWriter(writer io.Writer) *printWrapper {
	return &printWrapper{writer: writer}
}

func (p *printWrapper) Write(data []byte) (int, error) {
	if !p.wrapped {
		if _, err := p.writer.Write([]byte("\n")); err != nil {
			return 0, err
		}
		p.wrapped = true
	}
	return p.writer.Write(data)
}

type outputTracker struct {
	lastByte byte
	wrote    bool
}

func (t *outputTracker) Write(p []byte) (int, error) {
	if len(p) > 0 {
		t.lastByte = p[len(p)-1]
		t.wrote = true
	}
	return len(p), nil
}

func ensureTrailingNewline(writer io.Writer, tracker *outputTracker) {
	if tracker.lastByte != '\n' {
		fmt.Fprintln(writer)
	}
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
