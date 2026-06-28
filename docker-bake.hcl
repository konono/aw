// VERSION is the semver without "v" prefix (e.g. "3.4.1") to match
// version.Version in the aw binary. CI strips the prefix from the
// git tag: VERSION=${GITHUB_REF_NAME#v}
variable "VERSION" {
  default = "dev"
}

variable "REGISTRY" {
  default = "ghcr.io/konono"
}

group "default" {
  targets = ["claude", "codex", "opencode", "cursor"]
}

target "_common" {
  platforms = ["linux/amd64", "linux/arm64"]
  args = {
    AW_OCI_SOURCE  = "https://github.com/konono/aw"
    AW_OCI_VERSION = "${VERSION}"
  }
}

target "claude" {
  inherits   = ["_common"]
  context    = "./build/claude-debian12"
  dockerfile = "Dockerfile"
  args = {
    AW_OCI_OS   = "debian12"
    AW_OCI_TOOL = "claude"
  }
  tags = [
    "${REGISTRY}/aw-claude:${VERSION}-debian12",
    "${REGISTRY}/aw-claude:${VERSION}",
    "${REGISTRY}/aw-claude:debian12",
    "${REGISTRY}/aw-claude:latest",
  ]
}

target "codex" {
  inherits   = ["_common"]
  context    = "./build/codex-debian12"
  dockerfile = "Dockerfile"
  args = {
    AW_OCI_OS   = "debian12"
    AW_OCI_TOOL = "codex"
  }
  tags = [
    "${REGISTRY}/aw-codex:${VERSION}-debian12",
    "${REGISTRY}/aw-codex:${VERSION}",
    "${REGISTRY}/aw-codex:debian12",
    "${REGISTRY}/aw-codex:latest",
  ]
}

target "opencode" {
  inherits   = ["_common"]
  context    = "./build/opencode-debian12"
  dockerfile = "Dockerfile"
  args = {
    AW_OCI_OS   = "debian12"
    AW_OCI_TOOL = "opencode"
  }
  tags = [
    "${REGISTRY}/aw-opencode:${VERSION}-debian12",
    "${REGISTRY}/aw-opencode:${VERSION}",
    "${REGISTRY}/aw-opencode:debian12",
    "${REGISTRY}/aw-opencode:latest",
  ]
}

target "cursor" {
  inherits   = ["_common"]
  context    = "./build/cursor-debian12"
  dockerfile = "Dockerfile"
  args = {
    AW_OCI_OS   = "debian12"
    AW_OCI_TOOL = "cursor"
  }
  tags = [
    "${REGISTRY}/aw-cursor:${VERSION}-debian12",
    "${REGISTRY}/aw-cursor:${VERSION}",
    "${REGISTRY}/aw-cursor:debian12",
    "${REGISTRY}/aw-cursor:latest",
  ]
}
