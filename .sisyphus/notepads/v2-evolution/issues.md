# v2 Evolution - Issues

## Issues

- Background QA/repository-context review agents intermittently failed to start with `UnknownError`, so local build/test verification was used as the primary completion evidence for this task.
- Repository-wide `go test ./...` initially failed because the existing domain guardrail test matched the literal token `json.RawMessage` in the new streaming event implementation.
- 2026-04-29: F2 code-quality review found one substantive security/design gap in the plugin system: `Runtime.PluginCapabilities` is parsed and `SafetyPolicy.DeclaresPluginCapability()` exists, but plugin loading/execution never enforces declared capabilities, so capability declarations currently provide no runtime protection.
- 2026-04-29: `go test -race ./...` could not be executed in this Windows environment because the installed toolchain reports `-race is not supported on windows/386`; standard `go test ./...` and `go build ./...` did pass.
- 2026-04-29: External-process plugin fixtures and native-plugin stubs had to be updated to advertise capabilities because the strengthened plugin contract now rejects plugins with empty capability declarations whenever runtime policy configures an allowlist.
- 2026-04-30: Targeted plugin-package tests are currently blocked by pre-existing compile errors outside this task scope in `internal/store` (`nullableString` redeclared between `memory_store.go` and `step.go`, plus `session_store.go` has `no new variables on left side of :=`), so task verification relied on clean LSP diagnostics for changed files after the package build stopped upstream.
