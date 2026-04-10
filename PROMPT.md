# PROMPT.md

You are the principal engineer and system architect for this repository.
Your job is to build a **Docker-first, headless-first, low-resource, always-on OpenClaw-inspired agent platform**.

The working title is **ZClaw**.
You may rename it later, but keep all architecture decisions aligned with the constraints below.

---

## Mission

Build a server-native AI agent system that can run **100–300 logical agents on a single machine** using Docker while keeping **idle CPU**, **RAM growth**, and **disk usage** under control.

This product must feel like a better fit than OpenClaw for this specific use case:

- headless servers
- no local screen
- no desktop assumptions
- multiple long-lived agents
- low operational overhead
- easy install and update
- broad provider support

The platform should stay online 24/7, support heartbeat-driven follow-up behavior, and use a remote headless browser approach whenever web interaction is needed.

---

## Hard requirements

### Deployment
- The system must be **100% Docker-oriented**.
- Docker Compose must be the default deployment method.
- The repo must include easy operator scripts:
  - `scripts/install.sh`
  - `scripts/update.sh`
  - `scripts/doctor.sh`
  - `scripts/backup.sh`
- No mandatory host-level Node/Python app install path.
- The target host may have **no GUI and no monitor**.

### Runtime shape
- Do **not** create one heavyweight container per idle agent.
- Agents must be **logical entities** first: config, schedules, memory pointers, queue state, and policies.
- Actual execution environments must be **pooled** or **spawned on demand**.
- Browser sessions must be **shared/pool-based**, not permanently dedicated to every agent.

### Performance
- Optimize for **low idle CPU** and **low storage footprint**.
- Use shared container layers and minimal writable state.
- Avoid always-on heavyweight services unless proven necessary.
- Event-driven wakeups must be preferred over frequent polling loops.

### Availability
- Agents should behave like a 24/7 service.
- The platform must recover cleanly from restarts.
- Health checks and restart policies must be built in.
- Heartbeat behavior must exist, but it should be implemented efficiently.

### Browser automation
- Browser work must function on a machine with no screen.
- The default browser implementation should be a **remote Playwright-based browser sidecar**.
- Support screenshots, DOM snapshots, navigation, input, downloads, and session cleanup.
- Live visual observer/debug mode can exist, but it must be optional.

### Providers
- Provider breadth must reach **OpenClaw-level parity as a baseline**, then go beyond it.
- Build a provider adapter/plugin system so new providers can be added without core rewrites.
- Support hosted APIs and local/OpenAI-compatible endpoints.

---

## Explicit product interpretation

Do **not** build a shallow fork that merely renames OpenClaw.
Instead, build a cleaner architecture specifically optimized for this model:

> many configured agents, few active workers

That means:
- one main control plane
- lazy execution runtimes
- bounded worker pools
- remote headless browser service
- strong quotas and policies
- compact storage defaults

---

## Recommended architecture

### Control plane
Implement the core as a **Go service**.

It should own:
- agent registry
- provider registry
- schedules and heartbeats
- task queue orchestration
- policy enforcement
- memory references
- Docker orchestration
- health endpoints
- audit logs

### Storage
Use **SQLite in WAL mode** as the default single-node storage backend.

Store:
- agents
- schedules
- queue state
- provider config metadata
- artifact metadata
- audit records
- summaries

Use the filesystem for:
- workspaces
- artifacts
- downloads
- logs
- backups

Do not require Postgres or Redis in v1.
Only add optional scale-out backends later.

### Workers
Split workers by concern:

1. `tool-worker`
   - shell commands
   - repo operations
   - file changes
   - dependency installs
   - code execution

2. `browser-worker`
   - Playwright-based
   - remote WebSocket control
   - browser pooling
   - screenshot and snapshot support
   - strict cleanup

3. `provider-gateway` only if a provider truly needs a dedicated sidecar
   - otherwise, keep providers in-process via adapters

### Scheduler
Implement a scheduler that supports:
- heartbeat intervals
- cron jobs
- event-driven wakeups
- active-hours windows
- backoff and retry
- per-agent priorities
- jitter to avoid synchronized spikes

---

## Architectural rules

### Rule 1
Idle agents are cheap.
If 200 agents are configured and only 5 are active, the system should behave like roughly 5 active execution contexts, not 200.

### Rule 2
No per-agent permanent browser by default.
Browser work must go through pooled sessions with quotas.

### Rule 3
No provider-specific code scattered throughout the control plane.
All provider differences must sit behind adapters.

### Rule 4
No desktop-first assumptions.
Every feature must be usable on a headless Linux server.

### Rule 5
No unnecessary infrastructure in v1.
Avoid Redis, Kafka, Temporal, Kubernetes, and Postgres unless benchmarks force them.

### Rule 6
Do not let heartbeat become a resource tax.
Use event-driven design and lightweight models for maintenance turns.

---

## Provider baseline

Design the system so it can match or exceed this baseline provider set:

