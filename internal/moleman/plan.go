package moleman

import (
	"encoding/json"
	"fmt"
	"os"
)

func ResolvePlan(cfg *Config, pipeline string) ([]Node, error) {
	p, ok := cfg.Pipelines[pipeline]
	if !ok {
		return nil, fmt.Errorf("pipeline not found: %s", pipeline)
	}

	resolved := []Node{}
	stack := map[string]bool{}
	for _, node := range p.Plan {
		out, err := resolveNode(cfg, node, stack)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, out...)
	}
	return resolved, nil
}

func resolveNode(cfg *Config, node Node, stack map[string]bool) ([]Node, error) {
	switch node.Type {
	case "ref":
		if node.ID == "" {
			return nil, fmt.Errorf("ref node missing id")
		}
		if stack[node.ID] {
			return nil, fmt.Errorf("cycle detected at ref: %s", node.ID)
		}
		stack[node.ID] = true
		defer delete(stack, node.ID)

		if step, ok := cfg.Steps[node.ID]; ok {
			if step.Type != "run" {
				return nil, fmt.Errorf("step %s must be type 'run'", node.ID)
			}
			return []Node{nodeFromStep(node.ID, step)}, nil
		}
		if group, ok := cfg.Groups[node.ID]; ok {
			expanded := []Node{}
			for _, child := range group {
				out, err := resolveNode(cfg, child, stack)
				if err != nil {
					return nil, err
				}
				expanded = append(expanded, out...)
			}
			return expanded, nil
		}
		return nil, fmt.Errorf("ref not found: %s", node.ID)
	case "group":
		expanded := []Node{}
		for _, child := range node.Body {
			out, err := resolveNode(cfg, child, stack)
			if err != nil {
				return nil, err
			}
			expanded = append(expanded, out...)
		}
		return expanded, nil
	case "loop":
		out := node
		if out.MaxIters <= 0 {
			return nil, fmt.Errorf("loop maxIters must be > 0")
		}
		if len(out.Body) == 0 {
			return nil, fmt.Errorf("loop body is empty")
		}
		return []Node{out}, nil
	case "run":
		if node.Run == "" {
			return nil, fmt.Errorf("run node missing command")
		}
		return []Node{node}, nil
	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

func nodeFromStep(id string, step Step) Node {
	return Node{
		Type:      "run",
		ID:        id,
		Run:       step.Run,
		Stdin:     step.Stdin,
		StdinFile: step.StdinFile,
		Env:       step.Env,
		Timeout:   step.Timeout,
		Capture:   step.Capture,
		Print:     step.Print,
		Parse:     step.Parse,
		Shell:     step.Shell,
	}
}

func PrintPlan(plan []Node) error {
	raw, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal plan: %w", err)
	}
	_, err = os.Stdout.Write(raw)
	if err != nil {
		return fmt.Errorf("write plan: %w", err)
	}
	_, _ = os.Stdout.Write([]byte("\n"))
	return nil
}
