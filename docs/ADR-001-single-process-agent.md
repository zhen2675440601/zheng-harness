# ADR-001: Single-Process Single-Agent Runtime

## Status
Accepted

## Context
The MVP needs to be runnable from a local CLI, easy to debug, and small enough for a 1-2 person team to maintain. The project goal is to validate Harness Engineering for a general-purpose agent harness, not to build a distributed orchestration platform. Adding multi-agent coordination, background workers, or a service mesh in v1 would increase failure modes, make session persistence harder to reason about, and complicate testing.

## Decision
We run the v1 agent as a CLI-first, single-process, single-agent system. One runtime loop owns planning, tool execution, observation, verification, and self-correction for one session at a time. The primary interface is the `zheng-agent` CLI with `run`, `resume`, and `inspect` commands.

## Consequences
- The runtime stays inspectable and deterministic enough for TDD and replay testing.
- Interrupt handling and session persistence remain local process concerns instead of distributed coordination problems.
- v1 explicitly excludes multi-agent orchestration, remote gateways, and service deployment concerns.
- Future expansion is still possible, but only after the single-agent operating model proves useful.
