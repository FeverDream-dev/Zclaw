# ADR-002: SQLite WAL as Default Storage

## Status
Accepted

## Context
ZClaw targets single-node deployment with 100–300 agents. Storage must persist agents, schedules, tasks, provider configs, conversations, artifacts metadata, and audit logs. The system must work without requiring Postgres, Redis, or any external database daemon.

## Decision
Use SQLite in WAL (Write-Ahead Logging) mode as the default storage backend for single-node deployments.

WAL mode provides:
- Concurrent reads while writes are in progress
- No separate database process (embedded in the Go binary via modernc.org/sqlite)
- Single-file database that is trivial to backup
- Zero configuration, zero daemon overhead
- Prepared statements for query performance

The storage layer is abstracted behind repository interfaces so that Postgres can be added later as an optional backend without modifying the agent core or scheduler.

## Consequences
- Fresh install has zero database setup
- Backup is a file copy plus WAL checkpoint
- Write throughput is limited to one writer at a time (acceptable for 100–300 agents)
- Migration to Postgres requires implementing the same repository interfaces
- modernc.org/sqlite is pure Go (no CGO), keeping the build simple

## Alternatives Considered
- **Postgres**: Production-grade but requires a separate daemon, configuration, and backups. Overkill for single-node v1. Added later as optional backend.
- **BadgerDB**: Key-value store, good performance, but lacks SQL query flexibility for agent filtering and pagination.
- **bbolt**: Simple embedded KV, but no concurrent read support and no SQL ergonomics for complex queries.
