FROM node:22-slim

RUN apt-get update && \
    apt-get install -y --no-install-recommends wget && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY browser-worker/package.json ./
RUN npm install --production && \
    npx playwright install --with-deps chromium

COPY browser-worker/src/ ./src/

RUN adduser --disabled-password --gecos "" --uid 1000 browser

USER browser

EXPOSE 9222

HEALTHCHECK --interval=20s --timeout=5s --start-period=30s --retries=3 \
  CMD wget -q --spider http://localhost:9222/health || exit 1

LABEL org.opencontainers.image.title="ZClaw Browser Worker"
LABEL org.opencontainers.image.description="Playwright-based headless browser worker"

CMD ["node", "src/server.js"]