- Alibaba Model Studio
- Anthropic
- Amazon Bedrock
- BytePlus
- Chutes
- ComfyUI
- Cloudflare AI Gateway
- fal
- Fireworks
- GLM
- MiniMax
- Mistral
- Moonshot AI
- OpenAI
- OpenCode
- OpenRouter
- Qianfan
- Qwen
- Runway
- StepFun
- Synthetic
- Vercel AI Gateway
- Venice
- xAI
- Z.AI

Also leave room for:
- Ollama
- LiteLLM-compatible servers
- LocalAI
- LM Studio
- Groq
- Gemini-compatible adapters
- custom enterprise gateways

### Provider adapter contract
Every provider adapter should define:
- id
- auth modes
- model normalization
- capability flags
- streaming support
- tool-calling support
- vision/audio support
- retry/backoff hints
- rate-limit hints
- cost class

---

## Browser strategy

### Default
Use a **Playwright sidecar** container as the default browser implementation.

Why:
- good browser fidelity
- good remote-control ergonomics
- works headlessly
- easy to isolate in Docker

### Optional later
Support a browserless-style adapter if licensing and product goals allow it.
Do not copy code from Browserless blindly.

### Browser requirements
- pool sessions
- cap concurrency
- enforce timeouts
- clean stale sessions automatically
- store screenshots/downloads under artifact retention policies
- support debug observer mode without making it mandatory

---

## Heartbeat behavior

Implement heartbeat as a low-cost system.

### Goals
- agents should periodically reconsider pending work
- task completions should wake the parent agent quickly
- the system should avoid global synchronized wakeups

### Preferred strategy
1. event-driven wakeup first
2. cron/heartbeat second
3. jitter all periodic work
4. cheap utility model by default
5. premium model only when escalation is necessary

---

## Security and isolation requirements

- use least-privilege containers where practical
- separate browser worker and tool worker
- do not expose the Docker socket to arbitrary agent code
- enforce per-agent filesystem boundaries
- allow read-only and read-write execution policies
- support per-agent browser enable/disable
- support network restrictions where practical
- keep an audit trail for tool and browser actions

---

## Easy-install requirements

Implement these scripts early, not at the end.

### `scripts/install.sh`
Must:
- validate Docker Engine and Compose v2
- create directories
- initialize `.env`
- pull/build images
- start the stack
- print operator next steps

### `scripts/update.sh`
Must:
- create a backup first
- update images
- run migrations
- restart safely
- verify health

### `scripts/doctor.sh`
Must:
- inspect ports
- inspect env vars
- inspect disk space
- inspect DB health
- inspect browser worker reachability
- inspect Docker availability

### `scripts/backup.sh`
Must:
- export SQLite DB safely
- archive artifacts and configs
- emit timestamped backup files

---

## Suggested implementation order

1. Create ADRs for architecture decisions.
2. Scaffold the Go control plane.
3. Add Docker Compose and operator scripts.
4. Implement agent registry and scheduler.
5. Implement provider adapter framework.
6. Add first providers: OpenAI, Anthropic, OpenRouter, Ollama, LiteLLM-compatible.
7. Add tool-worker sandbox.
8. Add Playwright browser-worker.
9. Add quotas, retention, and cleanup.
10. Benchmark 10, 50, 100, and 300 logical agents.

---

## Code reuse and inspiration policy

You may study and selectively reuse ideas from these projects when license-compatible and clearly attributed:

- OpenClaw
- OpenHands
- Goose
- GoGogot
- AnythingLLM

### Rules
- Prefer architectural inspiration over copy-paste reuse.
- Track provenance for any reused file or substantial code block.
- Preserve notices and license requirements.
- Do not import code from incompatible or unclear licenses without explicit review.
- Treat Browserless code reuse as restricted unless a license review says otherwise.

---

## What not to build

Do not build:
- a desktop-first app
- a GUI-dependent browser flow
- a monolith with many always-on subservices by default
- one full runtime per configured idle agent
- a system that requires Kubernetes in v1
- a clone that only copies OpenClaw naming and folder structure without fixing the density problem

---

## Definition of success

Your implementation is moving in the right direction if:

- Docker Compose is the natural install path
- headless browser work functions on a server without a screen
- idle agents stay cheap
- worker pools are bounded and reusable
- provider adapters are easy to extend
- benchmark results show sane behavior as agent count rises
- the repo is easier to install and operate than a typical local-desktop-first agent framework

---

## Output expectations while working

Whenever you complete a major task, update the repo with:
- code
- docs
- migrations if needed
- script updates if needed
- tests or benchmark fixtures if possible

When proposing architecture changes:
- explain the tradeoff
- keep the density target in mind
- prefer simpler operations over theoretical elegance

When uncertain:
- choose the option with lower idle overhead, fewer required services, and cleaner Docker operation

---

## Final instruction

Build the product for **headless servers and many agents**, not for a developer laptop demo.
If a decision helps “one agent locally” but hurts “200 agents on one box,” reject it.
