package moleman

type Config struct {
	Version   int                 `yaml:"version"`
	Steps     map[string]Step     `yaml:"steps"`
	Groups    map[string][]Node   `yaml:"groups"`
	Pipelines map[string]Pipeline `yaml:"pipelines"`
}

type Step struct {
	Type      string            `yaml:"type"`
	Run       string            `yaml:"run"`
	Stdin     string            `yaml:"stdin,omitempty"`
	StdinFile string            `yaml:"stdinFile,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Timeout   string            `yaml:"timeout,omitempty"`
	Capture   []string          `yaml:"capture,omitempty"`
	Print     []string          `yaml:"print,omitempty"`
	Parse     *ParseConfig      `yaml:"parse,omitempty"`
	Shell     string            `yaml:"shell,omitempty"`
}

type Node struct {
	Type      string            `yaml:"type"`
	ID        string            `yaml:"id,omitempty"`
	Run       string            `yaml:"run,omitempty"`
	Stdin     string            `yaml:"stdin,omitempty"`
	StdinFile string            `yaml:"stdinFile,omitempty"`
	Env       map[string]string `yaml:"env,omitempty"`
	Timeout   string            `yaml:"timeout,omitempty"`
	Capture   []string          `yaml:"capture,omitempty"`
	Print     []string          `yaml:"print,omitempty"`
	Parse     *ParseConfig      `yaml:"parse,omitempty"`
	Shell     string            `yaml:"shell,omitempty"`

	MaxIters int    `yaml:"maxIters,omitempty"`
	Until    string `yaml:"until,omitempty"`
	Body     []Node `yaml:"body,omitempty"`
}

type Pipeline struct {
	Plan []Node `yaml:"plan"`
}

type ParseConfig struct {
	Kind string `yaml:"kind"`
	Into string `yaml:"into"`
}
