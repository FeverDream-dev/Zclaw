# ADR-001: Why Go for control plane

## Status
Proposed

## Context
- ZClaw's control plane must orchestrate 100-300 logical agents on a single Linux host with very limited memory headroom.
- The control plane communicates with a Playwright-based browser sidecar that runs in a separate process; reliability of the browser workers is critical but should be isolated from the orchestrator logic.
- We need a language/runtime with a small binary, low runtime overhead, strong concurrency primitives, and predictable ops for deployment at scale.
- Node-based control plane would imply embedding a full Node runtime and V8 heap into the control loop, which increases binary size, memory footprint, and operator surface area. There is a known Node browser-worker exception risk when scaling to hundreds of concurrent browser tasks.
- The density target of 100-300 agents implies the control plane must efficiently schedule, monitor, and recover a large number of lightweight agent state machines while keeping memory usage predictable.
- We also aim for fast deploys, straightforward instrumentation, and simple cross-compilation and binary distribution.

## Decision
- Implement the control plane in Go and maintain a compact, self-contained binary.
- Structure the orchestrator around a small, event-driven core that uses goroutines for agent-level parallelism and channels for coordination.
- Expose a stable, language-agnostic API (HTTP/REST and gRPC as needed) for the Node/browser sidecar to interact with the orchestrator, fed by a persistent store for agent state.
- Keep the browser workers in Playwright as a separate Node-based sidecar process; the Go control plane schedules tasks and communicates with the Node sidecar via the defined API, isolating Node's runtime from the core control plane.
- Favor a hexagonal architecture where the data layer (SQLite/Postgres interface later) and the delivery layer (API) are decoupled from the orchestration/core logic.
- Leverage Go's native tooling for static builds, cross-compilation, and straightforward deployment in containerless or minimal-container environments.
- Document and implement explicit error boundaries around the Node browser-worker boundary to avoid cascading failures into the control plane.

## Consequences
- Smaller, faster-to-start binary improves boot times and reduces memory pressure under load; the 100-300 agent target becomes more predictable to run on commodity hardware.
- Go's concurrency primitives enable high-throughput scheduling of hundreds of agents with relatively low memory overhead per goroutine, improving density resilience.
- A clear boundary between the Go control plane and the Node Playwright sidecar reduces fault domains and simplifies recovery/upgrade paths.
- Observability and debugging tooling in Go are straightforward, aiding operator onboarding and incident response.
- The Node browser-worker boundary introduces an IPC/serialization cost and a potential bottleneck at the sidecar API, which we mitigate with asynchronous queues and backpressure controls.
- Potential downsides include the need to maintain a Go-Node bridge and ensure that the browser sidecar protocol remains stable across releases.

## Alternatives Considered
- Node as the control plane: offers native JavaScript ecosystem cohesion but incurs a heavier runtime, larger binary, and more memory per process; would complicate scaling to 100-300 agents.
- Rust: promises small binaries and strong safety; however, Go provides simpler ergonomics, faster iteration, and a broader ops ecosystem, which aligns better with rapid deployment and maintenance in production.
- Python: rapid development but higher memory footprint and Global Interpreter Lock (GIL) implications reduce true concurrency for scheduling hundreds of agents.
- C/C++: potential for minimal footprint but increases complexity of build, safety, and long-term maintainability.
