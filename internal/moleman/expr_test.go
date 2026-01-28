package moleman

import "testing"

func TestEvalConditionBasic(t *testing.T) {
	data := map[string]any{
		"steps": map[string]any{
			"lint": map[string]any{
				"exitCode": 0,
			},
		},
		"stepsHistory": map[string]any{
			"lint": []map[string]any{
				{"exitCode": 1},
			},
		},
		"git": map[string]any{
			"branch": "main",
		},
	}

	cases := []struct {
		expr string
		want bool
	}{
		{"steps.lint.exitCode == 0", true},
		{"stepsHistory.lint[0].exitCode != 0", true},
		{"git.branch == \"main\"", true},
		{"git.branch == \"dev\"", false},
		{"true && false", false},
		{"true || false", true},
		{"{{ steps.lint.exitCode == 1 }}", false},
	}

	for _, tc := range cases {
		got, err := EvalCondition(tc.expr, data)
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", tc.expr, err)
		}
		if got != tc.want {
			t.Fatalf("expr %q = %v, want %v", tc.expr, got, tc.want)
		}
	}
}

func TestEvalConditionErrors(t *testing.T) {
	data := map[string]any{
		"steps": map[string]any{
			"lint": map[string]any{
				"exitCode": 0,
			},
		},
	}

	cases := []string{
		"",
		"steps.missing.exitCode == 0",
		"steps.lint.exitCode == \"zero\"",
	}

	for _, expr := range cases {
		if _, err := EvalCondition(expr, data); err == nil {
			t.Fatalf("expected error for %q", expr)
		}
	}
}
