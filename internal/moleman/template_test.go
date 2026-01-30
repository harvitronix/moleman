package moleman

import "testing"

func TestRenderTemplate(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"prompt": "hello",
		},
		"outputs": map[string]any{
			"review_json": map[string]any{
				"structured_output": map[string]any{
					"must_fix_count": 0,
				},
			},
		},
	}

	out, err := RenderTemplate("{{ .input.prompt }} {{ .outputs.review_json.structured_output.must_fix_count }}", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "hello 0" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderTemplateShellEscape(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"prompt": "we're good",
		},
	}

	out, err := RenderTemplate("{{ shellEscape .input.prompt }}", data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "'we'\"'\"'re good'" {
		t.Fatalf("unexpected output: %q", out)
	}
}

func TestRenderTemplateEmptyInput(t *testing.T) {
	out, err := RenderTemplate("", map[string]any{"ignored": true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Fatalf("unexpected output: %q", out)
	}
}
