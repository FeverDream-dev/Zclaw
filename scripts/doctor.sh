#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [[ -z "${NO_COLOR:-}" ]]; then
    RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
else
    RED=''; GREEN=''; YELLOW=''; CYAN=''; NC=''
fi

PASS=0
WARN=0
FAIL=0

check_pass() { PASS=$((PASS+1)); printf "${GREEN}[PASS]${NC} %s\n" "$*"; }
check_warn() { WARN=$((WARN+1)); printf "${YELLOW}[WARN]${NC} %s\n" "$*"; }
check_fail() { FAIL=$((FAIL+1)); printf "${RED}[FAIL]${NC} %s\n" "$*"; }

VERBOSE=false
JSON_OUTPUT=false

for arg in "$@"; do
    case "$arg" in
        --verbose) VERBOSE=true ;;
        --json) JSON_OUTPUT=true ;;
        --help|-h)
            echo "Usage: doctor.sh [--verbose] [--json]"
            exit 0 ;;
    esac
done

cd "$PROJECT_DIR"

# 1. Docker Engine
if docker info &>/dev/null; then
    DOCKER_VER=$(docker version --format '{{.Server.Version}}' 2>/dev/null)
    check_pass "Docker Engine ${DOCKER_VER} is running"
else
    check_fail "Docker Engine is not running"
fi

# 2. Docker Compose v2
if docker compose version &>/dev/null; then
    COMPOSE_VER=$(docker compose version --short 2>/dev/null)
    check_pass "Docker Compose v2 (${COMPOSE_VER})"
else
    check_fail "Docker Compose v2 not available"
fi

# 3. Disk space
FREE_GB=$(df -BG "$PROJECT_DIR/data" 2>/dev/null | awk 'NR==2{print $4}' | tr -d 'G')
if [[ -n "${FREE_GB:-}" ]]; then
    if [[ "$FREE_GB" -ge 5 ]]; then
        check_pass "Disk space: ${FREE_GB}GB free"
    else
        check_warn "Disk space low: ${FREE_GB}GB free (recommend >= 5GB)"
    fi
else
    check_warn "Could not check disk space"
fi

# 4. Ports
for PORT in 8080 8081 9222; do
    if ss -tlnp 2>/dev/null | grep -q ":${PORT} " || netstat -tlnp 2>/dev/null | grep -q ":${PORT} "; then
        # Port is in use - check if it's our containers
        if docker compose -f docker/compose.yaml ps --format json 2>/dev/null | grep -q "$PORT"; then
            check_pass "Port ${PORT} in use by ZClaw"
        else
            check_warn "Port ${PORT} in use by another process"
        fi
    else
        check_pass "Port ${PORT} available"
    fi
done

# 5. .env file
if [[ -f .env ]]; then
    check_pass ".env file exists"
    if grep -q "^OPENAI_API_KEY=$" .env && grep -q "^ANTHROPIC_API_KEY=$" .env; then
        check_warn "No API keys configured in .env"
    else
        check_pass "API keys configured in .env"
    fi
else
    check_fail ".env file missing (copy docker/env.example to .env)"
fi

# 6. Data directory
if [[ -d data ]]; then
    check_pass "data/ directory exists"
else
    check_warn "data/ directory missing (created on first run)"
fi

# 7. Docker socket
if [[ -S /var/run/docker.sock ]]; then
    check_pass "Docker socket accessible"
else
    check_warn "Docker socket not found at /var/run/docker.sock"
fi

# 8. Control plane health
if curl -sf http://localhost:8081/health >/dev/null 2>&1; then
    HEALTH=$(curl -sf http://localhost:8081/health 2>/dev/null)
    STATUS=$(echo "$HEALTH" | grep -o '"status":"[^"]*"' | head -1 | cut -d'"' -f4)
    check_pass "Control plane healthy (${STATUS:-unknown})"
else
    if docker compose -f docker/compose.yaml ps --status running 2>/dev/null | grep -q control-plane; then
        check_warn "Control plane container running but health check failed"
    else
        check_fail "Control plane is not running"
    fi
fi

# 9. Browser worker health
if curl -sf http://localhost:9222/health >/dev/null 2>&1; then
    check_pass "Browser worker healthy"
else
    check_warn "Browser worker not reachable (may not be started yet)"
fi

# 10. SQLite database
if [[ -f data/zclaw.db ]]; then
    SIZE=$(stat -c%s data/zclaw.db 2>/dev/null || echo "0")
    check_pass "SQLite database exists ($(($SIZE / 1024))KB)"
else
    check_warn "SQLite database not found (created on first run)"
fi

echo ""
echo "──────────────────────────────────"
echo " Results: ${GREEN}${PASS} pass${NC}  ${YELLOW}${WARN} warn${NC}  ${RED}${FAIL} fail${NC}"
echo "──────────────────────────────────"

if [[ "$JSON_OUTPUT" == "true" ]]; then
    printf '{"pass":%d,"warn":%d,"fail":%d}\n' "$PASS" "$WARN" "$FAIL"
fi

if [[ "$FAIL" -gt 0 ]]; then
    exit 1
fi
exit 0
