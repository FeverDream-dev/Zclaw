# ADR-005: Code reuse policy, inspiration vs copy

## Status
Proposed

## Context
- ZClaw studies multiple external sources and open projects (OpenClaw, OpenHands, Goose, GoGogot, AnythingLLM) as potential primitives or inspiration for components.
- We want to reuse ideas and code where permissible, but avoid blind copying and ensure proper provenance and licensing compatibility.
- Browserless licensing restrictions require careful handling when incorporating browser automation tooling or hosted virtualization in our stack.
- Our density target (100-300 agents) influences how much external code we can safely embed or reuse without escalating licensing obligations or support burdens.
- Clear attribution, license compatibility checks, and an auditable provenance trail help maintain compliance across the project.

## Decision
- Establish a policy for studying and reusing external code that favors permissive licenses (MIT, Apache 2.0, BSD) with attribution when used, and ensures compatibility with our own licensing.
- Permit code reuse and adaptations from external sources under compatible licenses, but require explicit provenance tracking, attribution, and version pinning.
- Prohibit wholesale, blind copying of code without an explicit review; require a short provenance report for any non-trivial borrow.
- Limit browserless-based tooling to internal licensing-compliant usage; do not rely on external hosted services unless licensing terms are fully understood and approved.
- Implement a lightweight governance process for license review, including a simple bill of materials (SBOM) for dependencies aligned with the 100-300 agent density model.
- Provide a template for attribution notices, license headers, and a changelog entry when incorporating external code.
- For any code reuse, ensure the distribution remains under an approved license, and communicate any restrictions to the team.

## Consequences
- Clear attribution and license compliance reduce legal and operational risks when building for 100-300 agents.
- The policy encourages disciplined exploration and reuse, avoiding license violations and license creep over time.
- Some external code may require additional maintenance burdens (upgrades, security patches) to stay compatible with our platform.
- The licensing policy may slow rapid experimentation if permissive-but-limitations-based code is not readily available, but reduces risk in the long run.

## Alternatives Considered
- Unrestricted reuse with minimal attribution: high risk of license violation and maintenance complexity.
- Strict no-reuse stance: hinders learning from the broader ecosystem and slows feature delivery.
- Post-incident licensing review: reactive rather than proactive; rejected due to risk exposure and alignment with our policy goals.
- License-aware internal fork strategy: feasible but requires governance for downstream redistribution and ongoing maintenance.
