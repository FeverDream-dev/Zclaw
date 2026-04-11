# Zclaw master upgrade prompt

You are the lead architect, product manager, and principal implementation engineer for **Zclaw**.

Your mission is to transform the current repository into the **best Docker-native, headless-first, low-overhead, OpenClaw-class self-hosted AI agent platform**, optimized for running **100 to 300 agents on a single server** with **very low idle CPU and low disk/RAM overhead**, while also adding a **public frontend API + embeddable website assistant widget**.

Do not produce a shallow wishlist. Produce an implementation-grade upgrade plan and then apply it across the repository structure, code, docs, scripts, APIs, schemas, tests, and Docker setup.

## Product direction

Zclaw must become:

- **Docker-native by default**
- **Headless-first**
- **Server-first**
- **Multi-agent and high-density**
- **Multi-provider**
- **Public-API-ready**
- **Website-embed-ready**
- **Multi-tenant and secure**
- **Operationally simple**
- **Benchmarkable and measurable**
- **Lighter at idle than OpenClaw-style desktop-first systems**

This is **not** a desktop-first app with Docker added later.
This is **not** one container or one browser per idle agent.
This is **not** a toy local AI shell.
This is a long-running agent gateway/control plane for real server use.

---

## Current repository facts you must respect

The current repository already has:

- a Go control plane daemon
- a Go CLI admin tool
- SQLite-backed startup and migrations
- a provider interface and registry
- provider registration from environment variables
- scheduler startup with worker pool size and heartbeat jitter
- built-in tools and HTTP tool routes
- agent/task/provider/stats/tool/connection/sub-agent API routes
- Docker Compose services for control-plane, browser-worker, and tool-worker
- operator scripts like install, doctor, update, backup, scale
- health endpoints
- an architecture centered on “agents are rows, workers are pooled”

You must **upgrade the existing direction**, not replace it with something that destroys the density-first idea.

---

## North-star requirements

### Core platform requirements

1. Zclaw must support **100 to 300 configured agents on a single machine** without requiring 100 to 300 live runtimes.
2. Idle agents must consume **near-zero CPU** and minimal memory.
3. Heavy resources must be **pooled and leased** only while work is active.
4. The platform must run **24/7** on a server with **no monitor and no GUI requirement**.
5. Browser-dependent tasks must work in a **headless server environment**.
6. The system must expose a **stable API** for external apps and websites.
7. The system must include a **frontend-safe public API**, **SSE/WebSocket streaming**, and an **embeddable website assistant widget**.
8. The platform must support **OpenClaw-level provider breadth plus more**, with a strong adapter contract.
9. The platform must support **heartbeat**, **scheduled jobs**, and **background task tracking** as distinct concepts.
10. The platform must be secure by default, benchmarked, observable, and easy to install.

---

## Design philosophy

Use these principles everywhere:

- **Agents are durable records, not durable processes**
- **Execution is leased, not permanently allocated**
- **Headless browser is a shared service, not per-agent bloat**
- **Public APIs are first-class product surfaces**
- **Operational simplicity beats cleverness**
- **Default-deny security**
- **Strict interface boundaries**
- **Benchmark every density claim**
- **Prefer boring reliable infrastructure**
- **Keep the core fast, keep integrations pluggable**

---

## Mandatory architecture decisions

You must keep or evolve the architecture toward:

### 1) Control plane
A long-lived Go service responsible for:

- agent registry
- task intake
- session routing
- provider routing
- policy enforcement
- queueing
- leases
- heartbeat orchestration
- cron orchestration
- background task ledger
- telemetry
- audit logging
- tenant/project/org boundaries
- frontend/public API auth and rate limits

### 2) Storage
Start with SQLite WAL for single-node installs, but design clean backend interfaces so Postgres can be added later.

Must separate:

- agents
- sessions
- messages
- tasks
- task events
- schedules
- heartbeat state
- provider configs
- widget/public API configs
- tenants/projects
- browser leases
- workspaces
- artifacts
- audits
- metrics snapshots

### 3) Worker model
Do **not** create one permanent worker per agent.

Use:

- pooled tool workers
- pooled browser workers
- queue-driven execution leases
- TTL-based cleanup
- capped concurrency
- per-task resource budgets
- automatic reclamation

