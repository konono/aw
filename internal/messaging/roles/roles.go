package roles

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *.md.tmpl
var templates embed.FS

// MemberData describes a team member for template rendering.
type MemberData struct {
	AgentName string
	Role      string
	IsSelf    bool
}

// TemplateData is passed to role templates.
type TemplateData struct {
	TeamName  string
	AgentName string
	Members   []MemberData
}

// Render renders the role template for the given role.
func Render(role string, data TemplateData) (string, error) {
	filename := role + ".md.tmpl"
	content, err := templates.ReadFile(filename)
	if err != nil {
		return "", fmt.Errorf("role template %q not found: %w", role, err)
	}

	tmpl, err := template.New(filename).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("parsing role template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("rendering role template: %w", err)
	}

	return buf.String(), nil
}
