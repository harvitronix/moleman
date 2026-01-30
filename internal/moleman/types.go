package moleman

type Config struct {
	Version  int                    `yaml:"version"`
	Agents   map[string]AgentConfig `yaml:"agents"`
	Workflow []WorkflowItem         `yaml:"workflow"`
}

type AgentConfig struct {
	Extends string            `yaml:"extends,omitempty"`
	Type    string            `yaml:"type"`
	Command string            `yaml:"command,omitempty"`
	Model   string            `yaml:"model,omitempty"`
	Thinking string           `yaml:"thinking,omitempty"`
	Args    []string          `yaml:"args,omitempty"`
	OutputSchema string       `yaml:"outputSchema,omitempty"`
	OutputFile   string       `yaml:"outputFile,omitempty"`
	Env     map[string]string `yaml:"env,omitempty"`
	Timeout string            `yaml:"timeout,omitempty"`
	Capture []string          `yaml:"capture,omitempty"`
	Print   []string          `yaml:"print,omitempty"`
	Session *SessionSpec      `yaml:"session,omitempty"`
}

type WorkflowItem struct {
	Type     string         `yaml:"type"`
	Name     string         `yaml:"name,omitempty"`
	Agent    string         `yaml:"agent,omitempty"`
	Input    InputSpec      `yaml:"input,omitempty"`
	Output   OutputSpec     `yaml:"output,omitempty"`
	Session  SessionSpec    `yaml:"session,omitempty"`
	MaxIters int            `yaml:"maxIters,omitempty"`
	Until    string         `yaml:"until,omitempty"`
	Body     []WorkflowItem `yaml:"body,omitempty"`
}

type InputSpec struct {
	Prompt string `yaml:"prompt,omitempty"`
	File   string `yaml:"file,omitempty"`
	From   string `yaml:"from,omitempty"`
}

type OutputSpec struct {
	ToNext bool   `yaml:"toNext,omitempty"`
	File   string `yaml:"file,omitempty"`
	Stdout bool   `yaml:"stdout,omitempty"`
}

type SessionSpec struct {
	Resume string `yaml:"resume,omitempty"`
}
