#!/bin/bash
set -e

# Create symlink to host-side claude home path (runs as root)
# installed_plugins.json etc. reference host absolute paths
if [ -n "$HOST_CLAUDE_HOME" ] && [ "$HOST_CLAUDE_HOME" != "/home/claude/.claude" ]; then
  mkdir -p "$(dirname "$HOST_CLAUDE_HOME")"
  ln -sfn /home/claude/.claude "$HOST_CLAUDE_HOME"
fi

# Fix permissions on .local volume for claude user
chown -R claude:claude /home/claude/.local

# Install tools via mise
WORKSPACE="${HOST_WORKSPACE:-/workspace}"
MISE_CMD="export HOME=/home/claude && export MISE_DATA_DIR=/home/claude/.local/share/mise && export MISE_CONFIG_DIR=/home/claude/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$WORKSPACE && export MISE_YES=1"
mkdir -p /home/claude/.config/mise
chown -R claude:claude /home/claude/.config

if [ -f "$WORKSPACE/mise.toml" ] || [ -f "$WORKSPACE/.mise.toml" ]; then
  echo "Installing tools from mise.toml..."
  su -s /bin/bash claude -c "$MISE_CMD && cd $WORKSPACE && mise trust --all && mise install"
  if su -s /bin/bash claude -c "$MISE_CMD && cd $WORKSPACE && mise tasks 2>/dev/null | grep -q '^install '"; then
    echo "Running mise run install..."
    su -s /bin/bash claude -c "$MISE_CMD && cd $WORKSPACE && mise run install"
  fi
else
  # No mise.toml: install Node.js (required for Claude Code)
  su -s /bin/bash claude -c "$MISE_CMD && mise use --global node@lts"
fi

# Install Claude Code if not present (as claude user)
if [ ! -x /home/claude/.local/bin/claude ]; then
  echo "Installing Claude Code..."
  su -s /bin/bash claude -c 'curl -fsSL https://claude.ai/install.sh | bash'
fi

# Copy and fix permissions on mounted .ssh (read-only mount, so copy first)
if [ -d /home/claude/.ssh-host ]; then
  cp -a /home/claude/.ssh-host /home/claude/.ssh
  chown -R claude:claude /home/claude/.ssh
  chmod 700 /home/claude/.ssh
  chmod 600 /home/claude/.ssh/* 2>/dev/null || true
  chmod 644 /home/claude/.ssh/*.pub 2>/dev/null || true
  chmod 644 /home/claude/.ssh/known_hosts 2>/dev/null || true
  chmod 644 /home/claude/.ssh/config 2>/dev/null || true
fi

# Fix permissions on mounted configs (.config/gh, .config/gcloud, etc.)
if [ -d /home/claude/.config ]; then
  chown -R claude:claude /home/claude/.config
fi

# Fix permissions on mounted .gitconfig
if [ -f /home/claude/.gitconfig ]; then
  chown claude:claude /home/claude/.gitconfig
fi

# Fix workspace permissions for claude user
if [ -n "${HOST_WORKSPACE:-}" ]; then
  mkdir -p "$HOST_WORKSPACE" 2>/dev/null || true
  chown claude:claude "$HOST_WORKSPACE" 2>/dev/null || true
else
  chown claude:claude /workspace 2>/dev/null || true
fi

# Run command as claude user with mise shims in PATH
export HOME=/home/claude
exec setpriv --reuid=$(id -u claude) --regid=$(id -g claude) --init-groups \
  env HOME=/home/claude \
  PATH="/home/claude/.local/share/mise/shims:/home/claude/.local/bin:$PATH" \
  "$@"
