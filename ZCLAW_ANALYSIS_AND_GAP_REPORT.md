# Zclaw analysis and upgrade gaps

## What Zclaw has already done

Zclaw is not just a README mockup anymore. The repo already contains:

- A Go control plane daemon and CLI (`cmd/dockclawd`, `cmd/dockclawctl`)
- SQLite-backed startup with migrations on boot
- A scheduler with worker-pool settings and heartbeat jitter
- Provider registration behind a provider interface/registry
- A built-in tool registry and HTTP routes for tools
- Agent CRUD, task enqueue, provider list, stats, tool, connection, and sub-agent HTTP routes
- Docker Compose services for a control plane, browser worker, and tool worker
- Operator scripts for install, update, backup, doctor, and scaling
- Health endpoints for both the control plane and the browser worker

## What the code and docs imply

The repo already follows the right high-level direction for a dense server-first fork of OpenClaw:

- agents as rows instead of long-lived per-agent processes
- pooled workers instead of “one runtime per idle agent”
- headless-first browser automation
- Docker-native install and lifecycle
- provider adapters behind a common abstraction

That is the right shape for a high-density platform.

## Biggest gaps between current state and a serious OpenClaw-class product

### 1) Product maturity gap
The repo is still alpha-stage and small. It has a tiny commit history and almost everything important still lives in a compact set of files rather than a battle-tested distributed control plane.

### 2) API-product gap
There is already an internal API surface, but it is not yet a polished public developer platform. What is still missing:

- stable versioned public API contract
- API auth for public/tenant usage
- rate limits and quotas
- frontend-safe session model
- SSE/WebSocket streaming contract for websites
- client SDKs
- embed widget
- webhook signing and replay protection for public apps
- tenant/project/org boundaries

### 3) Browser/runtime hardening gap
The current direction is good, but to become the “internet’s ideal OpenClaw alternative” it needs:

- browser session leasing and queueing
- remote CDP and browser pool abstractions
- stricter SSRF/network policy
- browser state persistence rules
- concurrency limits per tenant/agent/task
- upload/download handling
- structured DOM/a11y extraction
- screenshot fallback only when needed
- deterministic replay and tracing

### 4) Density proof gap
The promise is 100–300 agents per server, but users will want proof:

- benchmark harness for 10/50/100/300 agents
- idle CPU and RAM measurements
- browser-pool saturation tests
- queue latency charts
- failure recovery tests
- workspace cleanup metrics

### 5) Frontend/web assistant gap
This is the largest missing product piece. For websites, the project needs:

- public REST + SSE + WebSocket API
- embeddable chat widget via script or iframe
- session persistence and visitor identity mapping
- domain allowlists and public-widget secrets
- handoff to human/support webhook
- branding/theming
- safe public-tool restrictions
- analytics and abuse controls

### 6) “Most wanted” feature gap
The broader OpenClaw/OpenHands/Goose-style market expects more than provider count. It expects:

- strong auth and RBAC
- multi-user and multi-tenant operation
- memory and retrieval controls
- cron + heartbeat + background task ledger
- browser automation with isolated profiles
- plugin/MCP ecosystem
- observability
- file/workspace APIs
- approvals/guardrails
- agent templates and policies
- voice/media readiness
- web UI and embed surfaces

## Recommended strategy

Do not turn Zclaw into a line-by-line OpenClaw clone.

Instead, make it:

1. Docker-native from the core outward  
2. API-first for websites and external apps  
3. benchmark-first for density claims  
4. headless-browser-first for server deployments  
5. multi-tenant from day one  
6. compatible enough with OpenAI/OpenResponses-style HTTP patterns that frontend developers can integrate quickly  
7. OpenClaw-like in outcome, but simpler operationally and lighter at idle
