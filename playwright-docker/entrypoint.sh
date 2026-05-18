#!/bin/bash
set -e

if [ -n "$AW_HOST_CONFIG_HOME" ] && [ -n "$AW_CONTAINER_CONFIG_DIR" ] \
   && [ "$AW_HOST_CONFIG_HOME" != "$AW_CONTAINER_CONFIG_DIR" ]; then
  mkdir -p "$(dirname "$AW_HOST_CONFIG_HOME")"
  ln -sfn "$AW_CONTAINER_CONFIG_DIR" "$AW_HOST_CONFIG_HOME"
fi

# Symlink .claude.json from staging dir to home (for onboarding state persistence)
if [ -n "$AW_CONTAINER_CONFIG_DIR" ] && [ -f "$AW_CONTAINER_CONFIG_DIR/.claude.json" ]; then
  ln -sfn "$AW_CONTAINER_CONFIG_DIR/.claude.json" /home/agent/.claude.json
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

chown -R agent:agent /home/agent/.local

WORKSPACE="${HOST_WORKSPACE:-/workspace}"

if [ -f "$WORKSPACE/devbox.json" ]; then
  echo "Installing packages from devbox.json..."
  su -s /bin/bash agent -c "cd $WORKSPACE && devbox install"
  su -s /bin/bash agent -c "cd $WORKSPACE && devbox run install 2>/dev/null" || true
elif [ -f "$WORKSPACE/mise.toml" ] || [ -f "$WORKSPACE/.mise.toml" ]; then
  if ! su -s /bin/bash agent -c 'command -v mise' > /dev/null 2>&1; then
    echo "Installing mise..."
    su -s /bin/bash agent -c 'curl https://mise.jdx.dev/install.sh | sh'
  fi
  MISE_CMD="export HOME=/home/agent && export MISE_DATA_DIR=/home/agent/.local/share/mise && export MISE_CONFIG_DIR=/home/agent/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$WORKSPACE && export MISE_YES=1"
  mkdir -p /home/agent/.config/mise
  chown -R agent:agent /home/agent/.config
  echo "Installing tools from mise.toml..."
  su -s /bin/bash agent -c "$MISE_CMD && cd $WORKSPACE && mise trust --all && mise install"
  if su -s /bin/bash agent -c "$MISE_CMD && cd $WORKSPACE && mise tasks 2>/dev/null | grep -q '^install '"; then
    echo "Running mise run install..."
    su -s /bin/bash agent -c "$MISE_CMD && cd $WORKSPACE && mise run install"
  fi
else
  echo "No devbox.json or mise.toml found. devbox is available for installing tools."
fi

# Install Playwright browsers if npx is available
if su -s /bin/bash agent -c 'eval "$(devbox global shellenv 2>/dev/null)"; command -v npx' > /dev/null 2>&1; then
  if [ ! -d /home/agent/.cache/ms-playwright ]; then
    echo "Installing Playwright browsers..."
    su -s /bin/bash agent -c 'eval "$(devbox global shellenv 2>/dev/null)"; npx -y playwright install chromium'
  fi
fi

if [ -d /home/agent/.ssh-host ]; then
  cp -a /home/agent/.ssh-host /home/agent/.ssh 2>/dev/null || true
  chown -R agent:agent /home/agent/.ssh
  chmod 700 /home/agent/.ssh
  chmod 600 /home/agent/.ssh/* 2>/dev/null || true
  chmod 644 /home/agent/.ssh/*.pub 2>/dev/null || true
  chmod 644 /home/agent/.ssh/known_hosts 2>/dev/null || true
  chmod 644 /home/agent/.ssh/config 2>/dev/null || true
fi

if [ -d /home/agent/.config ]; then
  chown -R agent:agent /home/agent/.config
fi
if [ -f /home/agent/.gitconfig ]; then
  chown agent:agent /home/agent/.gitconfig
fi
if [ -n "${HOST_WORKSPACE:-}" ]; then
  mkdir -p "$HOST_WORKSPACE" 2>/dev/null || true
  chown agent:agent "$HOST_WORKSPACE" 2>/dev/null || true
else
  chown agent:agent /workspace 2>/dev/null || true
fi

export HOME=/home/agent

# Write devbox/nix PATH setup to .bashrc so all child processes (including
# commands spawned by AI tools like `bash -c "uv sync"`) inherit the PATH.
cat > /home/agent/.bashrc <<BASHRC
. /home/agent/.nix-profile/etc/profile.d/nix.sh 2>/dev/null
eval "\$(devbox global shellenv --preserve-path-stack -r 2>/dev/null | grep '^export ')"
export PATH="/home/agent/.local/share/mise/shims:\$PATH"
export MISE_TRUSTED_CONFIG_PATHS="${HOST_WORKSPACE:-/workspace}"
export MISE_YES=1
BASHRC
chown agent:agent /home/agent/.bashrc

exec setpriv --reuid="$(id -u agent)" --regid="$(id -g agent)" --init-groups \
  env HOME=/home/agent \
  bash -lc 'exec "$@"' -- "$@"