### 4) Browser model
Implement a browser subsystem for **server-only deployments**:

- headless Chromium/Chrome/Playwright control
- shared browser pool
- browser session leasing
- optional persistent browser contexts
- strict concurrency controls
- domain/network policy enforcement
- SSRF protections
- upload/download support
- storage/cookie persistence by policy
- DOM/a11y extraction first
- screenshots only when needed
- structured snapshots
- replay traces

Also support a remote/browserless-style mode as an optional backend.

### 5) Provider model
Keep a provider interface with clean capability flags. Support:

- chat/completions
- responses-style usage
- streaming
- tool calling
- vision
- embeddings where relevant
- model introspection
- rate-limit/backoff handling
- retry classification
- token/cost accounting
- provider health status

### 6) Public API + frontend API
Create a production-ready API surface that can be used by:

- your own dashboard UI
- external admin tools
- websites embedding an assistant
- customer portals
- mobile/web apps
- automation/webhooks

Must include:

- versioned REST API (`/api/v1/...`)
- public assistant API (`/public/v1/...`)
- SSE streaming responses
- optional WebSocket real-time channel
- OpenAI/OpenResponses-compatible ingress where practical
- session resumption
- visitor/user identity mapping
- tenant/project scoping
- API keys, scoped tokens, expiring tokens
- rate limits and quotas
- idempotency keys for writes
- webhook callbacks with signing

### 7) Embed widget / website assistant
Create a website-facing assistant product with:

- embeddable `<script>` widget
- iframe fallback
- theme customization
- launcher button
- floating chat bubble
- inline mode
- per-site allowed origins
- public conversation sessions
- anonymous visitor or signed-user mapping
- typing/streaming UI
- upload support
- knowledge/tool restrictions
- escalation hooks
- transcript export hooks
- analytics events
- abuse controls
- domain verification

---

## Feature scope to add

You must add or design for **all of these categories**.

### A. Agent lifecycle
- create/update/delete agents
- enable/disable/pause/resume
- templates
- policies
- tags
- grouping
- cloning
- import/export
- soft delete / archive
- versions / snapshots
- parent/child agents
- specialist subagents
- agent handoff rules

### B. Sessions and memory
- durable sessions
- session routing keys
- visitor/user/session separation
- message history retention policies
- compaction/summarization
- memory search hooks
- ephemeral vs persistent context
- workspace-context injection policies
- lazy/selective workspace injection
- conversation summary generation
- artifact references
- memory TTL / cleanup jobs

### C. Tasking model
Define and keep separate:

1. **Interactive turns**  
2. **Heartbeat turns**  
3. **Scheduled jobs / cron**  
4. **Background detached tasks**  
5. **Sub-agent jobs**  
6. **Webhook-triggered tasks**  

Each needs correct persistence, state tracking, and delivery semantics.

### D. Heartbeat
Implement heartbeat as a first-class feature:

- per-agent interval
- jitter
- active-hours windows
- max concurrent heartbeats
- backpressure awareness
- “nothing to do” short-circuit
- heartbeat-specific prompt channel
- audit trail
- disable when overloaded
- tenant quotas

### E. Scheduler / cron
Add:

- recurring schedules
- one-shot jobs
- jitter
- timezones
- active hours
- blackout windows
- retry policy
- dead-letter queue
- delivery routing
- webhook delivery
- channel delivery
- internal-only task mode
- dedupe protections across restart

### F. Background task ledger
Every detached unit of work must have:

- task id
- parent id
- agent id
- tenant/project/session linkage
- state transitions
- timestamps
- retries
- outputs
- artifacts
- audit trail
- failure reason
- billing/usage attribution

### G. Workspaces
Add serious workspace support:

- per-agent workspaces
- ephemeral workspaces
- per-task temp workspaces
- read-only and read-write modes
- cleanup policies
- quotas
- artifact indexing
- archive/restore
- workspace file API
- remote/container-safe file operations

### H. Tool system
Upgrade tools to support:

- manifest/spec schema
- capability metadata
- input validation
- output envelopes
- timeouts
- approvals
- network policy
- audit trail
- sandbox requirements
- cost hints
- sync + async execution
- streaming outputs where appropriate
- tool categories
- tenant allow/deny lists
- public-safe tool subsets

