package moleman

type InputData struct {
	Prompt string
	Issue  string
}

type GitData struct {
	Diff   string
	Status string
	Branch string
	Root   string
}

type StepResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration string
	Command  string
}

type RunContext struct {
	Input        InputData
	Git          GitData
	Steps        map[string]StepResult
	StepsHistory map[string][]StepResult
	Vars         map[string]any
	RunDir       string
	Workdir      string
	Verbose      bool
}

func (ctx *RunContext) TemplateData() map[string]any {
	steps := map[string]map[string]any{}
	stepsHistory := map[string][]map[string]any{}

	for id, result := range ctx.Steps {
		steps[id] = map[string]any{
			"stdout":   result.Stdout,
			"stderr":   result.Stderr,
			"exitCode": result.ExitCode,
		}
	}
	for id, results := range ctx.StepsHistory {
		history := make([]map[string]any, 0, len(results))
		for _, result := range results {
			history = append(history, map[string]any{
				"stdout":   result.Stdout,
				"stderr":   result.Stderr,
				"exitCode": result.ExitCode,
			})
		}
		stepsHistory[id] = history
	}

	return map[string]any{
		"input": map[string]any{
			"prompt": ctx.Input.Prompt,
			"issue":  ctx.Input.Issue,
		},
		"git": map[string]any{
			"diff":   ctx.Git.Diff,
			"status": ctx.Git.Status,
			"branch": ctx.Git.Branch,
			"root":   ctx.Git.Root,
		},
		"steps":        steps,
		"stepsHistory": stepsHistory,
		"vars":         ctx.Vars,
	}
}
