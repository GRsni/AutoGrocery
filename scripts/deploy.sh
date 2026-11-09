#!/usr/bin/env bash
# Build, transfer, and load Docker image on Raspberry Pi
# Usage: ./deploy.sh <user@host> [tag] [remote-dest] [--force]
# Example: ./deploy.sh user@192.168.1.x autogrocery:latest
#          ./deploy.sh user@192.168.1.x autogrocery:v1.2 /mnt/data
#          ./deploy.sh user@192.168.1.x --force       (rebuild even if source unchanged)

set -euo pipefail

# ---------------------------------------------------------------------------
# Parse flags first (before positional args), strip them from $@
# ---------------------------------------------------------------------------
FORCE_BUILD=0
CLEAN_ARGS=()
for arg in "$@"; do
    if [ "$arg" = "--force" ]; then
        FORCE_BUILD=1
    else
        CLEAN_ARGS+=("$arg")
    fi
done
set -- "${CLEAN_ARGS[@]}"

HOST="${1:?Usage: $0 <user@host> [tag] [remote-dest]}"
TAG="${2:-autogrocery:latest}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="${SCRIPT_DIR%/scripts}"
TAR_FILE="autogrocery-latest.tar"
CACHE_FILE="$PROJECT_ROOT/.deploy-cache"

# Extract username from user@host (defaults to 'pi' if no @)
REMOTE_USER="${HOST%%@*}"
if [ "$REMOTE_USER" = "$HOST" ]; then
    REMOTE_USER="pi"
fi

DEST_PATH="${3:-/home/${REMOTE_USER}/autogrocery}"

# ---------------------------------------------------------------------------
# Caching: skip build/transfer if source files haven't changed
# ---------------------------------------------------------------------------
compute_source_hash() {
    # Hash every file that affects the built image (Dockerfile + sources)
    # Exclude deploy.sh itself — not part of the image, only orchestrates builds
    git -C "$PROJECT_ROOT" ls-files \
        Dockerfile go.mod go.sum cmd/ scripts/ config/ internal/ pkg/ \
        -- ':!scripts/deploy.sh' \
        2>/dev/null | sort | xargs sha256sum 2>/dev/null | sha256sum | cut -d' ' -f1
}

LAST_HASH=""
if [ -f "$CACHE_FILE" ]; then
    LAST_HASH="$(cat "$CACHE_FILE")"
fi
CURRENT_HASH="$(compute_source_hash)"

if [ "$FORCE_BUILD" -eq 0 ] && [ "$LAST_HASH" = "$CURRENT_HASH" ] && [ -n "$LAST_HASH" ]; then
    echo "=== No source changes since last deploy (hash: ${CURRENT_HASH:0:12}) ==="
    echo "Checking if image already exists on remote..."
    EXISTS="$(ssh "$HOST" "docker images --format '{{.Repository}}:{{.Tag}}' | grep -c '^$(echo "$TAG" | cut -d: -f1):$(echo "$TAG" | cut -d: -f2)$' || true")"
    if [ "$EXISTS" -gt 0 ]; then
        echo "Image '$TAG' already exists on remote. Nothing to do."
        exit 0
    else
        echo "Image not found on remote — will rebuild."
    fi
fi

# ---------------------------------------------------------------------------
# Persist cache hash BEFORE building — survives partial failures so retries
# skip the expensive build step when source hasn't changed
# ---------------------------------------------------------------------------
echo "$CURRENT_HASH" > "$CACHE_FILE"

# ---------------------------------------------------------------------------
# Build & export
# ---------------------------------------------------------------------------
cd "$PROJECT_ROOT"

echo "=== Building Docker image for ARM64 ==="
docker buildx build \
    --platform linux/arm64 \
    --tag "$TAG" \
    --load \
    -f Dockerfile \
    .

echo "=== Exporting image to tar ==="
docker save -o "$TAR_FILE" "$TAG"
SIZE=$(du -h "$TAR_FILE" | cut -f1)
echo "Image exported: $TAR_FILE ($SIZE)"

