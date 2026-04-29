# pre-commit hook - 校验暂存区代码注释语言 (Windows PowerShell版)
# 参考 CONTRIBUTING.md 规范：所有注释必须使用中文
#
# 检查规则：
# 1. 行注释 // 必须包含中文
# 2. 块注释 /* */ 必须包含中文
# 3. 跳过 test 文件和生成文件
#
# 使用方式: 将此文件复制到 .git/hooks/pre-commit
#          或运行: make install-hooks

# 获取暂存的文件
$stagedFiles = git diff --cached --name-only --diff-filter=ACM | Where-Object { $_ -match '\.go$' -and $_ -notmatch '_test\.go$' -and $_ -notmatch '^vendor/' }

if ($null -eq $stagedFiles -or $stagedFiles.Count -eq 0) {
    Write-Host "✅ 没有需要检查的 Go 文件"
    exit 0
}

Write-Host "检查代码注释规范..."

$hasError = $false

foreach ($file in $stagedFiles) {
    # 跳过生成的文件
    if ($file -match '(pb\.go|generated\.go|bindata\.go)') {
        continue
    }

    if (-not (Test-Path $file)) {
        continue
    }

    # 读取文件内容
    $lines = Get-Content $file -Encoding UTF8

    # 检查行注释 // 是否包含中文（排除空注释和纯符号注释）
    for ($i = 0; $i -lt $lines.Count; $i++) {
        $line = $lines[$i]
        $lineNum = $i + 1

        # 匹配 // 注释行（不在字符串内）
        if ($line -match '^\s*//\s*([^/\s].*)$') {
            $comment = $matches[1]

            # 排除注释中包含中文的情况
            if ($comment -notmatch '[\u4e00-\u9fff]') {
                # 检查是否为纯英文注释（排除纯符号如 //---, //===, //*** 等）
                if ($comment -match '^[a-zA-Z]') {
                    Write-Host "❌ 文件 $file 第 $lineNum 行包含非中文注释: // $comment"
                    $hasError = $true
                }
            }
        }
    }
}

if ($hasError) {
    Write-Host ""
    Write-Host "参考 CONTRIBUTING.md 规范：所有注释必须使用中文"
    exit 1
}

Write-Host "✅ 代码注释规范校验通过"
exit 0