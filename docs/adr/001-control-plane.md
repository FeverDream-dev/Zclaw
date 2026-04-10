# ADR-001: Go for Control Plane

## Status
Accepted

## Context
ZClaw needs a control plane that manages 100–300 logical agents on a single server with minimal idle CPU and memory overhead. The control plane handles orchestration, scheduling, storage, Docker interaction, and provider API calls. We must choose a language that serves as the single always-on process.

## Decision
Use Go as the primary language for the control plane service (`dockclawd`).

Go provides:
- Single static binary under 30MB (multi-stage Alpine build)
- Native concurrency via goroutines for scheduler, worker pool, and API server
- Lower baseline memory (~20–40MB idle) compared to Node (~80–150MB) or Python runtimes
- Excellent Docker and SQLite ecosystem support
- No runtime dependency on Node, Python, or any interpreter

Node is retained exclusively for the Playwright browser worker sidecar where the Playwright API ecosystem is strongest.

## Consequences
- Control plane is a small, self-contained binary
- All agent orchestration, storage, and scheduling logic lives in one codebase
- Browser worker remains a separate Node container with its own lifecycle
- Operators need no host-level language runtimes

## Alternatives Considered
- **Node.js**: Good ecosystem, but higher memory baseline. A Node control plane managing 300 agents would consume significantly more RAM at idle.
- **Rust**: Excellent performance and memory, but slower iteration, smaller ecosystem for Docker/SQLite, and steeper contributor requirements.
- **Python**: Strong AI/ML ecosystem, but GIL limitations for concurrency, heavier runtime, and worse density characteristics.
