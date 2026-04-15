#!/usr/bin/env bash
# =============================================================================
# Synapse 開發環境啟動腳本
# 用法：./scripts/dev.sh [選項]
#
#   --backend-only   只啟動後端（跳過前端）
#   --frontend-only  只啟動前端（PostgreSQL 仍會啟動）
#   --no-pg          不啟動 PostgreSQL（假設已在執行）
#   --build          啟動前先 go build（預設 go run）
#   --stop           停止所有服務並退出
#   --reset          清除 PostgreSQL volume 並重新初始化
# =============================================================================
set -euo pipefail

# ── 路徑 ──────────────────────────────────────────────────────────────────────
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ENV_FILE="$ROOT/.env.dev"
ENV_EXAMPLE="$ROOT/.env.dev.example"
COMPOSE="docker compose -f $ROOT/docker-compose.dev.yml"
PID_DIR="$ROOT/.dev"
BACKEND_PID="$PID_DIR/backend.pid"

# ── 顏色 ──────────────────────────────────────────────────────────────────────
R='\033[0;31m'; G='\033[0;32m'; Y='\033[1;33m'; B='\033[0;34m'; N='\033[0m'
info()  { echo -e "${B}[INFO]${N}  $*"; }
ok()    { echo -e "${G}[ OK ]${N}  $*"; }
warn()  { echo -e "${Y}[WARN]${N}  $*"; }
error() { echo -e "${R}[ERR ]${N}  $*" >&2; }
die()   { error "$*"; exit 1; }

# ── 旗標解析 ──────────────────────────────────────────────────────────────────
OPT_BACKEND_ONLY=0
OPT_FRONTEND_ONLY=0
OPT_PG_ONLY=0
OPT_NO_PG=0
OPT_BUILD=0
OPT_STOP=0
OPT_RESET=0

for arg in "$@"; do
  case "$arg" in
    --backend-only)  OPT_BACKEND_ONLY=1  ;;
    --frontend-only) OPT_FRONTEND_ONLY=1 ;;
    --pg-only)       OPT_PG_ONLY=1       ;;
    --no-pg)         OPT_NO_PG=1         ;;
    --build)         OPT_BUILD=1         ;;
    --stop)          OPT_STOP=1          ;;
    --reset)         OPT_RESET=1         ;;
    *) die "未知選項: $arg（執行 ./scripts/dev.sh --help 查看說明）" ;;
  esac
done

# ── 停止 ──────────────────────────────────────────────────────────────────────
stop_all() {
  info "停止後端程序..."
  if [ -f "$BACKEND_PID" ]; then
    PID=$(cat "$BACKEND_PID")
    if kill -0 "$PID" 2>/dev/null; then
      kill "$PID" && ok "後端已停止 (PID $PID)"
    fi
    rm -f "$BACKEND_PID"
  else
    warn "找不到後端 PID 檔，跳過"
  fi

  info "停止 Docker 服務..."
  $COMPOSE down
  ok "Docker 服務已停止"
}

if [ "$OPT_STOP" -eq 1 ]; then
  stop_all
  exit 0
fi

# ── 重置 ──────────────────────────────────────────────────────────────────────
if [ "$OPT_RESET" -eq 1 ]; then
  warn "重置將清除 PostgreSQL volume（synapse-pg-dev-data），資料將遺失！"
  read -rp "確認重置？[y/N] " confirm
  [[ "$confirm" =~ ^[Yy]$ ]] || die "已取消"
  $COMPOSE down -v
  ok "PostgreSQL volume 已清除，重新啟動..."
fi

# ── 前置檢查 ──────────────────────────────────────────────────────────────────
check_cmd() {
  command -v "$1" &>/dev/null || die "找不到 $1，請先安裝"
}

check_cmd docker
check_cmd go
[ "$OPT_FRONTEND_ONLY" -eq 0 ] && [ "$OPT_BACKEND_ONLY" -eq 0 ] && check_cmd node || true

# ── 環境變數 ──────────────────────────────────────────────────────────────────
if [ ! -f "$ENV_FILE" ]; then
  if [ -f "$ENV_EXAMPLE" ]; then
    warn ".env.dev 不存在，從範本建立..."
    cp "$ENV_EXAMPLE" "$ENV_FILE"
    ok "已建立 .env.dev，請確認設定後重新執行"
    echo -e "  → 編輯: ${Y}$ENV_FILE${N}"
    exit 0
  else
    die ".env.dev 不存在，且找不到 .env.dev.example"
  fi
fi

info "載入環境變數：$ENV_FILE"
set -o allexport
# shellcheck source=/dev/null
source "$ENV_FILE"
set +o allexport

mkdir -p "$PID_DIR"

# ── PostgreSQL ────────────────────────────────────────────────────────────────
wait_for_pg() {
  info "等待 PostgreSQL 就緒..."
  MAX_WAIT=60
  ELAPSED=0
  until $COMPOSE exec -T postgres pg_isready \
      -U "${PG_USER:-synapse}" \
      -d "${PG_DATABASE:-synapse}" \
      -q 2>/dev/null; do
    ELAPSED=$((ELAPSED + 2))
    if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
      error "PostgreSQL 在 ${MAX_WAIT}s 內未就緒，查看日誌："
      $COMPOSE logs --tail=20 postgres
      die "啟動失敗"
    fi
    printf "  等待中... %ds\r" "$ELAPSED"
    sleep 2
  done
  ok "PostgreSQL 就緒"
}

