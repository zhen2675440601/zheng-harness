# v3 Core Extensibility: Provider + Agent + Verifier Plugins

## TL;DR
> **Summary**: v3 evolves zheng-harness from a tool-plugin-capable engine into a host-controlled extensibility kernel by pluginizing provider, agent strategy, and verifier families without expanding product surface or weakening recoverability.
> **Deliverables**: Provider plugin seam, strategy-style agent plugin seam, predeclared-policy verifier plugin seam, host-owned registries/lifecycle/provenance, fail-closed behavior, TDD-backed compatibility coverage.
> **Effort**: XL
> **Parallel**: YES - 5 waves
> **Critical Path**: Contract decisions → provider seam → verifier seam → agent seam → integration/provenance/CLI

## Context
### Original Request
v2的内容也已完成，后续应该如何规划。

### Interview Summary
- v2 is treated as completed baseline; no reopening of completed streaming/tool-plugin/orchestration work.
- Next milestone should be a **formal v3 plan**.
- Priority is **capability expansion**, not product entrypoints or hardening-only work.
- v3 scope boundary is **core extensibility only**: provider plugins + agent plugins + verifier plugins.
- Testing strategy is **TDD**.
- Agent plugins are **strategy-style extensions** only: host keeps orchestration, lifecycle, cancellation, and persistence authority.
- Verifier plugins may **only implement host-predeclared verification policies**; they do not mint arbitrary new policy names.
- Plugin failure policy is **fail-closed** for run/resume selection; inspect must remain readable.

### Metis Review (gaps addressed)
- Distinguish contracts by family; do not force a fake generic abstraction across provider, agent, verifier.
- Keep built-ins first-class and always available.
- Treat external-process loading as the portability baseline if dual-mode remains available.
- Add provenance persistence so inspect/resume remain trustworthy even if plugin artifacts later disappear.
- Define collision, fallback, and selection precedence rules explicitly.
- Keep memory scope constrained; plugin seams must not backdoor vector memory or autonomous long-term knowledge stores.
- Agent plugins are deferred behind provider and verifier seams because they carry the highest scope-creep risk.

## Work Objectives
### Core Objective
Introduce host-controlled plugin seams for provider selection, agent strategy selection, and verifier selection so zheng-harness becomes an extensible platform kernel while preserving current CLI-first operation, inspectability, recoverability, and verification authority.

### Deliverables
1. **Provider extensibility seam** mapped from current `internal/llm` hardcoded selection into host-owned registry + loader + provenance model.
2. **Verifier extensibility seam** mapped from current task-aware hardcoded dispatch into predeclared-policy plugin-backed strategy registration.
3. **Agent strategy extensibility seam** that allows strategy injection without surrendering orchestration, cancellation, persistence, or task graph authority.
4. **Shared host-side plugin governance primitives** for discovery, validation, collision handling, lifecycle, and fail-closed selection.
5. **Compatibility + provenance support** for config, CLI, session persistence, inspect, and resume.

### Definition of Done (verifiable conditions with commands)
```bash
go build ./...                                                      # Code compiles across all packages
go test ./...                                                       # Full suite passes
go test -race ./...                                                 # No race regressions
go test ./internal/plugin/...                                       # Shared loader/contract tests pass
go test ./internal/llm/... -run TestProviderPlugin                  # Provider plugin seam tests pass
go test ./internal/verify/... -run TestVerifierPlugin               # Verifier plugin seam tests pass
go test ./internal/runtime/... -run TestAgentStrategyPlugin         # Agent strategy plugin seam tests pass
go test ./cmd/agent/... -run TestPluginSelection                    # CLI/config selection tests pass
go test ./internal/store/... -run TestPluginProvenance              # Persistence/provenance tests pass
```

### Must Have
- Distinct extension contracts for provider, verifier, and agent strategy families.
- Built-in provider/agent/verifier implementations remain available and usable with zero plugins installed.
- Host-owned registries decide discovery, ID namespace, collision policy, selection precedence, lifecycle, and fallback semantics.
- Provider plugins extend the host-facing model/provider seam rather than replacing runtime ownership.
- Verifier plugins implement only host-predeclared policies.
- Agent plugins are strategy-style only; no recursive agent ecosystem and no transfer of orchestration control.
- Fail-closed behavior for configured plugin selection failures.
- Provenance persisted for plugin-backed execution: family, logical ID, execution mode, contract version, implementation version.
- Resume failures are deterministic and inspect remains readable even when plugin binaries are later absent.
- TDD-first implementation with contract, compatibility, and failure-isolation coverage.

### Must NOT Have (guardrails, AI slop patterns, scope boundaries)
- NO provider expansion campaign; v3 is mechanism-first, not many-new-providers work.
- NO web UI, HTTP API server, daemon mode, message gateway, or new product entrypoint.
- NO plugin marketplace, remote install, signature distribution service, or update channel.
- NO recursive worker spawning, peer-to-peer agent mesh, or freeform sub-agent ecosystem.
- NO verifier self-certification; host owns pass/fail framing and result schema validation.
- NO vector DB, embeddings, semantic memory expansion, or unconstrained knowledge stores.
- NO breaking removal of built-in provider/agent/verifier paths.
- NO speculative mega-abstraction that unifies all plugin families behind one leaky meta-interface.

