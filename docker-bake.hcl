// VERSION is the semver without "v" prefix (e.g. "3.4.1") to match
// version.Version in the aw binary. CI strips the prefix from the
// git tag: VERSION=${GITHUB_REF_NAME#v}
variable "VERSION" {
  default = "dev"
}

variable "REGISTRY" {
  default = "ghcr.io/konono"
}

variable "DEFAULT_OS" {
  default = "debian12"
}

group "default" {
  targets = ["image"]
}

target "image" {
  name       = "aw-${tool}-${os}"
  matrix = {
    tool = ["base", "claude", "codex", "opencode", "cursor"]
    os   = ["debian12", "ubuntu2604", "ubi9", "ubi10"]
  }
  platforms  = ["linux/amd64", "linux/arm64"]
  context    = "./build/${tool}-${os}"
  dockerfile = "Dockerfile"
  args = {
    AW_OCI_SOURCE  = "https://github.com/konono/aw"
    AW_OCI_VERSION = "${VERSION}"
    AW_OCI_OS      = "${os}"
    AW_OCI_TOOL    = "${tool}"
  }
  tags = concat(
    [
      "${REGISTRY}/aw-${tool}:${VERSION}-${os}",
      "${REGISTRY}/aw-${tool}:${os}",
    ],
    equal(os, DEFAULT_OS) ? [
      "${REGISTRY}/aw-${tool}:${VERSION}",
      "${REGISTRY}/aw-${tool}:latest",
    ] : [],
  )
}
