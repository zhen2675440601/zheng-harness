# ADR-005: Test-Driven Development First

## Status
Accepted

## Context
This project implements agent runtime behavior, persistence, safety policy, and verification logic. These areas are easy to regress if they are only validated manually. The project plan also requires evidence-driven verification rather than trust in implementation claims.

## Decision
We adopt TDD as the default development workflow for v1. New behavior should start with a failing test, then implementation, then refactoring while keeping the test suite green. CI and local contributor workflows are built around `go test`, race checks, and coverage generation.

## Consequences
- Runtime, store, and tool behavior are guarded by executable specifications instead of documentation alone.
- Contributors are expected to make changes through tests first, especially for agent loop and safety-sensitive behavior.
- The repository optimizes for maintainability and regression detection over rapid speculative coding.
- Changes that cannot be described by tests should be treated as a design smell and clarified before implementation.