## Verification Strategy
> ZERO HUMAN INTERVENTION - all verification is agent-executed.
- Test decision: TDD (RED-GREEN-REFACTOR)
- QA policy: Every task includes agent-executed happy path + failure/edge case scenarios
- Evidence: `.sisyphus/evidence/task-{N}-{slug}.{ext}`
- Compatibility policy: every plugin family requires contract tests, missing-plugin tests, version-mismatch tests, collision tests, and provenance tests

## Execution Strategy
### Parallel Execution Waves
Wave 1: Contract and governance foundation
Wave 2: Provider plugin seam
Wave 3: Verifier plugin seam
Wave 4: Agent strategy plugin seam
Wave 5: Integration, persistence provenance, CLI/config, docs

### Dependency Matrix (full, all tasks)

| Task | Blocks | Blocked By |
|------|--------|------------|
| T1 ADR for v3 extensibility boundaries | T2,T3,T4 | - |
| T2 Shared plugin family metadata/provenance model | T5,T6,T8,T11,T12 | T1 |
| T3 Shared host registry + selection rules | T5,T8,T11,T12 | T1 |
| T4 Shared fail-closed lifecycle + collision policy tests | T5,T8,T11,T12,T13 | T1 |
| T5 Provider contract + built-in adapter migration seam | T6,T7,T12 | T2,T3,T4 |
| T6 Provider external/native loader integration | T7,T12 | T2,T3,T4,T5 |
| T7 Provider config + CLI selection compatibility | T14 | T5,T6 |
| T8 Verifier contract + predeclared policy registry | T9,T10,T12 | T2,T3,T4 |
| T9 Verifier plugin loader + schema validation | T10,T12 | T8 |
| T10 TaskAwareVerifier host dispatch migration | T14 | T8,T9 |
| T11 Agent strategy contract + host execution boundary | T12,T13 | T2,T3,T4 |
| T12 Persistence/resume/inspect provenance support | T14 | T2,T3,T4,T6,T9,T11 |
| T13 Runtime/orchestration strategy plugin integration | T14 | T11 |
| T14 End-to-end integration + docs update | F1,F2,F3,F4 | T7,T10,T12,T13 |

### Agent Dispatch Summary
| Wave | Task Count | Categories |
|------|------------|------------|
| Wave 1 | 4 | writing, deep, deep, unspecified-high |
| Wave 2 | 3 | deep, deep, unspecified-high |
| Wave 3 | 3 | deep, deep, deep |
| Wave 4 | 2 | deep, deep |
| Wave 5 | 2 | deep, writing |

## TODOs
> Implementation + Test = ONE task. Never separate.
> EVERY task MUST have: Agent Profile + Parallelization + QA Scenarios.

- [ ] T1. Write ADR for v3 extensibility boundaries and family contracts

  **What to do**:
  1. Create a new ADR that formalizes v3 scope: provider plugins, strategy-style agent plugins, and verifier plugins only.
  2. Define three distinct host-owned contracts rather than a single generic meta-plugin interface.
  3. Document canonical portability rule: external-process loading is the semantic baseline; native loading remains optional optimization where already supported.
  4. Define fail-closed behavior for run/resume, inspect readability guarantees, collision rejection rules, and provenance persistence requirements.
  5. Document explicit non-goals: no marketplace, no product entrypoints, no vector memory expansion, no recursive agent ecosystems.

  **Must NOT do**: Do not implement any runtime code in this task. Do not promise remote distribution or backward-incompatible config changes.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: ADR quality determines downstream execution precision.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - No browser/UI work.

  **Parallelization**: Can Parallel: NO | Wave 1 | Blocks: T2, T3, T4 | Blocked By: none

  **References**:
  - Pattern: `docs/ADR-003-no-plugin-system.md` - Existing ADR style and decision-record tone.
  - Pattern: `docs/ADR-007-plugin-system-architecture.md` - v2 plugin architecture baseline for extension boundaries.
  - Reference: `internal/plugin/doc.go:1-5` - v2 explicitly deferred provider/agent/verifier plugins.
  - Reference: `.sisyphus/plans/v2-evolution.md:26-33` - Guardrails for portability baseline and scope control.

  **Acceptance Criteria**:
  - [ ] New ADR exists and explicitly defines distinct contracts for provider, verifier, and agent strategy families.
  - [ ] ADR states fail-closed runtime behavior and inspect/readability guarantees.
  - [ ] ADR states built-ins remain first-class defaults with zero plugins installed.
  - [ ] ADR includes explicit non-goals matching v3 boundaries.

  **QA Scenarios**:
  ```
  Scenario: ADR includes all mandatory v3 boundary decisions
    Tool: Bash
    Steps: Run `grep -n "fail-closed\|built-in\|provider\|verifier\|agent strategy\|non-goals" docs/ADR-*.md`
    Expected: New ADR contains all listed boundary topics with concrete language
    Evidence: .sisyphus/evidence/task-1-adr-boundaries.txt

  Scenario: ADR does not reopen excluded scope
    Tool: Bash
    Steps: Inspect ADR text for forbidden scope terms such as marketplace, HTTP API server, vector DB, recursive agents without allowed-context wording
    Expected: Forbidden items appear only inside explicit non-goals/exclusions
    Evidence: .sisyphus/evidence/task-1-adr-nongoals.txt
  ```

  **Commit**: YES | Message: `docs(adr): define v3 extensibility boundaries` | Files: `docs/ADR-*.md`

