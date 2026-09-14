#!/bin/bash
# aw-deps.sh — auto-detect and install language dependencies.
# Sourced by entrypoint scripts when AW_AUTO_DEPS_INSTALL=1.
# Requires: AW_WORKSPACE, MISE_CMD, run_as_user, aw_log (from aw-init.sh / entrypoint)

aw_install_deps() {
  cd "$AW_WORKSPACE" || return

  # --- Python ---
  if [ -f "uv.lock" ]; then
    if run_as_user "$MISE_CMD && command -v uv" &>/dev/null; then
      echo "Installing Python dependencies from uv.lock..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && uv sync" || true
    else
      aw_log "WARN: uv.lock found but uv is not available. Add uv to mise.toml."
    fi
  elif [ -f "requirements.txt" ]; then
    if run_as_user "$MISE_CMD && command -v python3" &>/dev/null; then
      echo "Installing Python dependencies from requirements.txt..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && pip install -q -r requirements.txt" || true
    else
      aw_log "WARN: requirements.txt found but python3 is not available. Add python to mise.toml."
    fi
  elif [ -f "pyproject.toml" ] && grep -q '\[project\]' pyproject.toml 2>/dev/null; then
    if run_as_user "$MISE_CMD && command -v python3" &>/dev/null; then
      echo "Installing Python dependencies from pyproject.toml..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && pip install -q -e ." || true
    else
      aw_log "WARN: pyproject.toml found but python3 is not available. Add python to mise.toml."
    fi
  fi

  # --- Node.js ---
  if [ -f "package.json" ]; then
    if ! run_as_user "$MISE_CMD && command -v node" &>/dev/null; then
      aw_log "WARN: package.json found but node is not available. Add node to mise.toml."
    elif [ -f "pnpm-lock.yaml" ]; then
      if run_as_user "$MISE_CMD && command -v pnpm" &>/dev/null; then
        echo "Installing Node.js dependencies with pnpm..."
        run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && pnpm install --frozen-lockfile" || true
      else
        aw_log "WARN: pnpm-lock.yaml found but pnpm is not available. Add pnpm via npm or mise.toml."
      fi
    elif [ -f "yarn.lock" ]; then
      if run_as_user "$MISE_CMD && command -v yarn" &>/dev/null; then
        echo "Installing Node.js dependencies with yarn..."
        run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && yarn install --frozen-lockfile" || true
      else
        aw_log "WARN: yarn.lock found but yarn is not available. Add yarn via npm or mise.toml."
      fi
    elif [ -f "package-lock.json" ]; then
      echo "Installing Node.js dependencies with npm ci..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && npm ci" || true
    else
      echo "Installing Node.js dependencies with npm install..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && npm install" || true
    fi
  fi

  # --- Go ---
  if [ -f "go.mod" ]; then
    if run_as_user "$MISE_CMD && command -v go" &>/dev/null; then
      echo "Installing Go dependencies..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && go mod download" || true
    else
      aw_log "WARN: go.mod found but go is not available. Add go to mise.toml."
    fi
  fi

  # --- Ruby ---
  if [ -f "Gemfile" ]; then
    if run_as_user "$MISE_CMD && command -v ruby" &>/dev/null && run_as_user "$MISE_CMD && command -v bundle" &>/dev/null; then
      echo "Installing Ruby dependencies..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && bundle install" || true
    else
      aw_log "WARN: Gemfile found but ruby/bundler is not available. Add ruby to mise.toml."
    fi
  fi

  # --- Rust ---
  if [ -f "Cargo.toml" ]; then
    if run_as_user "$MISE_CMD && command -v cargo" &>/dev/null; then
      echo "Installing Rust dependencies..."
      run_as_user "$MISE_CMD && cd \"$AW_WORKSPACE\" && cargo fetch" || true
    else
      aw_log "WARN: Cargo.toml found but cargo is not available. Add rust to mise.toml."
    fi
  fi
}
