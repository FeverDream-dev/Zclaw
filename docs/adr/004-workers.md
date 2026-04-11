# ADR-004: Pooled/on-demand workers, not per-agent containers

## Status
Proposed

## Context
- ZClaw targets a density of 100-300 agents on a single host; creating a dedicated container per agent would multiply the number of containers quickly and consume excessive memory and CPU overhead.
- A pooled worker model allows multiple agents to share a common worker process or pool of workers that can be assigned to tasks on demand.
- The architecture relies on a shared base image for workers to minimize startup latency and image size while supporting lazy spawning of new workers when the pool is exhausted.
- Cold-start latency must be kept within acceptable bounds to maintain responsiveness for task scheduling across hundreds of agents.
- The control plane should be capable of dynamically sizing worker pools according to load, with bounded concurrency to prevent resource thrash.
- It is essential to minimize the number of long-running processes to reduce memory pressure during idle periods and simplify updates.

## Decision
- Implement a pool of worker processes (or lightweight worker threads) shared by all agents, with a target density design point of 100-300 agents on a single host.
- Use a single, shared base image for workers to reduce duplication; spawn workers lazily on demand and reuse them when possible.
- Bound the pool size with a configurable maximum to prevent overcommit and ensure predictable memory usage; monitor and adapt through backpressure signals from the control plane.
- Maintain a clear mapping from agents to worker contexts, with eviction and recycling policies to ensure resource fairness.
- Communicate task assignments and results through a lightweight, high-throughput IPC channel (e.g., sockets or in-process channels if within the same binary) to minimize inter-process costs.
- Allow hot-swap of worker implementations to accommodate updates without disrupting the agent workload.
- When resources permit, scale the pool within the density target, with explicit plan to expand only after validating performance at 300 agents.
- Document acceptance criteria for when to spawn new workers and when to prune idle workers to maintain the target density.

## Consequences
- Significantly reduced per-agent overhead compared to per-agent containers; improved memory footprint and faster startup for agents.
- Improved density: 100-300 agents become feasible with a shared worker pool, provided the pool sizing and backpressure policies are tuned.
- Simpler deployment: a single base image reduces CI/CD complexity and streamlines upgrades.
- Complexity shifts toward pool management, scheduling fairness, and lifecycle management; requires robust metrics, health checks, and backpressure controls.
- Cold-start latency can still appear when a pool needs to spin up a new worker; mitigations include warm pools and predictive scaling.

## Alternatives Considered
- Per-agent containers: easy isolation but scales poorly at 100-300 agents due to container overhead, memory fragmentation, and orchestration complexity.
- Serverless per-task workers: reduces long-running state but incurs cold-start latency and potential latency spikes for every task.
- Hybrid model: a small number of long-running agents with per-task worker threads; considered but less scalable if tasks grow beyond the density target.
- Dedicated long-running master processes per agent: too heavy and complex for maintenance at scale.
