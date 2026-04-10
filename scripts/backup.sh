#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

if [[ -z "${NO_COLOR:-}" ]]; then
    GREEN='\033[0;32m'; NC='\033[0m'
else
    GREEN=''; NC=''
fi

info() { printf "${GREEN}[INFO]${NC} %s\n" "$*"; }

OUTPUT_DIR="${PROJECT_DIR}/data/backups"
COMPRESS=true
NO_DB=false

for arg in "$@"; do
    case "$arg" in
        --output-dir=*) OUTPUT_DIR="${arg#*=}" ;;
        --no-compress)  COMPRESS=false ;;
        --no-db)        NO_DB=true ;;
        --help|-h)
            echo "Usage: backup.sh [--output-dir=DIR] [--no-compress] [--no-db]"
            exit 0 ;;
    esac
done

cd "$PROJECT_DIR"

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_NAME="zclaw-backup-${TIMESTAMP}"
BACKUP_DIR="${OUTPUT_DIR}/${BACKUP_NAME}"

mkdir -p "$BACKUP_DIR"

# Backup SQLite database
if [[ "$NO_DB" == "false" && -f data/zclaw.db ]]; then
    info "Backing up SQLite database..."
    if docker compose -f docker/compose.yaml ps --status running 2>/dev/null | grep -q control-plane; then
        docker compose -f docker/compose.yaml exec -T control-plane \
            /usr/local/bin/dockclawd backup-db > "${BACKUP_DIR}/zclaw.sql" 2>/dev/null || true
    fi
    cp data/zclaw.db "${BACKUP_DIR}/zclaw.db" 2>/dev/null || true
    if [[ -f data/zclaw.db-wal ]]; then
        cp data/zclaw.db-wal "${BACKUP_DIR}/zclaw.db-wal" 2>/dev/null || true
    fi
    if [[ -f data/zclaw.db-shm ]]; then
        cp data/zclaw.db-shm "${BACKUP_DIR}/zclaw.db-shm" 2>/dev/null || true
    fi
    info "Database backed up"
fi

# Backup .env
if [[ -f .env ]]; then
    cp .env "${BACKUP_DIR}/.env"
    info "Configuration backed up"
fi

# Backup workspaces
if [[ -d data/workspaces ]] && [[ "$(ls -A data/workspaces 2>/dev/null)" ]]; then
    info "Backing up workspaces..."
    tar cf "${BACKUP_DIR}/workspaces.tar" -C data workspaces 2>/dev/null || true
fi

# Compress
if [[ "$COMPRESS" == "true" ]]; then
    info "Compressing backup..."
    tar czf "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" -C "$OUTPUT_DIR" "$BACKUP_NAME"
    rm -rf "$BACKUP_DIR"

    SIZE=$(stat -c%s "${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz" 2>/dev/null || echo "0")
    SIZE_MB=$((SIZE / 1024 / 1024))
    info "Backup: ${OUTPUT_DIR}/${BACKUP_NAME}.tar.gz (${SIZE_MB}MB)"
else
    SIZE=$(du -sh "$BACKUP_DIR" 2>/dev/null | cut -f1 || echo "unknown")
    info "Backup: ${BACKUP_DIR} (${SIZE})"
fi

# Prune old backups
KEEP_DAYS=${KEEP_BACKUPS:-30}
DELETED=0
if [[ "$COMPRESS" == "true" ]]; then
    while IFS= read -r -d '' file; do
        if [[ "$(stat -c%Y "$file")" -lt "$(date -d "-${KEEP_DAYS} days" +%s 2>/dev/null || echo 0)" ]]; then
            rm -f "$file"
            DELETED=$((DELETED + 1))
        fi
    done < <(find "$OUTPUT_DIR" -name "zclaw-backup-*.tar.gz" -print0 2>/dev/null)
fi

if [[ "$DELETED" -gt 0 ]]; then
    info "Pruned ${DELETED} backup(s) older than ${KEEP_DAYS} days"
fi

info "Backup complete"
