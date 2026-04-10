# ADR-003: Playwright Sidecar for Headless Browser

## Status
Accepted

## Context
ZClaw agents need browser automation for web research, screenshots, form filling, and data extraction. The target deployment is a headless Linux server with no monitor, no X11, and no desktop environment. Browser sessions must be pooled, capped, and cleaned up aggressively to prevent memory blowups.

## Decision
Use a Playwright-based browser worker as a Node.js sidecar container, controlled via HTTP/WebSocket from the Go control plane.

Playwright provides:
- First-class headless support with official Docker images
- Multi-browser support (Chromium, Firefox, WebKit)
- Remote connection via WebSocket
- Screenshot, DOM snapshot, navigation, input, and file download APIs
- Mature Node.js API surface that is difficult to replicate in Go

The browser worker runs as a separate container (`browser-worker`) that the control plane communicates with over HTTP. Sessions are pooled and capped with strict timeouts. No browser process exists unless at least one task needs it.

## Consequences
- Browser work functions on any Docker host, regardless of display
- Browser sessions are lazy (started on demand) and capped (max concurrent sessions)
- Node.js is required only inside the browser-worker container image
- Browserless is not used directly due to licensing complexity; its session management patterns are studied as inspiration

## Alternatives Considered
- **Selenium**: Mature but heavier, worse headless ergonomics, and no official Docker-first guidance matching Playwright.
- **Puppeteer**: Good for Chromium-only use cases, but Playwright offers multi-browser and better Docker support.
- **Raw CDP**: Maximum control but significant implementation effort with no library safety net.
- **Browserless**: Strong session management and pooling, but OSS/commercial licensing requires review before code reuse.
