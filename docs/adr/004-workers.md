# ADR-004: Pooled On-Demand Workers

## Status
Accepted

## Context
ZClaw targets 100–300 logical agents on a single machine. If each agent had its own container, the system would require 100–300 containers at idle, consuming excessive memory, disk, and CPU. This directly contradicts the density target.

## Decision
Implement a pooled, on-demand worker model:

- Agents are database rows with config, schedules, and state. They are not containers.
- Tool workers (shell execution) and browser workers are launched on demand when a task requires them.
- A bounded worker pool caps concurrent execution contexts.
- Workers use shared base images to minimize disk overhead.
- Workers are automatically cleaned up after task completion.

The control plane manages a queue of pending tasks. When a worker slot is available, the scheduler assigns a task, launches a worker container (or reuses an existing one), and collects results.

## Consequences
- 300 idle agents consume near-zero CPU (just database rows)
- Only active tasks incur runtime overhead
- Shared base images reduce total disk usage
- Cold-start latency exists when spawning new workers (mitigated by pool warm-up)
- The control plane must manage container lifecycle, timeouts, and cleanup

## Alternatives Considered
- **One container per agent**: Simple isolation but directly violates density targets. 300 containers at idle would consume 15–60GB of RAM.
- **Serverless (Lambda-style)**: Good density but requires external infrastructure (AWS Lambda, Knative). Violates "no unnecessary infra in v1."
- **Single shared container**: Maximum density but zero isolation. Agents could interfere with each other's filesystem and processes.
