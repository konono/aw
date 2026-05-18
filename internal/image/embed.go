package image

import (
	_ "embed"
	"fmt"

	"github.com/konono/aw/internal/profile"
)

//go:embed embed/Dockerfile.debian12
var dockerfileDebian12 []byte

//go:embed embed/Dockerfile.ubi9
var dockerfileUBI9 []byte

//go:embed embed/Dockerfile.ubi10
var dockerfileUBI10 []byte

//go:embed embed/Dockerfile.ubuntu2604
var dockerfileUbuntu2604 []byte

//go:embed embed/entrypoint.sh
var entrypointSh []byte

var dockerfiles = map[profile.OSTemplate][]byte{
	profile.OSDebian12:   dockerfileDebian12,
	profile.OSUBI9:       dockerfileUBI9,
	profile.OSUBI10:      dockerfileUBI10,
	profile.OSUbuntu2604: dockerfileUbuntu2604,
}

// DockerfileForOS returns the embedded Dockerfile for the given OS template.
func DockerfileForOS(os profile.OSTemplate) ([]byte, error) {
	df, ok := dockerfiles[os]
	if !ok {
		return nil, fmt.Errorf("unknown OS template: %q", os)
	}
	return df, nil
}

// DefaultDockerfile returns the content of the embedded default (Debian) Dockerfile.
func DefaultDockerfile() []byte {
	return dockerfileDebian12
}
