package moleman

import (
	"bytes"
	"fmt"
	"text/template"
)

func RenderTemplate(input string, data map[string]any) (string, error) {
	if input == "" {
		return "", nil
	}
	tpl, err := template.New("moleman").
		Option("missingkey=zero").
		Parse(input)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