### I. MCP / plugins / extensions
Support:

- MCP clients
- MCP server exposure
- plugin registry
- dynamic loading where safe
- bundled integrations
- capability discovery
- permission model
- versioned plugin manifests
- extension isolation
- health checks
- lifecycle hooks

### J. Providers
Achieve parity with current provider breadth goals and go beyond.
Support at least:

- OpenAI
- Anthropic
- OpenRouter
- Ollama
- Ollama Cloud
- LiteLLM
- Groq
- Gemini
- Mistral
- xAI
- AWS Bedrock
- Alibaba Qwen
- Fireworks
- Cloudflare Workers AI
- Zhipu / Z.AI
- DeepSeek
- Together AI
- MiniMax
- Moonshot / Kimi
- Vertex AI

Also design simple addition of new providers via one adapter file or small adapter package.

### K. Browser automation
Support:

- pooled headless browsers
- Playwright-based actions
- structured accessibility snapshots
- screenshots
- PDFs
- downloads/uploads
- cookies/storage persistence
- multiple logical profiles
- attach to remote CDP/browserless backends
- browser task queue
- browser session limit per tenant
- browser activity audit
- strict allowlist/denylist
- SSRF protections
- remote endpoint health tracking

### L. Auth / security / tenancy
Implement:

- operator auth
- tenant/project/org auth
- public widget tokens
- API keys
- scoped tokens
- per-origin restrictions
- RBAC
- audit logging
- secrets management surface
- per-agent tool policy
- per-tenant quotas
- network restrictions
- read-only filesystem modes
- CPU/memory/time limits
- safe defaults
- no direct Docker socket exposure to agent code
- signed webhooks
- replay defense
- CSRF/CORS/origin validation for public surfaces

### M. Frontend dashboard
Build a clean web dashboard for operators:

- login
- tenant/project switcher
- agent list
- agent detail
- task queue
- schedules
- heartbeat status
- browser pool status
- worker pool status
- provider health
- metrics
- logs/audit
- artifacts/workspaces
- widget management
- API key management
- domain/origin allowlists
- usage charts

### N. Public website assistant product
This is mandatory.

Create a full “assistant for websites” product surface with:

- public assistant definitions
- widget profiles
- site/domain allowlists
- JWT or signed-session support
- anonymous and authenticated visitors
- brand colors/logo
- intro prompts
- suggested actions
- streaming answers
- attachment upload
- safe public tools
- knowledge base hooks
- lead capture hooks
- handoff to human
- webhook integrations
- moderation and abuse controls
- analytics events
- transcript persistence
- session restore
- GDPR/export/delete hooks

### O. Realtime / transport
Support:

- REST
- SSE
- WebSocket
- webhook callbacks
- optional polling fallback

### P. Observability
Implement proper observability:

- health endpoints
- readiness/liveness
- structured logs
- metrics endpoint
- queue depth
- worker utilization
- browser utilization
- provider latency/error rates
- per-tenant usage
- cost/token estimates
- tracing hooks
- benchmark mode
- slow task logging
- leak detection

### Q. Benchmarks and density proof
Create a benchmark harness and documented metrics for:

- 10 agents
- 50 agents
- 100 agents
- 300 agents

Measure:

- idle CPU
- idle RAM
- task latency
- queue latency
- browser pool utilization
- provider latency
- disk growth
- cleanup behavior
- crash recovery
- restart recovery

The system must publish reproducible benchmark scripts and methodology.

### R. Install and ops scripts
Make ops dead-simple:

- `install.sh`
- `update.sh`
- `doctor.sh`
- `backup.sh`
- `restore.sh`
- `benchmark.sh`
- `scale-agents.sh`
- `rotate-logs.sh`

Each script must be safe, idempotent where possible, and production-friendly.

### S. Docker quality
Use Docker as a first-class product:

- minimal images
- multi-stage builds
- optional distroless where practical
- rootless where possible
- pinned versions
- health checks
- resource limits
- clean volumes
- explicit networks
- profiles/dev overrides
- image size reduction
- startup ordering
- graceful shutdown
- upgrade-safe migrations

### T. Documentation
Ship complete docs for:

- quick start
- architecture
- provider setup
- public API
- frontend widget
- auth and tenancy
- browser subsystem
- benchmarks
- production hardening
- restore/disaster recovery
- troubleshooting
- extension development
- contribution guide

---

## Frontend API requirements in detail

You must add a **frontend-safe API** so customers can place Zclaw on websites.

### Public assistant API
Design endpoints like:

- `POST /public/v1/chat/sessions`
- `POST /public/v1/chat/messages`
- `GET /public/v1/chat/sessions/{id}`
- `GET /public/v1/chat/sessions/{id}/stream`
- `POST /public/v1/chat/uploads`
- `POST /public/v1/widget/verify-origin`
- `POST /public/v1/events`

### Requirements
- browser-safe auth
- short-lived signed tokens
- anonymous visitor mode
- authenticated-user mode
- SSE token streaming
- resumable conversations
- per-widget config
- per-origin enforcement
- per-tenant quotas
- moderation hooks
- upload size and type policy
- abuse/rate limiting
- optional OpenResponses-like request format
- clear SDK contract

### JavaScript SDK
Ship:

- `@zclaw/web-sdk`
- `@zclaw/embed-widget`

SDK must support:

- init
- open/close
- send message
- stream response
- restore session
- set visitor metadata
- upload file
- receive events
- theme config
- custom transport hooks

### Embed widget
Must support:

- `<script>` embed
- iframe embed
- floating bubble
- inline mode
- theme overrides
- CSS variables
- event hooks
- accessibility
- mobile responsive layout
- cookie/session storage options
- domain restrictions

---

## Competitive positioning requirements

Borrow **ideas**, not complexity bloat.

Take inspiration from:
- OpenClaw for multi-channel, heartbeat, sessions, tools, API compatibility
- OpenHands for Docker sandbox discipline
- Goose for MCP/extensions/headless automation patterns
- AnythingLLM for embed/widget and operator UX ideas
- Browserless / remote CDP ecosystems for remote browser pooling patterns
- Playwright MCP / structured browser interaction concepts for accessibility-first browser control

But do not cargo-cult their architecture.
Zclaw must remain lighter, simpler, and more density-focused.

---

## What “internet most wants” should mean in this project

Interpret this as the combination of:

- multi-provider support
- stable APIs
- streaming responses
- web/website embedding
- multi-user and multi-tenant support
- better browser automation
- lower CPU and RAM usage
- durable task history
- heartbeats + schedules
- workspace and file APIs
- better observability
- easy install
- safe defaults
- public integrations
- dashboard UI
- extension ecosystem
- voice/media readiness
- benchmarked performance
- reliable remote/server deployment

You must implement or plan for all of these.

---

## Hard non-functional requirements

### Performance
- very low idle CPU
- bounded concurrency
- browser pool cap
- worker pool cap
- no busy loops
- event-driven wakeups preferred over polling
- minimal resident memory per idle agent

### Reliability
- restart-safe
- task state persisted
- schedule dedupe on restart
- clean shutdown
- retries with jitter/backoff
- dead-letter support
- health reporting

### Security
- least privilege
- auth everywhere it matters
- public/private API separation
- strong origin controls
- signed webhooks
- Docker isolation
- no unrestricted host access
- explicit tool/network policies
- audit trails

### Developer experience
- clean code layout
- small interfaces
- strong typing
- clear docs
- testable modules
- versioned APIs
- examples and curl snippets

---

## Required implementation output

When doing this upgrade, produce:

1. **A repo-wide architecture audit**
2. **A concrete gap analysis**
3. **A prioritized roadmap**
4. **A revised folder structure**
5. **New/updated Go packages**
6. **New database schema and migrations**
7. **New API handlers**
8. **Public API auth model**
9. **Frontend widget + JS SDK design**
10. **Benchmark harness**
11. **Updated Dockerfiles and Compose**
12. **Updated scripts**
13. **Tests**
14. **Docs**
15. **Example configs**
16. **Security notes**
17. **Performance notes**
18. **Acceptance criteria**

---

## Expected revised repository shape

Aim toward something like this:

