# PLAN.md

## Mission

Build a **Docker-native, headless-first, low-overhead, always-on agent platform** inspired by OpenClaw, but optimized for **server deployment**, **provider breadth**, and **high logical-agent density** on a single machine.

The design target is:

- **100–300 logical agents on one server**
- **low idle CPU usage**
- **low storage overhead**
- **no screen required**
- **easy install / update / backup scripts**
- **provider parity with OpenClaw as a minimum baseline**

---

## Product decisions

### Decision 1: do not build a monolith that keeps one full runtime per idle agent
Idle agents are data + schedules + memory pointers.
Workers are pooled or created on demand.

### Decision 2: Go control plane, Node browser sidecar
- Go handles orchestration, scheduling, policy, storage, and Docker interaction.
- Node is reserved for Playwright-based browser automation only.

### Decision 3: SQLite first, Postgres later
Single-node mode must have the lowest operational overhead possible.
Do not require Postgres, Redis, or Kubernetes in v1.

### Decision 4: browser automation must be remote and headless
Never assume a local desktop or physical screen.
All browser interactions must work through a sidecar or remote endpoint.

### Decision 5: provider breadth is table stakes
The architecture must support parity with OpenClaw's documented providers, plus extra adapters, without rewriting the core.

---

## Non-negotiable requirements

1. Everything runs under Docker Compose.
2. A fresh machine can be installed with a small script set.
3. The platform can run indefinitely with restart policies and health checks.
4. Browser work must function on a server without GUI.
5. Idle agents must consume near-zero CPU.
6. The system must avoid one heavyweight container per configured agent.
7. Provider adapters must be pluggable.
8. Secrets and per-agent policies must be isolated.

---

## Success metrics

These are engineering targets, not promises.
Adjust after measurement.

### Density targets
- 100 configured agents should be routine on modest hardware.
- 300 configured agents should be achievable on a stronger single-node server.
- Idle agents should not each require a dedicated container.

### Idle footprint targets
- Control plane remains the main always-on service.
- No browser process exists unless at least one job needs it.
- Heartbeats should be staggered and event-driven to prevent synchronized spikes.

### Operational targets
- fresh install in under 10 minutes on a clean Linux server
- update with one script
- backup with one script
- self-check / doctor command available
- broken worker auto-recovery without full system restart

---

## Delivery phases

## Phase 0 — Architecture lock

### Goal
Decide the runtime shape before coding too much.

### Deliverables
- architecture ADRs
- provider adapter interface spec
- browser worker interface spec
- agent state model
- storage schema draft
- queue/scheduler design
- security boundary definition

### Acceptance criteria
- written ADR for control plane language
- written ADR for storage backend
- written ADR for browser strategy
- written ADR for worker lifecycle
- written ADR for licensing and code-reuse policy

### Key outputs
- `docs/adr/001-control-plane.md`
- `docs/adr/002-storage.md`
- `docs/adr/003-browser.md`
- `docs/adr/004-workers.md`
- `docs/adr/005-licensing.md`

---

## Phase 1 — Skeleton and operator experience

### Goal
Make the repo runnable with minimal operator friction.

### Deliverables
- Docker Compose stack
- `install.sh`
- `update.sh`
- `doctor.sh`
- `backup.sh`
- `.env.example`
- health check endpoints
- CLI admin tool (`dockclawctl`)

### Acceptance criteria
- `./scripts/install.sh` brings up the stack on a supported Linux host
- `docker compose ps` shows healthy services
- `./scripts/doctor.sh` can validate Docker, disk path, ports, and required env vars
- `./scripts/backup.sh` exports DB + artifacts

### Notes
Start with the smallest useful service set:

- control plane
- browser worker
- reverse proxy optional

Do not add Postgres/Redis yet.

---

## Phase 2 — Agent core

### Goal
Implement logical agents without expensive per-agent runtime overhead.

### Deliverables
- agent registry
- agent create/update/delete
- per-agent config
- per-agent schedules
- per-agent quotas
- per-agent workspace paths
- task queue and execution states

### Acceptance criteria
- create 100 agents without creating 100 containers
- agents persist across restart
- agents can be enabled/disabled independently
- agent state is inspectable via CLI/API
- queue can hold pending work per agent

