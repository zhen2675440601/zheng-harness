# Learnings - Pre-Commit Doc Consolidation

## 2026-04-27 Task: Doc Consolidation Complete

### Key Learnings

1. **Doc role separation is critical**: README = entrypoint, PROGRESS = authority, USAGE = CLI manual. Avoid overlapping content.

2. **Positioning phrase must be consistent**: Use exact phrase `通用 Agent Harness` across all top-level docs. No variations.

3. **Cross-machine continuation guidance**: Must distinguish repo-tracked (plans/notepads/README/PROGRESS/USAGE) from machine-local (boulder.json/zheng.json/agent.db).

4. **Phase pointers**: README should have concise phase summary + links. PROGRESS should have full progress + "下一步执行入口" section pointing to next plan.

5. **Verification approach for docs**: grep/scan for stale wording + presence check for new phrases + link validation.

### Patterns Used

- Grep for "Coding Agent" → must return empty
- Grep for "通用 Agent Harness" → must return matches in all 3 docs
- Grep for "phase-3-general-task-protocol" → must return matches in README/PROGRESS

### Issues Encountered

- None significant. Subagent timeout on parallel tasks but work completed successfully.

### Recommendations for Phase 3

- Follow similar doc-role separation pattern when adding new docs
- Keep plans repo-tracked via .sisyphus/plans/
- Keep boulder.json machine-local via .gitignore