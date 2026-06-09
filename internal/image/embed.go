package image

import (
	"bytes"
	_ "embed"
	"fmt"
	"text/template"

	"github.com/konono/aw/internal/containerenv"
	"github.com/konono/aw/internal/profile"
)

//go:embed embed/Dockerfile.debian12.tmpl
var dockerfileDebian12Tmpl string

//go:embed embed/Dockerfile.ubi9.tmpl
var dockerfileUBI9Tmpl string

//go:embed embed/Dockerfile.ubi10.tmpl
var dockerfileUBI10Tmpl string

//go:embed embed/Dockerfile.ubuntu2604.tmpl
var dockerfileUbuntu2604Tmpl string

//go:embed embed/entrypoint.sh.tmpl
var entrypointShTmpl string

var dockerfileTmpls = map[profile.OSTemplate]string{
	profile.OSDebian12:   dockerfileDebian12Tmpl,
	profile.OSUBI9:       dockerfileUBI9Tmpl,
	profile.OSUBI10:      dockerfileUBI10Tmpl,
	profile.OSUbuntu2604: dockerfileUbuntu2604Tmpl,
}

func RenderDockerfile(os profile.OSTemplate, cenv containerenv.Config) ([]byte, error) {
	tmplStr, ok := dockerfileTmpls[os]
	if !ok {
		return nil, fmt.Errorf("unknown OS template: %q", os)
	}
	return renderTemplate("Dockerfile", tmplStr, cenv)
}

func RenderEntrypoint(cenv containerenv.Config) ([]byte, error) {
	return renderTemplate("entrypoint.sh", entrypointShTmpl, cenv)
}

func DefaultDockerfile() []byte {
	b, err := RenderDockerfile(profile.OSDebian12, containerenv.Default())
	if err != nil {
		panic(fmt.Sprintf("rendering default Dockerfile: %v", err))
	}
	return b
}

func renderTemplate(name, tmplStr string, data interface{}) ([]byte, error) {
	t, err := template.New(name).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing %s template: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("rendering %s template: %w", name, err)
	}
	return buf.Bytes(), nil
}