### Design rules
- agents are rows, not daemons
- agent identity is separate from execution context
- execution context is attached only when needed

---

## Phase 3 — Scheduler and heartbeat

### Goal
Implement 24/7 behavior without turning the machine into a heater.

### Deliverables
- heartbeat engine
- cron schedules
- event-driven wakeups
- jitter support
- active-hours windows
- retry/backoff policies
- per-agent heartbeat prompt files

### Acceptance criteria
- 100 agents can have heartbeat schedules without synchronized CPU spikes
- task completion can wake the parent agent without waiting for the next global tick
- agents can disable or narrow heartbeat windows
- low-priority maintenance turns use cheap models by default

### Rules
- heartbeat is **event-driven first**
- use periodic sweeps only as a fallback and safety net
- dedupe repeated wake triggers

---

## Phase 4 — Provider system

### Goal
Ship a clean provider layer that matches OpenClaw-style breadth but is easier to extend.

### Deliverables
- provider registry
- provider interface
- auth abstraction
- model alias normalization
- fallback chain support
- capability flags
- rate-limit/backoff hooks

### Initial provider set
Ship these first:
- OpenAI
- Anthropic
- OpenRouter
- Ollama
- LiteLLM-compatible generic adapter

### Expansion target
Add adapters toward parity with the OpenClaw-documented provider list:
- Alibaba Model Studio
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
- OpenCode
- Qianfan
- Qwen
- Runway
- StepFun
- Synthetic
- Vercel AI Gateway
- Venice
- xAI
- Z.AI

### Acceptance criteria
- providers can be registered without editing scheduler or agent core
- provider config validates cleanly
- model selection supports aliases and defaults
- tool capability differences are encoded in metadata, not hardcoded all over the app

---

## Phase 5 — Tool-worker sandbox

### Goal
Allow safe command/file execution without exposing the host recklessly.

### Deliverables
- tool-worker image
- on-demand worker launch
- workspace mount strategy
- read-only vs read-write modes
- resource caps
- timeout policy
- execution audit logging

### Acceptance criteria
- worker container can run commands in an isolated workspace
- container dies and cleans up correctly after task completion
- runaway tasks are killed by timeout
- per-agent policy can disable shell execution entirely

### Resource strategy
- reuse shared base image
- keep writable layers small
- use on-demand workers, not permanent ones

---

## Phase 6 — Headless browser worker

### Goal
Provide browser automation on machines with no screen.

### Default implementation
- Playwright-based browser worker sidecar
- remote WebSocket control
- screenshot, DOM snapshot, navigation, input, downloads

### Optional later adapter
- Browserless-compatible endpoint if licensing is approved

### Deliverables
- `browser-worker` image
- browser session manager
- concurrency limits
- tab/session cleanup
- artifact retention policy
- observer mode for debugging

### Acceptance criteria
- browser tasks work on a server with no monitor or local desktop
- no full browser is kept running when idle unless configured
- stale sessions are reaped automatically
- browser concurrency is capped to avoid memory explosions

### Key design rule
Do not bind one persistent browser to each agent.
Use pooled sessions and strict quotas.

---

## Phase 7 — Memory, artifacts, and retention

### Goal
Keep context useful without letting storage grow forever.

### Deliverables
- conversation store
- summary store
- artifact store
- retention policies
- compaction jobs
- export/import tools

### Acceptance criteria
- large histories are summarized automatically
- artifacts can be pruned by age and policy
- per-agent retention is configurable
- backup and restore include critical agent state

### Rules
- summaries beat raw transcript bloat
- compress and prune aggressively
- keep audit trails useful, not infinite

---

## Phase 8 — Fleet management UX

### Goal
Make the system practical for real operators.

### Deliverables
- CLI fleet commands
- agent templates
- policy templates
- usage dashboards
- provider health view
- queue depth view
- worker pool status view

### Acceptance criteria
- operator can see which agents are active, sleeping, blocked, or failing
- operator can restart or pause individual agents
- operator can inspect provider failures and browser backlog

---

## Phase 9 — Hardening and density tuning

### Goal
Turn the product from “works” into “runs many agents well.”

