#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [[ -z "${NO_COLOR:-}" ]]; then
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; NC=''
fi

info()    { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }
warn()    { printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }
error()   { printf "${RED}[FAIL]${NC} %s\n" "$*"; }
success() { printf "${GREEN}[ OK ]${NC} %s\n" "$*"; }

SKIP_BACKUP=false

for arg in "$@"; do
    case "$arg" in
        --skip-backup) SKIP_BACKUP=true ;;
        --help|-h)
            echo "Usage: update.sh [--skip-backup]"
            exit 0 ;;
    esac
done

cd "$PROJECT_DIR"

# Check stack is running
if ! docker compose -f docker/compose.yaml ps --status running | grep -q control-plane 2>/dev/null; then
    error "ZClaw stack is not running. Start it first: ./scripts/install.sh"
    exit 1
fi
success "Stack is running"

# Backup
if [[ "$SKIP_BACKUP" == "false" ]]; then
    info "Creating backup before update..."
    bash "$SCRIPT_DIR/backup.sh" 2>&1 || warn "Backup failed, continuing anyway"
else
    warn "Skipping backup (--skip-backup)"
fi

# Pull latest images
info "Pulling latest images..."
docker compose -f docker/compose.yaml pull 2>&1 || true

# Rebuild if needed
info "Rebuilding images..."
docker compose -f docker/compose.yaml build 2>&1

# Run migrations
info "Running database migrations..."
docker compose -f docker/compose.yaml exec control-plane \
    /usr/local/bin/dockclawd migrate 2>&1 || warn "Migration returned non-zero"

# Restart services
info "Restarting services..."
docker compose -f docker/compose.yaml up -d 2>&1

# Wait for health
info "Waiting for health checks..."
TIMEOUT=60
ELAPSED=0
while [[ $ELAPSED -lt $TIMEOUT ]]; do
    if curl -sf http://localhost:8081/health >/dev/null 2>&1; then
        break
    fi
    sleep 2
    ELAPSED=$((ELAPSED + 2))
done

if [[ $ELAPSED -ge $TIMEOUT ]]; then
    error "Health check timed out"
    echo "Rollback: docker compose -f docker/compose.yaml down && restore from latest backup"
    exit 1
fi

success "Update complete"
echo ""
echo "  API:    http://localhost:8080"
echo "  Health: http://localhost:8081/health"
echo ""
