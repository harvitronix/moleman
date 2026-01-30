package moleman

import "testing"

func TestEvalConditionBasic(t *testing.T) {
	data := map[string]any{
		"outputs": map[string]any{
			"review_json": map[string]any{
				"structured_output": map[string]any{
					"must_fix_count": 0,
				},
			},
			"previous": "ok",
		},
		"last": "ok",
	}

	cases := []struct {
		expr string
		want bool
	}{
		{"outputs.review_json.structured_output.must_fix_count == 0", true},
		{"outputs.review_json.structured_output.must_fix_count != 0", false},
		{"last == \"ok\"", true},
		{"true && false", false},
		{"true || false", true},
		{"{{ outputs.review_json.structured_output.must_fix_count == 1 }}", false},
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
		"outputs": map[string]any{
			"bad": "zero",
			"obj": map[string]any{
				"value": "nope",
			},
		},
	}

	cases := []string{
		"",
		"outputs.bad == 1",
		"outputs.obj == \"zero\"",
	}

	for _, expr := range cases {
		if _, err := EvalCondition(expr, data); err == nil {
			t.Fatalf("expected error for %q", expr)
		}
	}
}
