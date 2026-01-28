package moleman

import "testing"

func TestRenderTemplate(t *testing.T) {
	data := map[string]any{
		"input": map[string]any{
			"prompt": "hello",
		},
		"steps": map[string]any{
			"lint": map[string]any{
				"exitCode": 0,
			},
		},
	}

	out, err := RenderTemplate("{{ .input.prompt }} {{ .steps.lint.exitCode }}", data)
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