```text
Zclaw/
  cmd/
    dockclawd/
    dockclawctl/
  internal/
    agents/
    api/
      admin/
      public/
      sse/
      websocket/
      middleware/
    auth/
    browser/
    browserpool/
    config/
    connections/
    embeddings/
    events/
    heartbeat/
    memory/
    metrics/
    policies/
    providers/
      adapters/
    publicwidget/
    queue/
    rate_limit/
    runtime/
    scheduler/
    sessions/
    storage/
      sqlite/
      postgres/
      migrations/
    tasks/
    telemetry/
    tenants/
    tools/
    usage/
    webhooks/
    workspaces/
  web/
    dashboard/
    embed-widget/
    sdk/
  browser-worker/
  docker/
    compose.yaml
    compose.dev.yaml
    images/
  scripts/
  docs/
    architecture/
    api/
    widget/
    providers/
    ops/
    benchmarks/
  examples/
  tests/
```

---

## API contract expectations

Define stable versioned contracts.

### Admin API
For operators, orgs, tenants, dashboard, agents, schedules, tasks, widget configs, providers, audits, metrics.

### Public API
For website/app assistant usage, with sharply limited scope and safe defaults.

### Streaming
Use SSE for easiest frontend integration.
Optionally support WebSocket for richer clients.

### Compatibility
Where practical, support:
- OpenResponses-like inputs
- OpenAI-style chat/completions compatibility
- structured tool-calling envelopes

---

## Data model requirements

Design data models for:

- tenants
- projects
- users
- API keys
- public widget configs
- allowed origins
- agents
- agent versions
- sessions
- messages
- attachments
- artifacts
- tasks
- task events
- schedules
- heartbeats
- provider configs
- worker leases
- browser leases
- audits
- usage counters
- rate-limit buckets

---

## Testing requirements

Add tests for:

- provider registry
- API auth
- public widget token validation
- origin enforcement
- task enqueue/dequeue
- schedule dedupe
- heartbeat jitter
- worker pool cap behavior
- browser lease behavior
- artifact cleanup
- SSE streaming
- session resume
- upload policy
- benchmark harness sanity

Also add integration tests with Docker Compose.

---

## Documentation requirements

Write docs that are good enough for:

- a solo self-hoster
- a VPS deployer
- a SaaS-style internal team
- a developer embedding the widget into a website

Include copy-paste examples.

---

## Migration strategy

Do not break the repo carelessly.

Follow this migration sequence:

1. audit current code and mark stable pieces to keep
2. extract interfaces where missing
3. add missing packages incrementally
4. stabilize storage and migrations
5. stabilize queue/task model
6. stabilize public/admin APIs
7. add widget SDK
8. add dashboard
9. add benchmarks
10. harden ops and docs

---

## Deliverable style

When you respond and implement:

- be concrete
- show code-level structure
- propose schemas and endpoints
- specify config keys
- define interfaces
- outline migrations
- include acceptance criteria
- avoid vague filler
- call out tradeoffs
- prefer smaller, composable components

---

## Final acceptance criteria

The upgrade is only complete when all of the following are true:

1. Zclaw can be installed on a headless Linux server via Docker with simple scripts.
2. A user can create many agents without paying the idle cost of many live runtimes.
3. Browser tasks work in a headless environment using a pool instead of per-agent browsers.
4. The system has a strong provider abstraction with broad provider support.
5. The system has a public/admin API separation.
6. A developer can embed Zclaw as a website assistant using a script or iframe.
7. The website assistant supports streaming and session persistence.
8. The system has auth, tenancy, rate limits, and origin controls for public use.
9. The system has a dashboard and operator observability.
10. The repo ships benchmark tools proving density claims.
11. The docs and scripts are good enough for real-world use.
12. The architecture remains lighter at idle than desktop-first agent systems.

---

## First task you must do

Start by producing these sections in order:

1. **Current-state audit of the existing Zclaw repo**
2. **What to keep**
3. **What to refactor**
4. **What to add immediately**
5. **Public API and widget design**
6. **Revised architecture**
7. **Milestone roadmap**
8. **Concrete file-by-file implementation plan**
9. **Database migration plan**
10. **API endpoint specification**
11. **Benchmark plan**
12. **Security hardening checklist**
13. **Ops/documentation checklist**

Then proceed to implement the plan in the repository.

Do not answer with generic ideas only.
Answer like the engineer who is about to actually build the system.
