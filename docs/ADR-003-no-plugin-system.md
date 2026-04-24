# ADR-003: No Plugin System in v1

## Status
Accepted

## Context
The agent needs a safe tool layer, but dynamic extension mechanisms would expand the trust boundary early. A plugin system adds loading concerns, compatibility guarantees, version negotiation, and security review overhead before the core runtime is stable.

## Decision
Tools in v1 are registered in-process through the tool registry and bound to explicit executors. New tools are added by changing Go code, defining metadata such as schema, timeout, and safety level, and wiring the executor directly. We do not support dynamic loading, external plugin packages, or runtime-discovered extensions.

## Consequences
- Tool behavior remains reviewable in source control and testable through the normal Go workflow.
- Safety policy stays centralized instead of being spread across arbitrary extension points.
- Contributors can extend the toolset, but only through the existing registry and executor path.
- v1 intentionally trades extensibility for predictability, security, and implementation simplicity.
