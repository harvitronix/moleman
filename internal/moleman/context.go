package moleman

type RunContext struct {
	Input       string
	Outputs     map[string]any
	LastOutput  string
	Sessions    map[string]string
	RunDir      string
	Workdir     string
	Verbose     bool
	NodeResults []NodeResult
}

type NodeResult struct {
	Name     string
	Agent    string
	ExitCode int
	Duration string
	Command  string
}

func (ctx *RunContext) TemplateData() map[string]any {
	return map[string]any{
		"input": map[string]any{
			"prompt": ctx.Input,
		},
		"outputs":  ctx.Outputs,
		"last":     ctx.LastOutput,
		"sessions": ctx.Sessions,
	}
}
