#!/bin/bash
set -e

. /aw-init.sh

aw_log "Checking workspace packages..."
AW_PKG_FOUND=0
if [ -f "$AW_WORKSPACE/mise.toml" ] || [ -f "$AW_WORKSPACE/.mise.toml" ]; then
  if [ "${AW_SKIP_MISE_INSTALL:-}" = "1" ]; then
    echo "Skipping mise install (skip_mise_install is enabled)"
  else
    if ! run_as_user 'command -v mise' > /dev/null 2>&1; then
      echo "Installing mise..."
      run_as_user 'export MISE_INSTALL_MUSL=1 && curl -fsSL https://mise.jdx.dev/install.sh | sh'
    fi
    MISE_CMD="export HOME=$AW_HOME && export MISE_DATA_DIR=$AW_HOME/.local/share/mise && export MISE_CONFIG_DIR=$AW_HOME/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$AW_WORKSPACE && export MISE_YES=1"
    mkdir -p "$AW_HOME/.config/mise"
    echo "Installing tools from mise.toml..."
    run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && mise install"
    aw_fix_mise_shims "$MISE_CMD"
  fi
  AW_PKG_FOUND=1
fi
if [ "$AW_PKG_FOUND" = "0" ]; then
  echo "No mise.toml found in workspace."
fi

aw_exec "$@"