- [ ] T2. Introduce shared plugin metadata and provenance model

  **What to do**:
  1. Define host-owned metadata types for plugin family, logical ID, display name, contract version, implementation version, execution mode, and source path.
  2. Add provenance structures that can be attached to session/step records without requiring live plugin availability during inspect.
  3. Add serialization helpers and validation for known plugin families: tool, provider, verifier, agent_strategy.
  4. Keep the model additive so existing persisted sessions remain readable.
  5. Add tests that prove provenance objects round-trip cleanly and reject unknown/invalid family values.

  **Must NOT do**: Do not migrate runtime selection yet. Do not add marketplace metadata such as signatures, remote URLs, or update channels.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Shared metadata influences llm, runtime, store, inspect, and plugin management.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5, T6, T8, T11, T12 | Blocked By: T1

  **References**:
  - Pattern: `internal/plugin/manager.go:25-42` - Existing discovered/loaded plugin tracking concepts.
  - Pattern: `internal/domain/session.go` - Session persistence shape to extend additively.
  - Pattern: `internal/domain/step.go` - Step history model for per-execution provenance.
  - Reference: `README.md` inspect/resume continuity section - persisted history must stay readable.

  **Acceptance Criteria**:
  - [ ] Shared metadata/provenance types exist with validation for family, execution mode, and contract version fields.
  - [ ] Unknown plugin family or malformed version metadata is rejected by tests.
  - [ ] Existing persisted sessions remain decodable without mandatory plugin provenance fields.
  - [ ] `go test ./internal/... -run TestPluginProvenance` passes.

  **QA Scenarios**:
  ```
  Scenario: Provenance round-trip for valid plugin-backed execution
    Tool: Bash
    Steps: Run `go test ./internal/... -run TestPluginProvenanceRoundTrip -v`
    Expected: Test passes and proves encode/decode retains family, ID, mode, versions, and path
    Evidence: .sisyphus/evidence/task-2-provenance-roundtrip.txt

  Scenario: Reject malformed provenance metadata
    Tool: Bash
    Steps: Run `go test ./internal/... -run TestPluginProvenanceRejectsInvalid -v`
    Expected: Test passes by rejecting invalid family/mode/version values deterministically
    Evidence: .sisyphus/evidence/task-2-provenance-invalid.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add shared provenance metadata model` | Files: `internal/domain/*, internal/plugin/*, internal/store/*`

- [ ] T3. Build host-owned registry and selection rules for extensibility families

  **What to do**:
  1. Design and implement host-owned registries for provider, verifier, and agent strategy families with explicit namespace ownership.
  2. Define deterministic collision policy: duplicate IDs between built-ins and plugins are rejected unless explicitly namespaced by host rules.
  3. Define selection precedence and lookup APIs used by runtime/config/CLI.
  4. Preserve built-ins as always-registered defaults even when no plugin directory exists.
  5. Add concurrency-safe tests for registry registration, lookup, duplicate rejection, and family separation.

  **Must NOT do**: Do not let plugins self-register global routing authority. Do not hide family-specific behavior behind an over-generic registry API that erases contract differences.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: This is the core host-control boundary for all later tasks.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5, T8, T11, T12 | Blocked By: T1

  **References**:
  - Pattern: `internal/plugin/manager.go:31-50` - Existing manager state ownership and mutex discipline.
  - Reference: `internal/llm/provider.go:41-65` - Current hardcoded provider selection that must move behind registry.
  - Reference: `internal/verify/task_aware_verifier.go:33-40` - Current hardcoded verifier strategy map.

  **Acceptance Criteria**:
  - [ ] Family-specific registries exist and are concurrency-safe.
  - [ ] Duplicate logical IDs are rejected deterministically with test coverage.
  - [ ] Built-ins are present by default without plugin discovery.
  - [ ] Selection precedence is documented in code comments/tests and consumed by callers via host APIs.

  **QA Scenarios**:
  ```
  Scenario: Built-ins remain available with zero plugins installed
    Tool: Bash
    Steps: Run `go test ./internal/... -run TestRegistryBuiltinsAvailableWithoutPlugins -v`
    Expected: Test passes; registry resolves built-in provider/verifier/agent strategy IDs without discovery path
    Evidence: .sisyphus/evidence/task-3-builtins.txt

  Scenario: Duplicate plugin ID collision is rejected
    Tool: Bash
    Steps: Run `go test ./internal/... -run TestRegistryRejectsDuplicateIDs -v`
    Expected: Test passes with deterministic collision error and no partial registration
    Evidence: .sisyphus/evidence/task-3-collision.txt
  ```

  **Commit**: YES | Message: `feat(plugin): add host-owned family registries` | Files: `internal/plugin/*, internal/llm/*, internal/verify/*, internal/runtime/*`

