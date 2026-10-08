#!/bin/bash
set -e

. /aw-init.sh

MISE_CMD="export HOME=$AW_HOME && export MISE_DATA_DIR=$AW_HOME/.local/share/mise && export MISE_CONFIG_DIR=$AW_HOME/.config/mise && export MISE_TRUSTED_CONFIG_PATHS=$AW_WORKSPACE && export MISE_YES=1"

aw_log "Checking workspace packages..."
AW_PKG_FOUND=0
AW_MISE_FINGERPRINT_FILE="$AW_HOME/.aw_mise_fingerprint"
if [ -f "$AW_WORKSPACE/mise.toml" ] || [ -f "$AW_WORKSPACE/.mise.toml" ]; then
  if [ "${AW_SKIP_MISE_INSTALL:-}" = "1" ]; then
    echo "Skipping mise install (skip_mise_install is enabled)"
  elif [ -n "${AW_MISE_FINGERPRINT:-}" ] && [ -f "$AW_MISE_FINGERPRINT_FILE" ] &&
       [ "$(cat "$AW_MISE_FINGERPRINT_FILE")" = "$AW_MISE_FINGERPRINT" ]; then
    # aw build baked in the tools for exactly these inputs. Both sides of the
    # comparison come from the aw CLI, so a match means there is nothing to do.
    # The aw CLI leaves AW_MISE_FINGERPRINT unset whenever it cannot account
    # for every mise input, which lands in the install branch below.
    echo "Tools already baked into the image for this mise config. Skipping mise install."
  else
    if ! run_as_user 'command -v mise' > /dev/null 2>&1; then
      echo "Installing mise..."
      run_as_user 'export MISE_INSTALL_MUSL=1 && curl -fsSL https://mise.jdx.dev/install.sh | sh'
    fi
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

if [ "${AW_AUTO_DEPS_INSTALL:-}" = "1" ] && [ -f /aw-deps.sh ]; then
  . /aw-deps.sh
  aw_install_deps
fi

aw_exec "$@"
