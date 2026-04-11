# ADR-002: SQLite WAL first, Postgres later

## Status
Proposed

## Context
- ZClaw stores state about 100-300 agents across a single host. The storage layer must be zero-config, single-file when possible, and operate with minimal daemon overhead.
- The first-class datastore candidate is SQLite with Write-Ahead Logging (WAL) mode to maximize concurrent reads and writes without requiring a separate database daemon.
- Postgres remains a desirable long-term target for scale-out scenarios, but adds operational complexity and heavier resource footprints on small hosts.
- We must plan for a migration path from SQLite to Postgres without invasive rewrites to the rest of the codebase, so an interface-based data access layer is essential.
- Other embedded/key-value stores (BadgerDB, Bolt) were evaluated but fall short in terms of SQL-like querying needs, migrations, and ecosystem parity for complex agent state.
- Density target of 100-300 agents informs the practical limits for a single-host database; WAL enables safer concurrent access for reads while writes occur.

## Decision
- Start with SQLite 3.x in WAL mode for the primary on-disk store during day-to-day operation.
- Enable WAL mode at database open time and apply a minimal PRAGMA configuration to optimize for concurrent reads with occasional writes from the control plane and Playwright sidecar events.
- Implement a DB access layer with a small repository abstraction that exposes common operations (get, set, list, observe streams) and is independent of the underlying engine.
- Design the storage interface so the implementation can swap to Postgres later with no callers’ changes; wire via adapters and dependency injection.
- Prepare a Postgres-backed implementation as a drop-in replacement behind the same interface, including migrations and schema evolution tooling.
- For now, avoid distributed SQL or clustering; keep a single-node store aligned with the density target, enabling predictable memory and I/O patterns.
- Document migration strategy and the minimal feature matrix required for Postgres parity (types, indexes, transactions).

## Consequences
- Zero-config local database gives developers fast iteration and reliable startup, which is ideal for a density target of 100-300 agents.
- WAL mode supports multiple readers and a single writer scenario, which aligns with the control plane's pattern of frequent reads and occasional writes.
- The abstraction layer isolates callers from DB specifics, making future migration to Postgres straightforward.
- Potential caveats include slightly increased complexity in setup when enabling WAL on SQLite and ensuring proper vacuuming and checkpointing to avoid long-lived transactions.
- On larger density growth beyond 300 agents, Postgres will be necessary to meet concurrent transaction demands; the design anticipates that transition.

## Alternatives Considered
- Postgres from the start: offers robust concurrency and horizontal scaling but introduces daemon management, initialization overhead, and a more complex deployment footprint on small hosts.
- BadgerDB or Bolt: lightweight embedded stores; simpler to deploy but lack sophisticated SQL querying, tooling, and mature migrations.
- LMDB/SQLite without WAL: simpler but suffers from limited concurrent write behavior and higher risk under mixed-read/write workloads.
- A distributed key-value store (e.g., etcd) was considered not aligned with the local single-host density constraints and adds networked coordination complexity.