- [ ] T4. Add fail-closed lifecycle, capability validation, and collision policy test harness

  **What to do**:
  1. Extend host-side plugin management tests to cover provider/verifier/agent strategy family loading failure modes.
  2. Define deterministic errors for contract version mismatch, invalid metadata, startup timeout, shutdown failure, and unsupported native mode.
  3. Enforce fail-closed selection semantics for run/resume while preserving inspect readability.
  4. Add race-safe lifecycle tests around concurrent load/unload and partial initialization failure cleanup.
  5. Capture reusable test harness helpers for plugin artifact simulation across families.

  **Must NOT do**: Do not silently auto-fallback at runtime. Do not leave half-loaded plugin instances registered after validation failure.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Failure semantics and test harness breadth matter more than new product behavior.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 1 | Blocks: T5, T8, T11, T12, T13 | Blocked By: T1

  **References**:
  - Pattern: `internal/plugin/manager.go:88-159` - Load validation and cleanup flow.
  - Pattern: `internal/plugin/*_test.go` - Existing plugin test approach.
  - Reference: `README.md` inspect/resume behavior - inspect must not require live plugin presence.

  **Acceptance Criteria**:
  - [ ] Deterministic tests exist for version mismatch, invalid metadata, startup/shutdown errors, and unsupported mode.
  - [ ] Fail-closed behavior is covered for run/resume selection failures.
  - [ ] No partially loaded plugin remains registered after failed validation/load.
  - [ ] `go test -race ./internal/plugin/...` passes.

  **QA Scenarios**:
  ```
  Scenario: Version mismatch fails closed
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestPluginVersionMismatchFailsClosed -v`
    Expected: Test passes; plugin rejected; runtime selection error is deterministic; no fallback occurs
    Evidence: .sisyphus/evidence/task-4-version-mismatch.txt

  Scenario: Partial initialization cleans up correctly under race mode
    Tool: Bash
    Steps: Run `go test -race ./internal/plugin/... -run TestPluginLoadCleanupOnFailure -v`
    Expected: Test passes with no race and no leaked registration/close state
    Evidence: .sisyphus/evidence/task-4-cleanup-race.txt
  ```

  **Commit**: YES | Message: `test(plugin): enforce fail-closed lifecycle semantics` | Files: `internal/plugin/*_test.go, internal/plugin/testdata/*`

- [ ] T5. Introduce provider plugin contract and migrate built-in providers behind the seam

  **What to do**:
  1. Define a provider-family contract that extends the current host-facing provider/model boundary without surrendering runtime ownership.
  2. Refactor `internal/llm.NewProvider` hardcoded switch logic behind a host-owned provider registry.
  3. Register existing OpenAI, Anthropic, and DashScope implementations as built-ins through the same public seam used for plugin-backed providers.
  4. Preserve current streaming/generate behavior and provider config defaults.
  5. Add TDD coverage proving built-ins resolve through the new seam and unsupported provider IDs fail deterministically.

  **Must NOT do**: Do not add new provider integrations. Do not let provider plugins replace runtime planning/execution ownership. Do not break existing config defaults.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Provider selection sits on a hot path crossing config, llm, runtime, and streaming behavior.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T6, T7, T12 | Blocked By: T2, T3, T4

  **References**:
  - API/Type: `internal/llm/provider.go:24-65` - Current Provider interface and hardcoded constructor path.
  - API/Type: `internal/domain/ports.go:5-10` - Host-facing `domain.Model` seam that runtime depends on.
  - Pattern: `internal/llm/openai.go` - Existing built-in provider registration candidate.
  - Pattern: `internal/llm/anthropic.go` - Existing built-in provider registration candidate.
  - Pattern: `internal/llm/dashscope.go` - Existing built-in provider registration candidate.

  **Acceptance Criteria**:
  - [ ] Built-in providers register and resolve through host-owned provider registry.
  - [ ] Existing config-backed provider selection still works unchanged for built-in provider IDs.
  - [ ] Unsupported provider IDs fail deterministically with tests.
  - [ ] Streaming and non-streaming provider behavior remains green under existing tests.

  **QA Scenarios**:
  ```
  Scenario: Built-in provider resolves through plugin seam
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestProviderPluginBuiltinsUseRegistry -v`
    Expected: Test passes; built-in provider selection flows through registry-based seam instead of direct switch-only path
    Evidence: .sisyphus/evidence/task-5-provider-builtins.txt

  Scenario: Unknown provider ID fails deterministically
    Tool: Bash
    Steps: Run `go test ./internal/llm/... -run TestProviderPluginRejectsUnknownID -v`
    Expected: Test passes with stable unsupported-provider error and no panic
    Evidence: .sisyphus/evidence/task-5-provider-unknown.txt
  ```

  **Commit**: YES | Message: `feat(llm): add provider registry seam` | Files: `internal/llm/*, internal/plugin/*, internal/config/*`

- [ ] T6. Integrate provider plugin loading modes and contract validation

  **What to do**:
  1. Extend plugin loading for provider family artifacts using the host-owned metadata, contract validation, and fail-closed rules.
  2. Support the canonical external-process path first; keep native mode optional where the existing plugin infrastructure already supports it.
  3. Ensure provider plugin metadata exposes implementation version, contract version, logical ID, and execution mode for provenance capture.
  4. Add tests for startup failure, version mismatch, malformed responses, and unsupported mode behavior.
  5. Keep built-in providers available even when plugin discovery is disabled or empty.

  **Must NOT do**: Do not require native plugin parity on Windows. Do not auto-install or auto-discover from untrusted locations.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: This task expands current tool-only loader assumptions into provider-family semantics.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T7, T12 | Blocked By: T2, T3, T4, T5

  **References**:
  - Pattern: `internal/plugin/manager.go:52-128` - Discovery + load + validate flow.
  - Pattern: `internal/plugin/external.go` - External-process plugin loading baseline.
  - Pattern: `internal/plugin/native.go` - Native plugin loading constraints.
  - Reference: `.sisyphus/plans/v2-evolution.md:29-31` - External-process portability baseline.

  **Acceptance Criteria**:
  - [ ] Provider-family plugin artifacts validate contract version and metadata before registration.
  - [ ] External-process provider plugin loading path is covered by tests.
  - [ ] Unsupported native mode/platform behavior is deterministic and tested.
  - [ ] Built-ins remain available when provider plugin discovery fails or is disabled.

  **QA Scenarios**:
  ```
  Scenario: External-process provider plugin loads successfully
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestProviderExternalPluginLoad -v`
    Expected: Test passes; provider plugin loads, validates, and registers with provenance metadata
    Evidence: .sisyphus/evidence/task-6-provider-external.txt

  Scenario: Unsupported provider native mode fails deterministically
    Tool: Bash
    Steps: Run `go test ./internal/plugin/... -run TestProviderNativePluginUnsupportedMode -v`
    Expected: Test passes with stable unsupported-mode error and no partial registration
    Evidence: .sisyphus/evidence/task-6-provider-native-unsupported.txt
  ```

  **Commit**: YES | Message: `feat(plugin): support provider plugin loading` | Files: `internal/plugin/*, internal/llm/*`

- [ ] T7. Preserve provider config and CLI selection compatibility

  **What to do**:
  1. Keep current config behavior for built-in provider selection fully compatible.
  2. Introduce additive config/CLI fields for plugin-backed provider selection only where necessary.
  3. Define deterministic errors for conflicting provider selection between flags and config.
  4. Ensure help text and config validation distinguish built-in provider IDs from plugin-backed provider IDs without breaking existing flows.
  5. Add CLI and config tests covering built-in default behavior, explicit plugin selection, and invalid selection errors.

  **Must NOT do**: Do not rename existing config keys or require users to rewrite current `zheng.json`. Do not hide errors behind implicit fallback.

  **Recommended Agent Profile**:
  - Category: `unspecified-high` - Reason: Compatibility and user-facing ergonomics are the focus.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - CLI-only behavior.

  **Parallelization**: Can Parallel: YES | Wave 2 | Blocks: T14 | Blocked By: T5, T6

  **References**:
  - Pattern: `README.md` config example and precedence rules.
  - Pattern: `internal/config/config.go` - Current provider configuration behavior.
  - Pattern: `cmd/agent/cli.go` - CLI flag parsing and validation flow.

  **Acceptance Criteria**:
  - [ ] Existing built-in provider config files work without modification.
  - [ ] CLI/config can select plugin-backed provider IDs additively.
  - [ ] Conflicting selection sources produce deterministic user-facing errors.
  - [ ] `go test ./cmd/agent/... -run TestPluginSelection` passes.

  **QA Scenarios**:
  ```
  Scenario: Existing built-in config remains valid
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestBuiltInProviderConfigCompatibility -v`
    Expected: Test passes; old config selects the same built-in provider as before
    Evidence: .sisyphus/evidence/task-7-config-compat.txt

  Scenario: Conflicting CLI and config provider selection errors clearly
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... -run TestProviderSelectionConflict -v`
    Expected: Test passes with deterministic conflict error mentioning both sources
    Evidence: .sisyphus/evidence/task-7-config-conflict.txt
  ```

  **Commit**: YES | Message: `feat(cmd): preserve provider selection compatibility` | Files: `cmd/agent/*, internal/config/*, README.md, docs/USAGE.md`

- [ ] T8. Define verifier plugin contract and predeclared policy registry

  **What to do**:
  1. Define a verifier-family contract that returns host-validated verification results matching the existing domain schema.
  2. Preserve host ownership of valid policy names; plugin verifiers may bind only to predeclared policies.
  3. Add a verifier registry that maps policy identifiers to built-in or plugin-backed implementations with explicit precedence rules.
  4. Register current command/evidence/state-output verifier strategies through the new seam.
  5. Add tests proving unknown policy bindings are rejected and built-in policies remain available with zero plugins installed.

  **Must NOT do**: Do not allow plugins to invent arbitrary policy names. Do not let verifier plugins declare final trust without host schema validation.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Verification authority is a core trust boundary and must remain host-controlled.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T9, T10, T12 | Blocked By: T2, T3, T4

  **References**:
  - API/Type: `internal/domain/ports.go:30-32` - `domain.Verifier` interface.
  - Pattern: `internal/verify/task_aware_verifier.go:11-40` - Current policy map and dispatch behavior.
  - Pattern: `internal/verify/command_verifier.go` - Existing built-in policy implementation.
  - Pattern: `internal/verify/noncoding_verifier.go` - Existing evidence/state-output verification implementations.

  **Acceptance Criteria**:
  - [ ] Verifier registry supports only host-predeclared policy names.
  - [ ] Existing built-in verification strategies register via the new seam.
  - [ ] Unknown or unauthorized policy bindings are rejected with tests.
  - [ ] Zero-plugin operation still resolves current built-in policies.

  **QA Scenarios**:
  ```
  Scenario: Built-in verifier policies resolve through registry
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestVerifierPluginBuiltinsUseRegistry -v`
    Expected: Test passes; command/evidence/state-output policies resolve through host registry
    Evidence: .sisyphus/evidence/task-8-verifier-builtins.txt

  Scenario: Plugin cannot bind unknown policy name
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestVerifierPluginRejectsUnknownPolicyBinding -v`
    Expected: Test passes with deterministic rejection error and no registration
    Evidence: .sisyphus/evidence/task-8-verifier-policy-reject.txt
  ```

  **Commit**: YES | Message: `feat(verify): add verifier policy registry seam` | Files: `internal/verify/*, internal/plugin/*, internal/domain/*`

- [ ] T9. Implement verifier plugin loading and host-side result validation

  **What to do**:
  1. Extend plugin loading for verifier family artifacts using the shared metadata/lifecycle rules.
  2. Add host-side validation for plugin-produced verification results so malformed, incomplete, or contradictory outputs are rejected.
  3. Ensure verifier plugins can participate only through predeclared policy IDs.
  4. Add tests for malformed result schema, timeout/crash behavior, contract mismatch, and fail-closed selection.
  5. Ensure plugin verifier errors preserve debuggable provenance in session/step context.

  **Must NOT do**: Do not permit verifier plugins to bypass evidence expectations or silently mark success without host validation.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Safety and trust semantics dominate this task.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T10, T12 | Blocked By: T8

  **References**:
  - Pattern: `internal/plugin/manager.go:88-127` - Contract validation and registration lifecycle.
  - Pattern: `internal/domain/verification.go` - Result schema that host must validate.
  - Pattern: `internal/verify/*_test.go` - Existing verification tests to extend.

  **Acceptance Criteria**:
  - [ ] Verifier plugins load only after metadata + contract validation.
  - [ ] Host rejects malformed or incomplete verification results with tests.
  - [ ] Timeout/crash behavior is deterministic and fail-closed.
  - [ ] Provenance is attached to verifier execution for debugging/inspect.

  **QA Scenarios**:
  ```
  Scenario: Malformed verifier plugin result is rejected
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestVerifierPluginRejectsMalformedResult -v`
    Expected: Test passes; malformed plugin response is rejected and verification fails closed
    Evidence: .sisyphus/evidence/task-9-verifier-malformed.txt

  Scenario: Verifier plugin timeout fails closed
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestVerifierPluginTimeoutFailsClosed -v`
    Expected: Test passes with deterministic timeout error and preserved provenance context
    Evidence: .sisyphus/evidence/task-9-verifier-timeout.txt
  ```

  **Commit**: YES | Message: `feat(plugin): support verifier plugin loading` | Files: `internal/plugin/*, internal/verify/*`

- [ ] T10. Migrate TaskAwareVerifier to host-dispatched registry-based selection

  **What to do**:
  1. Refactor `TaskAwareVerifier` to select strategies through the host-owned verifier registry instead of the current hardcoded strategy map.
  2. Preserve current category-to-policy default mapping and explicit policy normalization behavior.
  3. Keep built-in defaults authoritative when no plugin binding is configured.
  4. Add tests covering explicit policy selection, category fallback, unknown policy behavior, and plugin-backed policy dispatch.
  5. Ensure strict/standard mode behavior remains compatible unless intentionally changed and documented.

  **Must NOT do**: Do not silently change current category defaults. Do not allow plugin dispatch to override host policy normalization rules.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: This task rewires active verification flow while preserving current semantics.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 3 | Blocks: T14 | Blocked By: T8, T9

  **References**:
  - API/Type: `internal/verify/task_aware_verifier.go:17-84` - Existing dispatch flow and normalization rules.
  - Pattern: `internal/domain/task.go` - Category/default behavior consumed by verifier selection.
  - Pattern: `internal/verify/*_test.go` - Current task-aware verifier tests.

  **Acceptance Criteria**:
  - [ ] `TaskAwareVerifier` uses host registry-based selection.
  - [ ] Existing default policy mapping by task category remains intact.
  - [ ] Plugin-backed policies can be dispatched only through predeclared host policy names.
  - [ ] Existing behavior remains green under updated tests.

  **QA Scenarios**:
  ```
  Scenario: Existing task categories still map to the same default policies
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestTaskAwareVerifierCategoryDefaults -v`
    Expected: Test passes; coding/research/file_workflow/default mappings remain unchanged
    Evidence: .sisyphus/evidence/task-10-verifier-defaults.txt

  Scenario: Plugin-backed predeclared policy dispatch works
    Tool: Bash
    Steps: Run `go test ./internal/verify/... -run TestTaskAwareVerifierDispatchesPluginBackedPolicy -v`
    Expected: Test passes; registered plugin-backed verifier handles a predeclared policy through the registry path
    Evidence: .sisyphus/evidence/task-10-verifier-plugin-dispatch.txt
  ```

  **Commit**: YES | Message: `refactor(verify): dispatch task-aware verifier via registry` | Files: `internal/verify/*`

- [ ] T11. Define strategy-style agent plugin contract with strict host control

  **What to do**:
  1. Define an agent-strategy contract that allows extension of planning/execution strategy while keeping orchestration, cancellation, persistence, and task graph authority in the host.
  2. Specify what context an agent strategy may observe and what decisions it may return.
  3. Prevent recursive agent spawning and unconstrained direct access to session persistence internals.
  4. Register existing built-in runtime strategy through the same seam.
  5. Add contract tests for valid strategy responses, invalid response rejection, and boundary enforcement.

  **Must NOT do**: Do not define agent plugins as full autonomous sub-agents with their own orchestration authority. Do not let strategy plugins write directly to stores or bypass tool/verification guardrails.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Highest ambiguity and highest scope-creep risk; boundary precision is critical.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T12, T13 | Blocked By: T2, T3, T4

  **References**:
  - API/Type: `internal/domain/ports.go:5-10` - Existing model/action planning seam.
  - Pattern: `internal/runtime/engine.go` - Host-owned execution loop that must remain authoritative.
  - Pattern: `internal/orchestration/orchestrator.go` - Host-owned orchestration boundary.
  - Reference: `internal/orchestration/types.go` - DAG model that must remain host-controlled.

  **Acceptance Criteria**:
  - [ ] Agent strategy contract exists with explicit allowed inputs/outputs and prohibited authorities.
  - [ ] Built-in runtime strategy is registered via the same seam.
  - [ ] Invalid strategy responses are rejected deterministically.
  - [ ] Recursive spawn/orchestration take-over is impossible by contract and tested.

  **QA Scenarios**:
  ```
  Scenario: Built-in runtime strategy resolves through seam
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestAgentStrategyBuiltInUsesRegistry -v`
    Expected: Test passes; host executes built-in strategy via registry-based seam
    Evidence: .sisyphus/evidence/task-11-agent-builtins.txt

  Scenario: Invalid strategy response is rejected
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... -run TestAgentStrategyRejectsInvalidResponse -v`
    Expected: Test passes with deterministic validation error and no corrupted session state
    Evidence: .sisyphus/evidence/task-11-agent-invalid.txt
  ```

  **Commit**: YES | Message: `feat(runtime): define agent strategy plugin seam` | Files: `internal/runtime/*, internal/plugin/*, internal/domain/*`

- [ ] T12. Persist plugin provenance and preserve inspect/resume behavior

  **What to do**:
  1. Extend persistence models and stores so plugin-backed provider/verifier/agent executions record provenance additively.
  2. Ensure inspect surfaces provenance even if the original plugin binary is absent.
  3. Ensure resume fails closed with deterministic error if a required plugin is missing/invalid, while preserving the previous persisted session state.
  4. Add migration-safe tests for old records without plugin provenance fields.
  5. Add end-to-end tests covering persisted plugin-backed steps, inspect readability, and resume failure semantics.

  **Must NOT do**: Do not mutate or erase old history when plugin lookup fails. Do not make inspect dependent on successful live plugin loading.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: This task protects recoverability, one of the repo’s core guarantees.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 5 | Blocks: T14 | Blocked By: T2, T3, T4, T6, T9, T11

  **References**:
  - Pattern: `internal/store/session_store.go` - Session persistence path.
  - Pattern: `internal/store/step.go` - Step persistence path.
  - Pattern: `cmd/agent/cli.go` inspect/resume behavior.
  - Reference: `README.md` - inspect/resume continuity guarantees.

  **Acceptance Criteria**:
  - [ ] Plugin provenance is persisted additively for plugin-backed execution records.
  - [ ] Inspect displays provenance without requiring live plugin presence.
  - [ ] Resume fails closed with deterministic error when required plugin is unavailable or invalid.
  - [ ] Old sessions without provenance remain readable.

  **QA Scenarios**:
  ```
  Scenario: Inspect remains readable after plugin artifact disappears
    Tool: Bash
    Steps: Run `go test ./internal/store/... ./cmd/agent/... -run TestInspectReadableWithoutPluginArtifact -v`
    Expected: Test passes; inspect shows stored provenance and history without requiring plugin binary
    Evidence: .sisyphus/evidence/task-12-inspect-readable.txt

  Scenario: Resume fails closed but preserves session state when plugin is missing
    Tool: Bash
    Steps: Run `go test ./internal/store/... ./cmd/agent/... -run TestResumeFailsClosedWhenPluginMissing -v`
    Expected: Test passes with deterministic error and unchanged persisted history
    Evidence: .sisyphus/evidence/task-12-resume-failclosed.txt
  ```

  **Commit**: YES | Message: `feat(store): persist plugin execution provenance` | Files: `internal/store/*, internal/domain/*, cmd/agent/*`

- [ ] T13. Integrate agent strategy plugins into runtime/orchestration without surrendering host authority

  **What to do**:
  1. Wire the agent strategy registry into runtime and orchestration selection points.
  2. Keep host-owned cancellation, worker lifecycle, DAG scheduling, tool execution, and verification dispatch authoritative.
  3. Add tests for plugin-backed strategy execution, cancellation propagation, failure isolation, and no-recursive-spawn enforcement.
  4. Ensure built-in path remains default when no strategy plugin is configured.
  5. Verify that strategy plugin failures do not corrupt session persistence or partial result handling.

  **Must NOT do**: Do not let agent strategies open their own unmanaged workers or bypass bounded concurrency rules. Do not route around verifier or tool safety boundaries.

  **Recommended Agent Profile**:
  - Category: `deep` - Reason: Integration with runtime/orchestration is behaviorally sensitive and concurrency-heavy.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - Not applicable.

  **Parallelization**: Can Parallel: YES | Wave 4 | Blocks: T14 | Blocked By: T11

  **References**:
  - Pattern: `internal/runtime/engine.go` - Main execution loop integration point.
  - Pattern: `internal/orchestration/orchestrator.go` - Multi-agent dispatch authority.
  - Pattern: `internal/orchestration/worker.go` - Worker lifecycle and cancellation path.
  - Reference: `.sisyphus/plans/v2-evolution.md` Wave 4 tasks - Existing orchestration invariants to preserve.

  **Acceptance Criteria**:
  - [ ] Plugin-backed agent strategies can run through host runtime/orchestration selection points.
  - [ ] Cancellation and failure isolation remain intact under plugin-backed strategy execution.
  - [ ] Recursive worker spawning is prevented and covered by tests.
  - [ ] Built-in strategy remains default and compatible.

  **QA Scenarios**:
  ```
  Scenario: Plugin-backed strategy respects host cancellation
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... ./internal/orchestration/... -run TestAgentStrategyPluginCancellationPropagation -v`
    Expected: Test passes; cancellation reaches plugin-backed strategy without leaked workers
    Evidence: .sisyphus/evidence/task-13-agent-cancel.txt

  Scenario: Strategy plugin cannot recursively spawn unmanaged workers
    Tool: Bash
    Steps: Run `go test ./internal/runtime/... ./internal/orchestration/... -run TestAgentStrategyPluginCannotSpawnRecursiveWorkers -v`
    Expected: Test passes with deterministic boundary violation error and preserved host control
    Evidence: .sisyphus/evidence/task-13-agent-recursion-block.txt
  ```

  **Commit**: YES | Message: `feat(orchestration): integrate agent strategy plugins` | Files: `internal/runtime/*, internal/orchestration/*, internal/plugin/*`

- [ ] T14. Run end-to-end extensibility validation and update operator-facing docs

  **What to do**:
  1. Add end-to-end tests that cover one provider path, one verifier path, and one agent-strategy path through the public extension seams.
  2. Add compatibility tests proving built-in-only usage remains unchanged.
  3. Update README, USAGE, validation matrix, and any needed ADR index/docs to explain the v3 extensibility model and fail-closed semantics.
  4. Document what provenance users should expect in inspect output and what errors to expect on resume when plugins are missing.
  5. Verify command examples and config examples remain concrete and do not imply excluded scope such as marketplaces or HTTP APIs.

  **Must NOT do**: Do not publish speculative future roadmap as implemented behavior. Do not remove v2 tool-plugin documentation; update it into v3 context.

  **Recommended Agent Profile**:
  - Category: `writing` - Reason: This task blends integration proof with user-facing docs accuracy.
  - Skills: [] - No special skill required.
  - Omitted: [`playwright`] - No UI/browser requirement.

  **Parallelization**: Can Parallel: NO | Wave 5 | Blocks: F1, F2, F3, F4 | Blocked By: T7, T10, T12, T13

  **References**:
  - Pattern: `README.md` - Existing architecture, config, and non-goal documentation.
  - Pattern: `docs/USAGE.md` - CLI/operator documentation.
  - Pattern: `docs/validation-matrix.md` - Verification evidence ledger.
  - Reference: `.sisyphus/plans/v2-evolution.md` final validation wave expectations.

  **Acceptance Criteria**:
  - [ ] End-to-end tests prove one provider, one verifier, and one agent-strategy path through public seams.
  - [ ] Built-in-only behavior remains green under integration tests.
  - [ ] README/USAGE/validation docs explain selection, provenance, and fail-closed semantics accurately.
  - [ ] Documentation does not imply excluded marketplace/product-surface scope.

  **QA Scenarios**:
  ```
  Scenario: Built-in-only mode still works after v3 extensibility changes
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... ./internal/runtime/... ./internal/verify/... -run TestBuiltInOnlyCompatibility -v`
    Expected: Test passes; existing behavior works with zero plugin artifacts installed
    Evidence: .sisyphus/evidence/task-14-builtins-e2e.txt

  Scenario: Plugin-backed end-to-end flow documents deterministic missing-plugin behavior
    Tool: Bash
    Steps: Run `go test ./cmd/agent/... ./internal/... -run TestPluginBackedFlowAndMissingPluginResumeBehavior -v`
    Expected: Test passes; docs and tests align on provenance visibility and fail-closed resume semantics
    Evidence: .sisyphus/evidence/task-14-plugin-e2e.txt
  ```

  **Commit**: YES | Message: `docs(v3): document core extensibility model` | Files: `README.md, docs/USAGE.md, docs/validation-matrix.md, docs/ADR-*.md`

## Final Verification Wave (MANDATORY — after ALL implementation tasks)
> 4 review agents run in PARALLEL. ALL must APPROVE. Present consolidated results to user and get explicit "okay" before completing.
> **Do NOT auto-proceed after verification. Wait for user's explicit approval before marking work complete.**
> **Never mark F1-F4 as checked before getting user's okay.** Rejection or user feedback -> fix -> re-run -> present again -> wait for okay.
- [ ] F1. Plan Compliance Audit — oracle
- [ ] F2. Code Quality Review — unspecified-high
- [ ] F3. Real Manual QA — unspecified-high (+ playwright if UI)
- [ ] F4. Scope Fidelity Check — deep

## Commit Strategy
- Prefer one commit per completed task or tightly-coupled task pair within a wave.
- Preserve semantic commit scopes: `plugin`, `llm`, `verify`, `runtime`, `store`, `cmd`, `docs`.
- Do not squash provenance/persistence changes into unrelated contract commits.

## Success Criteria
- zheng-harness can execute with built-ins only exactly as before.
- At least one provider path, one verifier path, and one agent-strategy path operate through the same public seam used by plugins.
- Plugin-backed execution records enough provenance for inspect/debugging and deterministic resume failure handling.
- Missing/invalid plugins fail closed for run/resume but do not destroy inspectability of persisted history.
- v3 remains a platform-kernel release, not a product-surface expansion.
