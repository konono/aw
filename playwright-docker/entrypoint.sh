#!/bin/bash
set -e

if [ "${AW_LAUNCH_MODE:-claude}" = "claude" ]; then
  if [ -n "$HOST_CLAUDE_HOME" ] && [ "$HOST_CLAUDE_HOME" != "/home/claude/.claude" ]; then
    mkdir -p "$(dirname "$HOST_CLAUDE_HOME")"
    ln -sfn /home/claude/.claude "$HOST_CLAUDE_HOME"
  fi
fi

chown -R claude:claude /home/claude/.local

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
  su -s /bin/bash claude -c "$MISE_CMD && mise use --global node@lts gh@latest"
fi

# Install Playwright browsers if node is available
if su -s /bin/bash claude -c "$MISE_CMD && command -v npx" > /dev/null 2>&1; then
  if [ ! -d /home/claude/.cache/ms-playwright ]; then
    echo "Installing Playwright browsers..."
    su -s /bin/bash claude -c "$MISE_CMD && npx -y playwright install chromium"
  fi
fi

case "${AW_LAUNCH_MODE:-claude}" in
  claude)
    if [ ! -x /home/claude/.local/bin/claude ]; then
      echo "Installing Claude Code..."
      su -s /bin/bash claude -c 'curl -fsSL https://claude.ai/install.sh | bash'
    fi
    ;;
  codex)
    if ! su -s /bin/bash claude -c "export PATH=/home/claude/.local/share/mise/shims:/home/claude/.local/bin:\$PATH && command -v codex" > /dev/null 2>&1; then
      echo "Installing Codex CLI..."
      su -s /bin/bash claude -c "export PATH=/home/claude/.local/share/mise/shims:/home/claude/.local/bin:\$PATH && npm install -g @openai/codex"
    fi
    ;;
esac

if [ -d /home/claude/.ssh-host ]; then
  cp -a /home/claude/.ssh-host /home/claude/.ssh
  chown -R claude:claude /home/claude/.ssh
  chmod 700 /home/claude/.ssh
  chmod 600 /home/claude/.ssh/* 2>/dev/null || true
  chmod 644 /home/claude/.ssh/*.pub 2>/dev/null || true
  chmod 644 /home/claude/.ssh/known_hosts 2>/dev/null || true
  chmod 644 /home/claude/.ssh/config 2>/dev/null || true
fi

if [ -d /home/claude/.config ]; then
  chown -R claude:claude /home/claude/.config
fi

if [ -f /home/claude/.gitconfig ]; then
  chown claude:claude /home/claude/.gitconfig
fi

if [ -n "${HOST_WORKSPACE:-}" ]; then
  mkdir -p "$HOST_WORKSPACE" 2>/dev/null || true
  chown claude:claude "$HOST_WORKSPACE" 2>/dev/null || true
else
  chown claude:claude /workspace 2>/dev/null || true
fi

export HOME=/home/claude
exec setpriv --reuid=$(id -u claude) --regid=$(id -g claude) --init-groups \
  env HOME=/home/claude \
  PATH="/home/claude/.local/share/mise/shims:/home/claude/.local/bin:$PATH" \
  "$@"