# ---------------------------------------------------------------------------
# Pre-flight: check remote disk space
# ---------------------------------------------------------------------------
echo "=== Checking remote disk space at $DEST_PATH ==="
REMOTE_FREE_KB="$(ssh "$HOST" "df -k \"$DEST_PATH\" | awk 'NR==2{print \$4}'")"
REQUIRED_KB=$(( $(stat -c%s "$TAR_FILE") / 1024 + 51200 ))  # tar size + 50 MB margin

if [ "$REMOTE_FREE_KB" -lt "$REQUIRED_KB" ]; then
    echo "ERROR: Not enough space on remote at $DEST_PATH."
    echo "  Available: $(( REMOTE_FREE_KB / 1024 )) MB"
    echo "  Required:  $(( REQUIRED_KB / 1024 )) MB"
    echo "  Try a path on a larger filesystem, e.g.:"
    echo "    ./deploy.sh $HOST autogrocery:latest /mnt/data"
    exit 1
fi

echo "Remote $DEST_PATH has $(( REMOTE_FREE_KB / 1024 )) MB free — OK."

# ---------------------------------------------------------------------------
# Transfer (with retry)
# ---------------------------------------------------------------------------
MAX_SCP_RETRIES=3
SCP_RETRY=0

while [ "$SCP_RETRY" -lt "$MAX_SCP_RETRIES" ]; do
    SCP_RETRY=$(( SCP_RETRY + 1 ))
    echo "=== Transferring to $HOST (attempt $SCP_RETRY/$MAX_SCP_RETRIES) ==="

    if scp "$TAR_FILE" "$HOST:$DEST_PATH/"; then
        break
    fi

    echo "WARNING: SCP failed. Retrying..."
    sleep 5
done

if [ "$SCP_RETRY" -ge "$MAX_SCP_RETRIES" ]; then
    echo "ERROR: SCP failed after $MAX_SCP_RETRIES attempts."
    exit 1
fi

# ---------------------------------------------------------------------------
# Load on remote (clean up tar immediately)
# ---------------------------------------------------------------------------
echo "=== Loading image on remote ==="
echo "  (this may take a minute — progress streamed from docker)"
ssh -t "$HOST" "docker load -i $DEST_PATH/$TAR_FILE && rm -f $DEST_PATH/$TAR_FILE" || {
    echo "ERROR: docker load failed on remote." >&2
    exit 1
}
echo "  Image loaded and tar cleaned up."

echo "=== Verifying on remote ==="
ssh "$HOST" "docker images | grep '$(echo $TAG | cut -d: -f1)'"

echo ""
echo "Deploy complete! Image '$TAG' is on the remote."
echo ""
echo "Next steps:"
echo "  1. Create config dir: ssh $HOST 'mkdir -p /home/${REMOTE_USER}/autogrocery/config'"
echo "  2. Copy credentials:  scp -r config/* $HOST:/home/${REMOTE_USER}/autogrocery/config/"
echo ""
echo "  3. Set up systemd (will be installed automatically below):"

CONFIG_DIR="/home/${REMOTE_USER}/autogrocery/config"

# Generate systemd files with correct paths
SERVICE_FILE=$(cat <<EOF
[Unit]
Description=autoGrocery ticket scraper
After=network-online.target
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/bin/docker run \
    --rm \
    --name autogrocery-%i \
    -v ${CONFIG_DIR}:/app/config \
    autogrocery:latest
TimeoutStartSec=1800

[Install]
WantedBy=multi-user.target
EOF
)

TIMER_FILE=$(cat <<EOF
[Unit]
Description=Run autoGrocery daily at 3:00 AM

[Timer]
OnCalendar=*-*-* 03:00:00
RandomizedDelaySec=300
Persistent=true
Unit=autogrocery.service

[Install]
WantedBy=timers.target
EOF
)

echo "  Uploading systemd files..."
echo "$SERVICE_FILE" | ssh "$HOST" "sudo tee /etc/systemd/system/autogrocery.service"
echo "$TIMER_FILE" | ssh "$HOST" "sudo tee /etc/systemd/system/autogrocery.timer"

echo "  Reloading systemd and enabling timer..."
ssh "$HOST" "sudo systemctl daemon-reload && sudo systemctl enable --now autogrocery.timer"

echo ""
echo "Done! autoGrocery will run daily at 3:00 AM."
echo "Check status: ssh $HOST 'systemctl status autogrocery.timer'"
