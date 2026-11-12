# Build stage for Go binary
FROM golang:1.26-bookworm AS go-builder

WORKDIR /src/go
COPY go/ .
RUN go mod download
RUN go build -ldflags "-X main.Version=$(git describe --tags --always --dirty)-$(date +%Y-%m-%dT%H:%M:%SZ)" -o /usr/local/bin/autogrocery ./cmd/autogrocery/

# Final stage: Python + Playwright + Go binary (Debian-based for Raspberry Pi OS compatibility)
FROM python:3.12-slim-bookworm

# Install Node.js (for npx) and Firefox's system dependencies
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl \
    && curl -fsSL https://deb.nodesource.com/setup_20.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && apt-get install -y --no-install-recommends \
    xvfb \
    chromium \
    chromium-driver \
    libgtk-3-0 \
    libdbus-1-3 \
    libnss3 \
    libx11-xcb1 \
    libxcomposite1 \
    libxdamage1 \
    libxrandr2 \
    libgbm1 \
    libasound2 \
    libatspi2.0-0 \
    libxshmfence1 \
    libatomic1 \
    && rm -rf /var/lib/apt/lists/*

# Set virtual display and Firefox sandbox settings for headless Docker
ENV DISPLAY=:99
ENV MOZ_ENABLE_WAYLAND=0
ENV MOZ_DISABLE_RDD_SANDBOX=1

# Set Rod cache dir (unused when CHROME_PATH points to system Chromium)
ENV ROD_CACHE_DIR=/root/.cache/rod

# Install Python dependencies and Playwright browser binaries
# invisible-core>=56 dropped ARM64 support; pin <56 for Raspberry Pi (ARM64)
RUN pip install --no-cache-dir playwright "invisible-playwright" "invisible-core<56" && \
    python -m playwright install --with-deps chromium firefox

# Copy Go binary from builder
COPY --from=go-builder /usr/local/bin/autogrocery /usr/local/bin/autogrocery

# Set working directory
WORKDIR /app

# Copy scripts and config
COPY scripts/ /app/scripts/
COPY config/ /app/config/

# Environment variables
ENV HEADLESS=1
ENV CHROME_PATH=/usr/bin/chromium
ENV PYTHONUNBUFFERED=1

# Default command: start Xvfb then run the cookie refresh script
CMD ["sh", "-c", "Xvfb :99 -screen 0 1280x720x24 &>/dev/null & sleep 1 && python /app/scripts/full-flow.py"]
