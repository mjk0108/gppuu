#!/usr/bin/env bash
set -euo pipefail

ok() { echo "[OK] $1"; }
warn() { echo "[WARN] $1"; }
err() { echo "[ERR] $1"; }

need_cmd() {
  local cmd="$1"
  local hint="$2"
  if command -v "$cmd" >/dev/null 2>&1; then
    ok "$cmd: $("$cmd" --version 2>/dev/null | head -n1 || true)"
    return 0
  fi
  warn "$cmd 未安装。$hint"
  return 1
}

missing=0
need_cmd node "安装 Node.js 20+（推荐 LTS）" || missing=1
need_cmd npm "Node.js 安装后会自带 npm" || missing=1
need_cmd go "安装 Go 1.23+ https://go.dev/dl/" || missing=1

if command -v wails >/dev/null 2>&1; then
  ok "wails: $(wails version 2>/dev/null | head -n1 || true)"
else
  warn "wails 未安装。执行: go install github.com/wailsapp/wails/v2/cmd/wails@latest"
  missing=1
fi

echo
if [[ "$missing" -eq 1 ]]; then
  err "环境不完整。请先补齐依赖，再执行构建。"
  exit 1
fi

ok "环境检查通过。"
