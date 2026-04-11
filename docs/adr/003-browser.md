# ADR-003: Playwright sidecar, headless-first

## Status
Proposed

## Context
- ZClaw uses a Node.js Playwright-based browser sidecar to render and drive headless browsers for each agent/task.
- The browser driver must be Docker-ready, support multiple browsers, and expose a stable protocol to the control plane for remote control.
- Alternatives include Selenium, Puppeteer, or direct Chrome DevTools Protocol (CDP) access; each carries different degree of headless support, ecosystem, and licensing implications.
- There is a licensing concern with Browserless (a hosted, managed Playwright solution) and similar services; keeping a self-contained Playwright sidecar avoids licensing entanglements while preserving flexibility.
- The density target of 100-300 agents benefits from a headless-first approach with low overhead per browser instance and efficient multiplexing across channels.

## Decision
- Use Playwright as the browser automation framework inside a dedicated sidecar process, run as a lightweight, Docker-ready container or a separate host process on the node.
- Host the sidecar as headless by default, with options to enable visible debugging if needed, but keep normal operation fully headless to minimize resource usage.
- Support all major browsers via Playwright (Chromium, Firefox, WebKit) to maximize compatibility with web apps used by agents.
- Expose a WebSocket-based control channel from the control plane to the sidecar for low-latency remote control and status streaming.
- Keep the sidecar isolated from the core control plane to reduce fault propagation and allow independent scaling of the browser layer.
- Reuse Playwright's testing and automation tooling for reliability, with a lightweight wrapper around the browser pool to avoid per-agent process creation.
- Address licensing concerns by avoiding reliance on third-party hosted services; if Browserless licensing ever becomes necessary, we will switch to an on-premise or self-hosted licensing plan with attribution compliance.

## Consequences
- Richer browser automation capability across 100-300 agents with consistent headless behavior; easier to debug when browser behavior diverges.
- Docker-ready sidecar enables consistent deployment in varied environments and simplified upgrades.
- Multi-browser support reduces the risk of platform-specific render issues; however, it increases the surface area for resource usage and updates.
- A WebSocket API provides low-latency control and streaming, but requires careful back-pressure handling and error propagation in the control plane.
- Licensing constraints are addressed by keeping playright sidecar self-contained and avoiding reliance on external services; if a service is used in the future, attribution and licensing compliance must be maintained.
- Potential risks include sidecar process crashes, memory fragmentation with long-running browser instances, and the need for robust recycling/timeout strategies.

## Alternatives Considered
- Selenium: mature browser automation but heavier footprint and weaker WebSocket-style control; Playwright generally provides better headless support and cross-browser consistency.
- Puppeteer: strong Chrome-centric tooling but limited in cross-browser support compared to Playwright.
- CDP directly: minimal layer could reduce latency but places more complexity on managing browser processes and cross-browser compatibility.
- Browserless-hosted: simplifies orchestration but imposes licensing and usage constraints; self-hosting avoids vendor lock-in.