### Deliverables
- benchmark harness
- load-test scenarios
- density profile configs
- startup latency tracking
- idle CPU measurement
- memory profile reports
- image size audit

### Acceptance criteria
- benchmark report exists for 10 / 50 / 100 / 300 agents
- image sizes are documented and reduced where possible
- idle CPU behavior is measured, not guessed
- browser saturation failure mode is graceful
- queue backpressure works under stress

---

## Implementation guardrails

### Guardrail 1: no unnecessary infra
Do not add Redis, Kafka, Postgres, NATS, Temporal, or Kubernetes in v1 unless a measured bottleneck forces it.

### Guardrail 2: no one-container-per-agent design
That would directly violate the density target.

### Guardrail 3: no desktop assumptions
No feature may require a local display.

### Guardrail 4: no giant base images by default
Keep control-plane and worker images separate.

### Guardrail 5: no provider logic scattered through the codebase
Provider behavior must live behind adapter boundaries.

### Guardrail 6: no permanent browsers without need
Browser processes should be lazy, pooled, and capped.

---

## Licensing and code reuse policy

### Allowed with proper compliance review
- MIT projects
- Apache-2.0 projects

### Use as inspiration first
- OpenClaw
- OpenHands
- Goose
- GoGogot
- AnythingLLM

### Special caution
- Browserless licensing must be reviewed before direct code reuse.

### Hard rule
Do not paste code into the repo just because it is convenient.
Track provenance for every borrowed file or substantial snippet.

---

## Easy-install script plan

### `scripts/install.sh`
Responsibilities:
- check Docker Engine and Compose v2
- create directories
- copy `.env.example` to `.env` if absent
- prompt for required provider secrets or skip
- pull/build images
- start stack
- print next steps

### `scripts/update.sh`
Responsibilities:
- backup current state
- pull new images
- run migrations
- recreate changed services
- verify health

### `scripts/doctor.sh`
Responsibilities:
- check Docker health
- check disk free space
- check port conflicts
- check required mounts
- check env file completeness
- check DB availability
- check browser worker availability

### `scripts/backup.sh`
Responsibilities:
- snapshot SQLite DB
- tar artifacts and configs
- emit timestamped backup archive

### `scripts/scale-agents.sh`
Responsibilities:
- generate test agents
- apply templates
- simulate schedules
- help benchmark density

---

## Performance tactics to bake in early

1. Use one control-plane process.
2. Use bounded worker pools.
3. Use shared image layers.
4. Use SQLite WAL and prepared statements.
5. Summarize old context.
6. Use cheap utility models for maintenance turns.
7. Jitter scheduled wakeups.
8. Reuse HTTP clients and keep-alive connections.
9. Cap browser concurrency.
10. Avoid indexing everything by default.

---

## Risks

### Risk: browser memory blowups
Mitigation:
- session limits
- browser pool caps
- aggressive cleanup
- disable browser by default on agents that do not need it

### Risk: provider sprawl makes the code messy
Mitigation:
- strict adapter interface
- provider capability descriptors
- shared transport/auth helpers

### Risk: 300 agents all waking together
Mitigation:
- jitter
- active-hours windows
- event-driven wakeups
- per-agent priority classes

### Risk: Docker image sprawl consumes disk
Mitigation:
- shared base images
- separate small control-plane image
- image pruning guidance
- artifact retention and cleanup jobs

### Risk: system becomes impossible to operate
Mitigation:
- ship install/update/doctor/backup scripts first
- keep single-node mode simple

---

## Definition of done for v1

V1 is done when all of this is true:

1. A clean Linux server can install the stack with the provided scripts.
2. Agents can be created and scheduled without creating a full runtime per idle agent.
3. Browser automation works headlessly through Docker.
4. At least five major providers are production-ready and the provider adapter system is stable.
5. The architecture clearly supports expansion to OpenClaw-level provider breadth.
6. The system can stay online 24/7 with health checks and restart policies.
7. Benchmark results for 100+ logical agents exist.

---

## Immediate next actions

1. Write ADRs.
2. Scaffold the Go control plane.
3. Add Compose stack and scripts.
4. Implement agent registry and scheduler.
5. Add provider abstraction.
6. Add tool-worker.
7. Add Playwright browser-worker.
8. Run density benchmarks.
