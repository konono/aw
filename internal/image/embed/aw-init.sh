#!/bin/bash
# aw-init.sh — shared container initialization for aw.
# Sourced by entrypoint scripts (both built-in and custom).
# Uses AW_USER / AW_HOME env vars set by the aw CLI at container launch.
set -e

aw_log() { echo "[aw:entrypoint] $1" >&2; }

run_as_user() {
  /bin/bash -lc "$1"
}

# aw_fix_mise_shims — fix non-standard binary names installed by mise.
# Call after "mise install" with MISE_CMD as the first argument.
# Fixes: podman shim naming, docker-compose shim naming, podman compose plugin.
aw_fix_mise_shims() {
  local mise_cmd="$1"

  # podman: podman-remote-static-linux_<arch> -> podman
  # Uses subshell (cd ...) so the glob expands relative to $PODMAN_DIR/bin/
  run_as_user "$mise_cmd && \
    PODMAN_DIR=\$(mise where podman 2>/dev/null) && \
    mkdir -p \"\$PODMAN_DIR/bin\" && \
    (cd \"\$PODMAN_DIR/bin\" && ln -sf ../podman-remote-static-linux_* podman)" || true

  # docker-compose: docker-cli-plugin-docker-compose -> docker-compose
  run_as_user "$mise_cmd && \
    DC_DIR=\$(mise where docker-compose 2>/dev/null) && \
    mkdir -p \"\$DC_DIR/bin\" && \
    (cd \"\$DC_DIR/bin\" && ln -sf ../docker-cli-plugin-docker-compose docker-compose)" || true

  # podman compose plugin: podman searches ~/.docker/cli-plugins/ not PATH
  if [ -S /run/container.sock ]; then
    run_as_user "$mise_cmd && \
      DC_DIR=\$(mise where docker-compose 2>/dev/null) && \
      mkdir -p \"$AW_HOME/.docker/cli-plugins\" && \
      ln -sf \"\$DC_DIR/docker-cli-plugin-docker-compose\" \"$AW_HOME/.docker/cli-plugins/docker-compose\"" || true
  fi

  run_as_user "$mise_cmd && mise reshim" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Resolve core variables
# ---------------------------------------------------------------------------
AW_USER="${AW_USER:-agent}"
AW_HOME="${AW_HOME:-/home/$AW_USER}"
AW_WORKSPACE="${HOST_WORKSPACE:-${AW_WORKSPACE:-/workspace}}"
export AW_USER AW_HOME AW_WORKSPACE

AW_ENV_FILE="$AW_HOME/.aw_env.sh"
BASHRC_FILE="$AW_HOME/.bashrc"
BASH_PROFILE_FILE="$AW_HOME/.bash_profile"

# ---------------------------------------------------------------------------
# Dynamic /etc/passwd injection (OpenShift GID 0 pattern)
# ---------------------------------------------------------------------------
# The image is built with a fixed UID (1001) but runs with the host user's
# UID:GID via --user UID:GID --group-add 0.  Container runtimes (e.g.
# Podman --userns=keep-id) may inject a passwd entry with the host's home
# path, which breaks tools that use getpwuid() instead of $HOME (ssh, git).
_aw_uid=$(id -u)
_aw_gid=$(id -g)
if [ "$_aw_uid" != "0" ]; then
  _aw_tmp=$(grep -v -e "^[^:]*:[^:]*:${_aw_uid}:" -e "^${AW_USER}:" /etc/passwd) && printf '%s\n' "$_aw_tmp" > /etc/passwd || true
  echo "${AW_USER}:x:${_aw_uid}:${_aw_gid}:${AW_USER}:${AW_HOME}:/bin/bash" >> /etc/passwd
fi
unset _aw_uid _aw_gid _aw_tmp
export HOME="$AW_HOME"

# ---------------------------------------------------------------------------
# Fix ownership when runtime UID differs from build-time UID (1001)
# ---------------------------------------------------------------------------
if [ "$(stat -c '%u' "$AW_HOME")" != "$(id -u)" ]; then
  aw_log "Fixing file ownership for UID $(id -u)..."
  sudo chown -R "$(id -u):$(id -g)" "$AW_HOME" 2>/dev/null || true
fi

# ---------------------------------------------------------------------------
# Extra package installation
# ---------------------------------------------------------------------------
if [ -n "${AW_PACKAGES:-}" ]; then
  IFS=',' read -ra _aw_pkgs <<< "$AW_PACKAGES"
  _aw_install=()
  if command -v apt-get > /dev/null 2>&1; then
    for _p in "${_aw_pkgs[@]}"; do
      _p=$(echo "$_p" | xargs)
      [ -z "$_p" ] && continue
      dpkg -s "$_p" > /dev/null 2>&1 || _aw_install+=("$_p")
    done
    if [ ${#_aw_install[@]} -gt 0 ]; then
      aw_log "Installing packages: ${_aw_install[*]}"
      sudo apt-get update -qq && sudo apt-get install -y --no-install-recommends "${_aw_install[@]}"
      sudo rm -rf /var/lib/apt/lists/*
    fi
  elif command -v dnf > /dev/null 2>&1; then
    for _p in "${_aw_pkgs[@]}"; do
      _p=$(echo "$_p" | xargs)
      [ -z "$_p" ] && continue
      rpm -q "$_p" > /dev/null 2>&1 || _aw_install+=("$_p")
    done
    if [ ${#_aw_install[@]} -gt 0 ]; then
      aw_log "Installing packages: ${_aw_install[*]}"
      sudo dnf install -y "${_aw_install[@]}"
      sudo dnf clean all
    fi
  fi
fi

# ---------------------------------------------------------------------------
# Tool config symlinks
# ---------------------------------------------------------------------------
if [ -n "$AW_CONTAINER_CONFIG_DIR" ] && [ -f "$AW_CONTAINER_CONFIG_DIR/.claude.json" ]; then
  ln -sfn "$AW_CONTAINER_CONFIG_DIR/.claude.json" "$AW_HOME/.claude.json"
fi

if [ -n "$AW_DATA_SYMLINKS" ]; then
  IFS=',' read -ra LINKS <<< "$AW_DATA_SYMLINKS"
  for link in "${LINKS[@]}"; do
    LINK_PATH="${link%%:*}"
    TARGET_PATH="${link##*:}"
    mkdir -p "$TARGET_PATH"
    mkdir -p "$(dirname "$LINK_PATH")"
    ln -sfn "$TARGET_PATH" "$LINK_PATH"
  done
fi

# ---------------------------------------------------------------------------
# SSH
# ---------------------------------------------------------------------------
aw_log "Setting up SSH..."
if [ -d "$AW_HOME/.ssh-host" ]; then
  echo "Configuring SSH keys..."
  cp -a "$AW_HOME/.ssh-host" "$AW_HOME/.ssh" 2>/dev/null || true
  chmod 700 "$AW_HOME/.ssh"
  chmod 600 "$AW_HOME/.ssh/"* 2>/dev/null || true
  chmod 644 "$AW_HOME/.ssh/"*.pub 2>/dev/null || true
  chmod 644 "$AW_HOME/.ssh/known_hosts" 2>/dev/null || true
  chmod 644 "$AW_HOME/.ssh/config" 2>/dev/null || true
fi

if [ -n "$SSH_AUTH_SOCK" ] && [ ! -d "$AW_HOME/.ssh" ]; then
  mkdir -p "$AW_HOME/.ssh"
  chmod 700 "$AW_HOME/.ssh"
  cat > "$AW_HOME/.ssh/config" <<SSHCFG
Host *
  StrictHostKeyChecking accept-new
  UserKnownHostsFile $AW_HOME/.ssh/known_hosts
SSHCFG
  chmod 644 "$AW_HOME/.ssh/config"
fi

# ---------------------------------------------------------------------------
# Git credential helper
# ---------------------------------------------------------------------------
aw_log "Setting up git credentials..."
if [ -n "$GITHUB_TOKEN" ]; then
  cat > "$AW_HOME/.git-credential-token" <<'CRED'
#!/bin/sh
echo username=x-access-token
echo "password=${GITHUB_TOKEN}"
CRED
  chmod +x "$AW_HOME/.git-credential-token"
fi

# ---------------------------------------------------------------------------
# Container runtime socket
# ---------------------------------------------------------------------------
aw_log "Setting up container socket..."
if [ -S /run/container.sock ]; then
  sudo chmod 666 /run/container.sock 2>/dev/null || true
fi

# ---------------------------------------------------------------------------
# Generate shell environment files
# ---------------------------------------------------------------------------
aw_log "Generating shell environment..."
export HOME="$AW_HOME"

echo "Initializing shell environment..."

cat > "$AW_ENV_FILE" <<ENVEOF
if [ -n "\${AW_BASH_ENV_LOADED:-}" ]; then
  return 0
fi
AW_BASH_ENV_LOADED=1
export HOME=$AW_HOME

case ":\${PATH}:" in
  *:$AW_HOME/.local/bin:*) ;;
  *) export PATH="$AW_HOME/.local/bin:\${PATH}" ;;
esac
case ":\${PATH}:" in
  *:$AW_HOME/.local/share/mise/shims:*) ;;
  *) export PATH="$AW_HOME/.local/share/mise/shims:\${PATH}" ;;
esac
export MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE:-$AW_WORKSPACE}"
export MISE_YES=1
if [ -S /run/container.sock ]; then
  [ -z "\${DOCKER_HOST:-}" ] && export DOCKER_HOST="unix:///run/container.sock"
  [ -z "\${CONTAINER_HOST:-}" ] && export CONTAINER_HOST="unix:///run/container.sock"
fi
if [ -n "\${GITHUB_TOKEN:-}" ] && [ -x "$AW_HOME/.git-credential-token" ]; then
  export GIT_CONFIG_COUNT=1
  export GIT_CONFIG_KEY_0="credential.https://github.com.helper"
  export GIT_CONFIG_VALUE_0="!$AW_HOME/.git-credential-token"
fi
ENVEOF

cat > "$BASHRC_FILE" <<BASHRC
if [ -f $AW_HOME/.aw_env.sh ]; then
  . $AW_HOME/.aw_env.sh
fi
BASHRC

cat > "$BASH_PROFILE_FILE" <<BASH_PROFILE
if [ -f $AW_HOME/.bashrc ]; then
  . $AW_HOME/.bashrc
fi
BASH_PROFILE

# ---------------------------------------------------------------------------
# aw_exec — final exec wrapper for entrypoint scripts
# ---------------------------------------------------------------------------
aw_exec() {
  aw_log "Launching: $*"
  if [ "${AW_SESSION_LOG}" = "1" ]; then
    if ! command -v pty-logger >/dev/null 2>&1; then
      aw_log "ERROR: AW_SESSION_LOG=1 but pty-logger is not installed."
      aw_log "Build the image with 'aw build --from-template' and session_log: true."
      exit 1
    fi
    aw_log "Session logging enabled — wrapping with script + pty-logger"
    local typescript=/tmp/aw-typescript
    local pty_cols=${PTY_LOGGER_COLS:-120}
    local pty_rows=${PTY_LOGGER_ROWS:-40}
    : > "$typescript"
    PTY_LOGGER_COLS=$pty_cols PTY_LOGGER_ROWS=$pty_rows pty-logger "$typescript" &
    sleep 0.3
    exec env HOME="$AW_HOME" BASH_ENV="$AW_ENV_FILE" \
      script -qf "$typescript" -c "stty cols $pty_cols rows $pty_rows 2>/dev/null; bash -lc \"$*\"" >/dev/null 2>&1
  fi
  exec env HOME="$AW_HOME" BASH_ENV="$AW_ENV_FILE" \
    bash -lc "$*"
}
