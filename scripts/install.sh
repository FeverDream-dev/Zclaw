#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# Colors (respect NO_COLOR)
if [[ -z "${NO_COLOR:-}" ]]; then
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; NC='\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; NC=''
fi

info()    { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }
warn()    { printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }
error()   { printf "${RED}[FAIL]${NC} %s\n" "$*"; }
success() { printf "${GREEN}[ OK ]${NC} %s\n" "$*"; }

NON_INTERACTIVE=false
SKIP_BUILD=false

for arg in "$@"; do
    case "$arg" in
        --non-interactive) NON_INTERACTIVE=true ;;
        --skip-build) SKIP_BUILD=true ;;
        --help|-h)
            echo "Usage: install.sh [--non-interactive] [--skip-build]"
            exit 0 ;;
    esac
done

cd "$PROJECT_DIR"

# Check Docker Engine
if ! command -v docker &>/dev/null; then
    error "Docker Engine is not installed."
    echo "Install: https://docs.docker.com/engine/install/"
    exit 1
fi

DOCKER_VERSION=$(docker version --format '{{.Server.Version}}' 2>/dev/null || echo "0.0")
if [[ "$DOCKER_VERSION" == "0.0" ]]; then
    error "Docker daemon is not running."
    echo "Start with: sudo systemctl start docker"
    exit 1
fi
success "Docker Engine ${DOCKER_VERSION} detected"

# Check Docker Compose v2
if ! docker compose version &>/dev/null; then
    error "Docker Compose v2 is not available."
    echo "Install: https://docs.docker.com/compose/install/"
    exit 1
fi
COMPOSE_VERSION=$(docker compose version --short 2>/dev/null)
success "Docker Compose ${COMPOSE_VERSION} detected"

# Check if running as root
if [[ "$(id -u)" -eq 0 ]]; then
    warn "Running as root. Consider using a non-root user."
fi

# Create directories
info "Creating data directories..."
mkdir -p data/workspaces data/artifacts data/logs data/backups
success "Directories created"

# Initialize .env
if [[ ! -f .env ]]; then
    cp docker/env.example .env
    info "Created .env from template"
else
    success ".env already exists"
fi

# Prompt for API keys
if [[ "$NON_INTERACTIVE" == "false" ]]; then
    if [[ -z "${OPENAI_API_KEY:-}" ]]; then
        echo ""
        read -rp "Enter OpenAI API Key (or press Enter to skip): " OPENAI_KEY
        if [[ -n "$OPENAI_KEY" ]]; then
            if grep -q "^OPENAI_API_KEY=$" .env; then
                sed -i "s/^OPENAI_API_KEY=$/OPENAI_API_KEY=${OPENAI_KEY}/" .env
            fi
        fi
    fi

    if [[ -z "${ANTHROPIC_API_KEY:-}" ]]; then
        read -rp "Enter Anthropic API Key (or press Enter to skip): " ANTHROPIC_KEY
        if [[ -n "$ANTHROPIC_KEY" ]]; then
            if grep -q "^ANTHROPIC_API_KEY=$" .env; then
                sed -i "s/^ANTHROPIC_API_KEY=$/ANTHROPIC_API_KEY=${ANTHROPIC_KEY}/" .env
            fi
        fi
    fi
fi

# Build / Pull images
if [[ "$SKIP_BUILD" == "false" ]]; then
    info "Building images..."
    docker compose -f docker/compose.yaml build 2>&1
    success "Images built"
else
    info "Skipping image build (--skip-build)"
fi

# Start the stack
info "Starting ZClaw stack..."
docker compose -f docker/compose.yaml up -d 2>&1
success "Stack started"

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
    error "Health check timed out after ${TIMEOUT}s"
    echo "Check logs: docker compose -f docker/compose.yaml logs"
    exit 1
fi
success "Control plane is healthy"

echo ""
success "ZClaw is running!"
echo ""
echo "  API:        http://localhost:8080"
echo "  Health:     http://localhost:8081/health"
echo "  Browser:    http://localhost:9222/health"
echo ""
echo "  CLI:        docker compose -f docker/compose.yaml exec control-plane dockclawctl agent list"
echo "  Logs:       docker compose -f docker/compose.yaml logs -f"
echo "  Stop:       docker compose -f docker/compose.yaml down"
echo "  Doctor:     ./scripts/doctor.sh"
echo ""