if [ "$OPT_PG_ONLY" -eq 1 ]; then
  info "啟動 PostgreSQL + Adminer..."
  $COMPOSE up -d
  wait_for_pg

  echo ""
  echo -e "  ${G}PostgreSQL${N} → 127.0.0.1:${PG_PORT:-5432}  (${PG_USER:-synapse} / ${PG_PASSWORD:-Synapse@2026})"
  echo -e "  ${G}Adminer${N}    → http://localhost:${ADMINER_PORT:-8080}"
  echo ""
  exit 0
fi

if [ "$OPT_NO_PG" -eq 0 ]; then
  info "啟動 PostgreSQL + Adminer..."
  $COMPOSE up -d
  wait_for_pg

  PG_PORT_ACTUAL="${PG_PORT:-5432}"
  ADMINER_PORT_ACTUAL="${ADMINER_PORT:-8080}"
  echo ""
  echo -e "  ${G}PostgreSQL${N} → 127.0.0.1:${PG_PORT_ACTUAL}  (${PG_USER:-synapse} / ${PG_PASSWORD:-Synapse@2026})"
  echo -e "  ${G}Adminer${N}    → http://localhost:${ADMINER_PORT_ACTUAL}"
  echo ""
fi

# ── 後端 ──────────────────────────────────────────────────────────────────────
start_backend() {
  # 若已有 PID 且程序仍在執行，跳過
  if [ -f "$BACKEND_PID" ]; then
    OLD=$(cat "$BACKEND_PID")
    if kill -0 "$OLD" 2>/dev/null; then
      warn "後端已在執行 (PID $OLD)，跳過"
      return
    fi
    rm -f "$BACKEND_PID"
  fi

  # 匯出資料庫連線設定
  export DB_DRIVER="${DB_DRIVER:-postgres}"
  export DB_HOST="${DB_HOST:-127.0.0.1}"
  export DB_PORT="${DB_PORT:-5432}"
  export DB_USERNAME="${DB_USERNAME:-synapse}"
  export DB_PASSWORD="${DB_PASSWORD:-${PG_PASSWORD:-Synapse@2026}}"
  export DB_DATABASE="${DB_DATABASE:-synapse}"
  export DB_SSL_MODE="${DB_SSL_MODE:-disable}"
  export APP_ENV="${APP_ENV:-development}"
  export SERVER_MODE="${SERVER_MODE:-debug}"
  export LOG_LEVEL="${LOG_LEVEL:-info}"

  cd "$ROOT"

  if [ "$OPT_BUILD" -eq 1 ]; then
    info "編譯後端..."
    CGO_ENABLED=0 go build -o bin/synapse . \
      && ok "編譯完成：bin/synapse" \
      || die "編譯失敗"
    info "啟動後端（bin/synapse）..."
    bin/synapse &
  elif command -v air &>/dev/null; then
    info "啟動後端（air 熱重載）..."
    air &
  else
    info "啟動後端（go run）— 提示：執行 go install github.com/air-verse/air@latest 啟用熱重載"
    go run . &
  fi

  BPID=$!
  echo "$BPID" > "$BACKEND_PID"

  # 等待後端健康
  SERVER_PORT="${SERVER_PORT:-8080}"
  MAX_WAIT=30
  ELAPSED=0
  until curl -sf "http://localhost:${SERVER_PORT}/healthz" &>/dev/null; do
    ELAPSED=$((ELAPSED + 1))
    if [ "$ELAPSED" -ge "$MAX_WAIT" ]; then
      error "後端在 ${MAX_WAIT}s 內未回應"
      kill "$BPID" 2>/dev/null || true
      rm -f "$BACKEND_PID"
      die "後端啟動失敗"
    fi
    sleep 1
  done
  ok "後端就緒 → http://localhost:${SERVER_PORT}  (PID $BPID)"
}

if [ "$OPT_FRONTEND_ONLY" -eq 0 ]; then
  start_backend
fi

# ── 前端 ──────────────────────────────────────────────────────────────────────
start_frontend() {
  UI_DIR="$ROOT/ui"
  [ -d "$UI_DIR" ] || { warn "ui/ 目錄不存在，跳過前端"; return; }

  if [ ! -d "$UI_DIR/node_modules" ]; then
    info "安裝前端依賴..."
    (cd "$UI_DIR" && npm install) || die "npm install 失敗"
  fi

  info "啟動前端開發伺服器..."
  (cd "$UI_DIR" && npm run dev)
  # npm run dev 是前景程序，腳本在此阻塞直到 Ctrl+C
}

if [ "$OPT_BACKEND_ONLY" -eq 0 ]; then
  start_frontend
else
  # 純後端模式：前景等待 Ctrl+C
  echo ""
  info "後端運行中（按 Ctrl+C 停止）"
  trap 'stop_all; exit 0' INT TERM
  wait
fi
