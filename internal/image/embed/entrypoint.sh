#!/bin/bash
set -e

run_as_agent() {
  local script="$1"

  if [ "$(id -un)" = "agent" ]; then
    /bin/bash -lc "$script"
  else
    su -s /bin/bash agent -c "$script"
  fi
}

# Symlink .claude.json from staging dir to home (for onboarding state persistence)
if [ -n "$AW_CONTAINER_CONFIG_DIR" ] && [ -f "$AW_CONTAINER_CONFIG_DIR/.claude.json" ]; then
  ln -sfn "$AW_CONTAINER_CONFIG_DIR/.claude.json" /home/agent/.claude.json
fi

# Create data symlinks for tools that store data separately from config
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

WORKSPACE="${HOST_WORKSPACE:-/workspace}"
AW_ENV_FILE=/home/agent/.aw_env.sh
BASHRC_FILE=/home/agent/.bashrc
BASH_PROFILE_FILE=/home/agent/.bash_profile

# AI tool is already installed at build time via Dockerfile.
# Install project packages (supports both devbox.json and mise.toml)
NIX_ENV="export HOME=/home/agent && . /home/agent/.nix-profile/etc/profile.d/nix.sh 2>/dev/null;"
if [ -f "$WORKSPACE/devbox.json" ]; then
  echo "Installing packages from devbox.json..."
  run_as_agent "$NIX_ENV cd \"$WORKSPACE\" && devbox install"
  if run_as_agent "$NIX_ENV cd \"$WORKSPACE\" && devbox run --list 2>/dev/null" | grep -q '^install'; then
    echo "Running devbox run install..."
    run_as_agent "$NIX_ENV cd \"$WORKSPACE\" && devbox run install"
  fi
elif [ -f "$WORKSPACE/mise.toml" ] || [ -f "$WORKSPACE/.mise.toml" ]; then
  if ! run_as_agent 'command -v mise' > /dev/null 2>&1; then
    echo "Installing mise..."
    run_as_agent 'curl https://mise.jdx.dev/install.sh | sh'
  fi
  MISE_CMD="export HOME=/home/agent && export MISE_DATA_DIR=/home/agent/.local/share/mise && export MISE_CONFIG_DIR=/home/agent/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$WORKSPACE && export MISE_YES=1"
  mkdir -p /home/agent/.config/mise
  echo "Installing tools from mise.toml..."
  run_as_agent "$MISE_CMD && cd \"$WORKSPACE\" && mise trust --all && mise install"
  if run_as_agent "$MISE_CMD && cd \"$WORKSPACE\" && mise tasks 2>/dev/null | grep -q '^install '"; then
    echo "Running mise run install..."
    run_as_agent "$MISE_CMD && cd \"$WORKSPACE\" && mise run install"
  fi
else
  echo "No devbox.json or mise.toml found. devbox is available for installing tools."
fi

# Copy and fix permissions on mounted .ssh (read-only mount, so copy first)
if [ -d /home/agent/.ssh-host ]; then
  cp -a /home/agent/.ssh-host /home/agent/.ssh 2>/dev/null || true
  chmod 700 /home/agent/.ssh
  chmod 600 /home/agent/.ssh/* 2>/dev/null || true
  chmod 644 /home/agent/.ssh/*.pub 2>/dev/null || true
  chmod 644 /home/agent/.ssh/known_hosts 2>/dev/null || true
  chmod 644 /home/agent/.ssh/config 2>/dev/null || true
fi

export HOME=/home/agent

cat > "$AW_ENV_FILE" <<EOF
if [ -n "\${AW_BASH_ENV_LOADED:-}" ] || [ -n "\${AW_BASH_ENV_RECURSION_GUARD:-}" ]; then
  return 0
fi
AW_BASH_ENV_LOADED=1
export HOME=/home/agent

. /home/agent/.nix-profile/etc/profile.d/nix.sh 2>/dev/null
case ":\${PATH}:" in
  *:/home/agent/.nix-profile/bin:*) ;;
  *) export PATH="/home/agent/.nix-profile/bin:\${PATH}" ;;
esac
case ":\${PATH}:" in
  *:/home/agent/.local/bin:*) ;;
  *) export PATH="/home/agent/.local/bin:\${PATH}" ;;
esac
export AW_BASH_ENV_RECURSION_GUARD=1
eval "\$(devbox global shellenv --install 2>/dev/null | grep '^export ' || true)"
unset AW_BASH_ENV_RECURSION_GUARD
case ":\${PATH}:" in
  *:/home/agent/.local/share/mise/shims:*) ;;
  *) export PATH="/home/agent/.local/share/mise/shims:\${PATH}" ;;
esac
export MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE:-/workspace}"
export MISE_YES=1
EOF

cat > "$BASHRC_FILE" <<'BASHRC'
if [ -f /home/agent/.aw_env.sh ]; then
  . /home/agent/.aw_env.sh
fi
BASHRC

cat > "$BASH_PROFILE_FILE" <<'BASH_PROFILE'
if [ -f /home/agent/.bashrc ]; then
  . /home/agent/.bashrc
fi
BASH_PROFILE

exec env HOME=/home/agent BASH_ENV="$AW_ENV_FILE" \
  bash -lc 'exec "$@"' -- "$@"
