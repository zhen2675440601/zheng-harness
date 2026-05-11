- T6: SQLite server-path hardening benefits from setting a connection-level `busy_timeout` even with WAL enabled; concurrent session writers stop surfacing transient lock failures under parallel step appends.
- T6: `inspect` truth stays available during active writes and after writer shutdown when readers use separate WAL-enabled connections against committed state.
- T8: OpenAI-compatible provider tests need separate coverage for deterministic transport/auth/malformed-response failures on both `Generate` and `Stream`, because runtime surfaces provider errors verbatim through the adapter boundary.

- T9: Anthropic provider parity needs deterministic failure classes on both non-streaming and SSE paths; transport/auth/malformed-response coverage catches regressions where raw http errors bypass fail-closed runtime messaging.
- T10: Server integration coverage is more reliable when test harness clocks advance per request; fixed timestamps caused duplicate server-generated session IDs and false 409 conflicts under concurrent run tests.
- T10: SSE integration tests need subscribers attached before emitting runtime events; gating emission on a post-stream-open signal avoids flaky missing-frame failures in session stream assertions.
- T10: Graceful shutdown currently rejects new API starts by surfacing the manager-closed path as a deterministic 500 `start session` error while still cancelling in-flight sessions to interrupted state.

- Chat-oriented API support can be layered onto the existing session persistence by storing `conversation` metadata inside `sessions.config_json`, which avoids new tables while still enabling conversation→session chaining and transcript assembly across multiple sessions.
- Preserving backward compatibility required keeping `StartConversation`/`ListConversations` semantics session-centric while introducing new `StartChat`/`SubmitReply`/`ListChats` methods for first-class conversation behavior.
