# ADR-004: No Vector Database for MVP Memory

## Status
Accepted

## Context
The MVP requires persistent memory, but its primary goal is to make memory inspectable, bounded, and useful for resumable coding sessions. Embeddings, vector search, and semantic retrieval would introduce extra infrastructure, evaluation difficulty, and failure modes before the project has proven that simpler memory is insufficient.

## Decision
We do not use a vector database in v1. Memory is implemented as constrained structured storage over simple persisted entries rather than embedding pipelines or semantic search. Retrieval stays within the SQLite/KV-style memory model and the explicit scope/type rules already defined by the system.

## Consequences
- The memory subsystem is easier to test, explain, and inspect.
- v1 avoids hidden ranking behavior and complex retrieval quality questions.
- Some semantic recall capabilities are intentionally out of scope for the MVP.
- If vector search is ever introduced later, it must justify its complexity against the current inspectable baseline.
