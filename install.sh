#!/usr/bin/env bash
# install.sh — install llmeval binary and Claude Code slash command
#
# Does NOT require Go. Downloads a pre-built binary from the latest GitHub release.
# Falls back to `go install` if no pre-built binary exists for your platform.
#
# Usage:
#   bash <(curl -fsSL https://raw.githubusercontent.com/farhaan/llmeval/main/install.sh)
#   bash install.sh          # from repo root
set -euo pipefail

REPO_OWNER="farhaan"
REPO_NAME="llmeval"
BINARY="llmeval"
CMD_SRC=".claude/commands/llmeval.md"

info()  { echo "  $*"; }
ok()    { echo "  ✓ $*"; }
warn()  { echo "  ⚠ $*"; }
die()   { echo "ERROR: $*" >&2; exit 1; }

require() {
  command -v "$1" &>/dev/null || die "$1 is required but not found. Install it and retry."
}


detect_platform() {
  local os arch

  case "$(uname -s)" in
    Linux)  os="linux"  ;;
    Darwin) os="darwin" ;;
    *)      echo ""; return ;;
  esac

  case "$(uname -m)" in
    x86_64)          arch="amd64" ;;
    aarch64 | arm64) arch="arm64" ;;
    *)               echo ""; return ;;
  esac

  echo "${os}_${arch}"
}

# Prefer ~/.local/bin (no sudo needed). Fall back to /usr/local/bin if writable.
install_dir() {
  if [[ -w "/usr/local/bin" ]]; then
    echo "/usr/local/bin"
  else
    echo "${HOME}/.local/bin"
  fi
}

ensure_in_path() {
  local dir="$1"
  if [[ ":${PATH}:" != *":${dir}:"* ]]; then
    warn "${dir} is not in your PATH."
    warn "Add the following to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
    warn "  export PATH=\"${dir}:\$PATH\""
  fi
}

# Install binary

echo "Installing ${BINARY} binary..."

PLATFORM="$(detect_platform)"
INSTALL_DIR="$(install_dir)"
mkdir -p "${INSTALL_DIR}"

installed=false

if [[ -n "${PLATFORM}" ]]; then
  require curl

  # Fetch the latest release tag from the GitHub API.
  LATEST_TAG="$(curl -fsSL "https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest" \
    | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"\(.*\)".*/\1/')" || true

  if [[ -n "${LATEST_TAG}" ]]; then
    # GoReleaser archive name: llmeval_0.1.0_linux_amd64.tar.gz
    VERSION="${LATEST_TAG#v}"
    ARCHIVE="${BINARY}_${VERSION}_${PLATFORM}.tar.gz"
    URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download/${LATEST_TAG}/${ARCHIVE}"

    info "Downloading ${ARCHIVE} ..."
    TMP="$(mktemp -d)"
    trap 'rm -rf "${TMP}"' EXIT

    if curl -fsSL "${URL}" -o "${TMP}/${ARCHIVE}"; then
      tar -xzf "${TMP}/${ARCHIVE}" -C "${TMP}"
      install -m 755 "${TMP}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
      ok "${INSTALL_DIR}/${BINARY} (${LATEST_TAG})"
      installed=true
    else
      warn "Pre-built binary not available for ${PLATFORM}. Trying go install..."
    fi
  else
    warn "Could not fetch latest release tag. Trying go install..."
  fi
else
  warn "Unsupported platform ($(uname -s)/$(uname -m)). Trying go install..."
fi

if [[ "${installed}" == false ]]; then
  if command -v go &>/dev/null; then
    go install "github.com/${REPO_OWNER}/${REPO_NAME}/cmd/${BINARY}@latest"
    GOBIN="$(go env GOPATH)/bin"
    ok "${GOBIN}/${BINARY} ($(${GOBIN}/${BINARY} version 2>/dev/null || echo 'installed'))"
  else
    die "No pre-built binary available for your platform and Go is not installed.\n" \
        "Install Go from https://go.dev/dl/ and re-run this script."
  fi
fi

ensure_in_path "${INSTALL_DIR}"

# Install slash command

echo ""
echo "Installing Claude Code slash command..."

GLOBAL_CMDS="${HOME}/.claude/commands"
mkdir -p "${GLOBAL_CMDS}"

if [[ -f "${CMD_SRC}" ]]; then
  cp "${CMD_SRC}" "${GLOBAL_CMDS}/llmeval.md"
else
  require curl
  curl -fsSL "https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/${CMD_SRC}" \
    -o "${GLOBAL_CMDS}/llmeval.md"
fi
ok "Slash command installed to ${GLOBAL_CMDS}/llmeval.md"

# Register Claude Code plugin for this directory 

echo ""
echo "Registering Claude Code plugin..."

PLUGIN_DIR="$(cd "$(dirname "$0")" && pwd)"
if command -v claude &>/dev/null; then
  claude plugins marketplace add "${PLUGIN_DIR}" 2>/dev/null || true
  claude plugins install "${BINARY}@${BINARY}" --scope project 2>/dev/null || true
  ok "Plugin registered for ${PLUGIN_DIR}"
  info "Run /reload-plugins in Claude Code to activate."
else
  warn "claude CLI not found — skipping plugin registration."
  info "Install Claude Code and re-run this script to enable /llmeval subcommands."
fi

echo ""
echo "Provider API key status (run 'llmeval providers' anytime to recheck):"
echo ""
"${BINARY}" providers 2>/dev/null || true
echo ""
echo "Set keys via 'export KEY=...' or a .env file in your project root."
echo "Add '.env' and '.llmeval/' to .gitignore."
echo ""
echo "Done. Run /llmeval inside Claude Code to get started."
