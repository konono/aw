#!/bin/bash
set -e

. /aw-init.sh

# Install workspace packages (devbox/mise)
if [ -f "$AW_WORKSPACE/devbox.json" ]; then
  echo "Installing packages from devbox.json..."
  run_as_user "cd \"$AW_WORKSPACE\" && devbox install"
  run_as_user "cd \"$AW_WORKSPACE\" && devbox run install 2>/dev/null" || true
elif [ -f "$AW_WORKSPACE/mise.toml" ] || [ -f "$AW_WORKSPACE/.mise.toml" ]; then
  if ! run_as_user 'command -v mise' > /dev/null 2>&1; then
    echo "Installing mise..."
    run_as_user 'curl https://mise.jdx.dev/install.sh | sh'
  fi
  MISE_CMD="export HOME=$AW_HOME && export MISE_DATA_DIR=$AW_HOME/.local/share/mise && export MISE_CONFIG_DIR=$AW_HOME/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$AW_WORKSPACE && export MISE_YES=1"
  mkdir -p "$AW_HOME/.config/mise"
  echo "Installing tools from mise.toml..."
  run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && mise trust --all && mise install"
  if run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && mise tasks 2>/dev/null | grep -q '^install '"; then
    echo "Running mise run install..."
    run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && mise run install"
  fi
else
  echo "No devbox.json or mise.toml found."
fi

# Install Playwright browsers if npx is available
if run_as_user 'eval "$(devbox global shellenv 2>/dev/null)"; command -v npx' > /dev/null 2>&1; then
  if [ ! -d "$AW_HOME/.cache/ms-playwright" ]; then
    echo "Installing Playwright browsers..."
    run_as_user 'eval "$(devbox global shellenv 2>/dev/null)"; npx -y playwright install chromium'
  fi
fi

aw_exec "$@"
