# ADR-002: SQLite Persistence and Constrained Memory

## Status
Accepted

## Context
The agent must support resumable sessions and inspectable memory, but the MVP should avoid operational complexity. The project also needs a storage option that works in local CLI environments, integrates cleanly with Go, and does not require CGO or external infrastructure.

## Decision
We persist sessions and memory in SQLite using `modernc.org/sqlite`, a pure-Go driver. Memory entries are constrained by explicit scope and type boundaries rather than free-form long-term storage. Supported scopes are `session`, `project`, and `global`. Supported memory types are `preference`, `fact`, and `summary`. Entries must remain inspectable and provenance-backed instead of opaque blobs.

## Consequences
- Contributors can run the full MVP locally without provisioning a separate database service.
- Session resume and memory inspection stay simple and portable across development environments.
- The memory model remains intentionally narrow, which reduces accidental over-collection and unclear retention semantics.
- If future scale or query needs change, storage evolution must preserve provenance and inspectability rather than bypass them.
