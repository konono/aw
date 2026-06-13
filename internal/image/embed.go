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

//go:embed embed/Dockerfile.debian12.devbox.tmpl
var dockerfileDebian12DevboxTmpl string

//go:embed embed/Dockerfile.ubi9.devbox.tmpl
var dockerfileUBI9DevboxTmpl string

//go:embed embed/Dockerfile.ubi10.devbox.tmpl
var dockerfileUBI10DevboxTmpl string

//go:embed embed/Dockerfile.ubuntu2604.devbox.tmpl
var dockerfileUbuntu2604DevboxTmpl string

//go:embed embed/entrypoint.sh.devbox.tmpl
var entrypointShDevboxTmpl string

var dockerfileTmpls = map[profile.OSTemplate]string{
	profile.OSDebian12:   dockerfileDebian12Tmpl,
	profile.OSUBI9:       dockerfileUBI9Tmpl,
	profile.OSUBI10:      dockerfileUBI10Tmpl,
	profile.OSUbuntu2604: dockerfileUbuntu2604Tmpl,
}

var dockerfileDevboxTmpls = map[profile.OSTemplate]string{
	profile.OSDebian12:   dockerfileDebian12DevboxTmpl,
	profile.OSUBI9:       dockerfileUBI9DevboxTmpl,
	profile.OSUBI10:      dockerfileUBI10DevboxTmpl,
	profile.OSUbuntu2604: dockerfileUbuntu2604DevboxTmpl,
}

func RenderDockerfile(os profile.OSTemplate, pkgMgr profile.PackageManager, cenv containerenv.Config) ([]byte, error) {
	tmpls := dockerfileTmpls
	if pkgMgr == profile.PackageManagerDevbox {
		tmpls = dockerfileDevboxTmpls
	}
	tmplStr, ok := tmpls[os]
	if !ok {
		return nil, fmt.Errorf("unknown OS template: %q", os)
	}
	return renderTemplate("Dockerfile", tmplStr, cenv)
}

func RenderEntrypoint(pkgMgr profile.PackageManager, cenv containerenv.Config) ([]byte, error) {
	tmpl := entrypointShTmpl
	if pkgMgr == profile.PackageManagerDevbox {
		tmpl = entrypointShDevboxTmpl
	}
	return renderTemplate("entrypoint.sh", tmpl, cenv)
}

func DefaultDockerfile() []byte {
	b, err := RenderDockerfile(profile.OSDebian12, profile.PackageManagerApt, containerenv.Default())
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
