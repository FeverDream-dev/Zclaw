#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [[ -z "${NO_COLOR:-}" ]]; then
    GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
else
    GREEN=''; RED=''; NC=''
fi

info()    { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }
error()   { printf "${RED}[FAIL]${NC} %s\n" "$*"; }

COUNT=""
PREFIX="test-agent"
PROVIDER="openai"
MODEL="gpt-4o-mini"
SCHEDULE="*/5 * * * *"
CLEANUP=false
DRY_RUN=false
API_URL="http://localhost:8080"

for arg in "$@"; do
    case "$arg" in
        --count=*)     COUNT="${arg#*=}" ;;
        --prefix=*)    PREFIX="${arg#*=}" ;;
        --provider=*)  PROVIDER="${arg#*=}" ;;
        --model=*)     MODEL="${arg#*=}" ;;
        --schedule=*)  SCHEDULE="${arg#*=}" ;;
        --cleanup)     CLEANUP=true ;;
        --dry-run)     DRY_RUN=true ;;
        --api-url=*)   API_URL="${arg#*=}" ;;
        --help|-h)
            echo "Usage: scale-agents.sh --count N [options]"
            echo "  --count N       Number of agents to create (required)"
            echo "  --prefix NAME   Agent name prefix (default: test-agent)"
            echo "  --provider ID   Provider ID (default: openai)"
            echo "  --model NAME    Model name (default: gpt-4o-mini)"
            echo "  --schedule CRON Cron schedule (default: */5 * * * *)"
            echo "  --cleanup       Delete all agents matching prefix"
            echo "  --dry-run       Show what would be created"
            echo "  --api-url URL   API URL (default: http://localhost:8080)"
            exit 0 ;;
    esac
done

cd "$PROJECT_DIR"

# Cleanup mode
if [[ "$CLEANUP" == "true" ]]; then
    info "Fetching agents matching '${PREFIX}*'..."
    AGENTS=$(curl -sf "${API_URL}/api/v1/agents?limit=1000" 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
ids = [a['id'] for a in data.get('agents', []) if a['name'].startswith('${PREFIX}')]
for id in ids:
    print(id)
" 2>/dev/null || echo "")

    if [[ -z "$AGENTS" ]]; then
        info "No agents matching '${PREFIX}*' found"
        exit 0
    fi

    COUNT=0
    for id in $AGENTS; do
        if [[ "$DRY_RUN" == "true" ]]; then
            info "[dry-run] Would delete agent: ${id}"
        else
            curl -sf -X DELETE "${API_URL}/api/v1/agents/${id}" 2>/dev/null && info "Deleted: ${id}" || error "Failed: ${id}"
        fi
        COUNT=$((COUNT + 1))
    done
    info "Cleanup: ${COUNT} agents"
    exit 0
fi

# Create mode
if [[ -z "$COUNT" ]]; then
    error "--count is required"
    exit 1
fi

info "Creating ${COUNT} test agents (prefix: ${PREFIX}, provider: ${PROVIDER}, model: ${MODEL})"

CREATED=0
FAILED=0
for i in $(seq 1 "$COUNT"); do
    NAME="${PREFIX}-${i}"

    if [[ "$DRY_RUN" == "true" ]]; then
        info "[dry-run] Would create: ${NAME}"
        CREATED=$((CREATED + 1))
        continue
    fi

    RESPONSE=$(curl -sf -X POST "${API_URL}/api/v1/agents" \
        -H "Content-Type: application/json" \
        -d "{
            \"name\": \"${NAME}\",
            \"provider\": {
                \"provider_id\": \"${PROVIDER}\",
                \"model\": \"${MODEL}\",
                \"temperature\": 0.7
            },
            \"schedule\": {
                \"cron\": \"${SCHEDULE}\",
                \"enabled\": true,
                \"jitter_seconds\": 30
            },
            \"policy\": {
                \"allow_shell\": false,
                \"allow_browser\": false,
                \"max_memory_mb\": 128,
                \"timeout_seconds\": 60,
                \"max_concurrent_tasks\": 1
            }
        }" 2>/dev/null)

    if [[ $? -eq 0 ]]; then
        CREATED=$((CREATED + 1))
        printf "\r${GREEN}[INFO]${NC} Created %d/%d agents" "$CREATED" "$COUNT"
    else
        FAILED=$((FAILED + 1))
    fi
done

echo ""

# Verify
FINAL_COUNT=$(curl -sf "${API_URL}/api/v1/agents?limit=1000" 2>/dev/null | python3 -c "
import sys, json
data = json.load(sys.stdin)
print(sum(1 for a in data.get('agents', []) if a['name'].startswith('${PREFIX}')))
" 2>/dev/null || echo "unknown")

info "Created: ${CREATED}, Failed: ${FAILED}, Total matching: ${FINAL_COUNT}"
