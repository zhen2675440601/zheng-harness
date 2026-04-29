.PHONY: fmt lint test test-race test-cover notecheck install-hooks check-commit-msg check-comments

fmt:
	gofmt -w .

lint:
	go vet ./...

test:
	go test ./...

test-race:
	go test -race ./...

test-cover:
	go test -cover ./...

smoke-test:
	go test -tags=smoke ./internal/llm/...

notecheck:
	powershell -ExecutionPolicy Bypass -File ./scripts/check-notepads.ps1

# ============================================================
# Git Hooks - 代码规范自动化
# ============================================================

# 安装 git hooks（将 .githooks 目录链接到 .git/hooks）
install-hooks:
	@echo "安装 git hooks..."
	@mkdir -p .git/hooks
	@cp .githooks/commit-msg .git/hooks/commit-msg
	@chmod +x .git/hooks/commit-msg
	@if [ -f .githooks/pre-commit ]; then \
		cp .githooks/pre-commit .git/hooks/pre-commit; \
		chmod +x .git/hooks/pre-commit; \
	fi
	@echo "✅ git hooks 已安装"
	@echo "   - commit-msg: 校验提交信息格式和语言"
	@echo "   - pre-commit: 校验代码注释语言"

# 卸载 git hooks
uninstall-hooks:
	@echo "卸载 git hooks..."
	@rm -f .git/hooks/commit-msg .git/hooks/pre-commit
	@echo "✅ git hooks 已卸载"

# 检查提交信息规范（可单独运行）
check-commit-msg:
	@echo "检查提交信息规范..."
	@last_msg=$$(git log -1 --pretty=%B | head -1); \
	if echo "$$last_msg" | grep -qE '^(feat|fix|refactor|perf|test|docs|chore|style|ci)(\([^)]+\))?: '; then \
		if echo "$$last_msg" | grep -q '[\u4e00-\u9fff]'; then \
			echo "✅ 提交信息格式正确"; \
		else \
			echo "❌ 提交信息必须使用中文"; \
			exit 1; \
		fi; \
	else \
		echo "❌ 提交信息格式不正确"; \
		echo "正确格式: <类型>(<范围>): <标题>"; \
		exit 1; \
	fi

# 检查代码注释规范（扫描暂存区文件）
check-comments:
	@echo "检查代码注释规范..."
	@staged=$$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$$' | grep -v '_test\.go$$' | grep -v '^vendor/'); \
	if [ -z "$$staged" ]; then \
		echo "✅ 没有需要检查的 Go 文件"; \
		exit 0; \
	fi; \
	for file in $$staged; do \
		if grep -qE '^\s*//[[:space:]]*[a-zA-Z][^/]' "$$file" 2>/dev/null; then \
			echo "❌ 文件 $$file 包含非中文注释"; \
			echo "参考 CONTRIBUTING.md 规范：所有注释必须使用中文"; \
			exit 1; \
		fi; \
	done; \
	echo "✅ 代码注释规范校验通过"

# 运行所有检查（lint + 规范检查）
check: lint check-commit-msg check-comments
	@echo ""
	@echo "✅ 所有检查通过"
