# Phase 3: General Task Protocol - Decisions

## Task 9 Code Search Tool Decisions
- Added a separate `code_search` builtin instead of modifying `grep_search`, preserving the existing general text-search contract while introducing language-aware code-specific filtering.
- Used JSON input for `code_search` (`pattern`, `language`, `output_mode`, `max_results`) to match newer structured tool adapters such as `web_fetch` and `ask_user` without changing `ToolCall.Input` away from string.
- Default exclusions for `code_search` skip `.git`, `.sisyphus`, `vendor`, `node_modules`, minified assets, and binary files before regex evaluation so source-oriented search avoids noisy or unsafe non-code paths.
