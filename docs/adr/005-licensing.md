# ADR-005: Licensing and Code Reuse Policy

## Status
Accepted

## Context
ZClaw draws architectural inspiration from several open-source projects: OpenClaw, OpenHands, Goose, GoGogot, and AnythingLLM. The team must decide what can be reused, what must be studied-only, and how to track provenance.

## Decision

### Allowed with attribution
- MIT-licensed code may be selectively reused with license notices preserved.
- Apache-2.0 code may be selectively reused with license notices and NOTICES file preserved.

### Inspiration only
- Architecture patterns, interface designs, and operational ideas may be adopted from any project regardless of license.
- No code is copied from these sources without explicit license compatibility verification.

### Restricted
- Browserless code must not be reused directly until a formal license review confirms compatibility.
- Projects with unclear or non-permissive licenses are studied for ideas only.

### Mandatory provenance tracking
- Every file or substantial code block borrowed from an external project must include a comment noting the source, license, and date.
- A `PROVENANCE.md` file should track all external code contributions.

### Hard rule
No code is pasted into the repository for convenience. If it is useful enough to include, it is useful enough to attribute properly.

## Consequences
- ZClaw remains a clean implementation with its own architecture
- License compliance is auditable via PROVENANCE.md
- Contributors know what can and cannot be copied
- Browserless integration (if added) requires a separate licensing decision

## Alternatives Considered
- **No reuse policy**: Risky. Could lead to accidental license violations.
- **Fork-and-modify**: Faster initially but constrains architecture to upstream decisions.
- **Rewrite everything**: Pure but slow. Selective reuse with attribution is more practical.
