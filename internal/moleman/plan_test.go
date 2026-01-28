package moleman

import "testing"

func TestResolvePlanStepAndGroup(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Steps: map[string]Step{
			"lint": {Type: "run", Run: "pnpm lint"},
		},
		Groups: map[string][]Node{
			"core": {
				{Type: "ref", ID: "lint"},
			},
		},
		Pipelines: map[string]Pipeline{
			"default": {
				Plan: []Node{{Type: "ref", ID: "core"}},
			},
		},
	}

	plan, err := ResolvePlan(cfg, "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("expected 1 node, got %d", len(plan))
	}
	if plan[0].Type != "run" || plan[0].Run == "" {
		t.Fatalf("expected run node, got %+v", plan[0])
	}
}

func TestResolvePlanDetectsCycle(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Groups: map[string][]Node{
			"a": {{Type: "ref", ID: "b"}},
			"b": {{Type: "ref", ID: "a"}},
		},
		Pipelines: map[string]Pipeline{
			"default": {
				Plan: []Node{{Type: "ref", ID: "a"}},
			},
		},
	}

	if _, err := ResolvePlan(cfg, "default"); err == nil {
		t.Fatalf("expected cycle error")
	}
}

func TestResolvePlanRejectsBadStepType(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Steps: map[string]Step{
			"lint": {Type: "shell", Run: "pnpm lint"},
		},
		Pipelines: map[string]Pipeline{
			"default": {
				Plan: []Node{{Type: "ref", ID: "lint"}},
			},
		},
	}

	if _, err := ResolvePlan(cfg, "default"); err == nil {
		t.Fatalf("expected step type error")
	}
}

func TestResolvePlanValidatesLoop(t *testing.T) {
	cfg := &Config{
		Version: 1,
		Pipelines: map[string]Pipeline{
			"default": {
				Plan: []Node{{
					Type: "loop",
				}},
			},
		},
	}

	if _, err := ResolvePlan(cfg, "default"); err == nil {
		t.Fatalf("expected loop validation error")
	}
}
