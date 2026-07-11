# Shell Script Line Endings Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 通过仓库级 Git 属性和自动检查，确保所有受版本控制的 `*.sh` 在 Windows checkout 后仍使用 LF，使 WSL `make release-check` 无需手工转换即可运行。

**Architecture:** 根目录 `.gitattributes` 用 `*.sh text eol=lf` 固化工作树契约；一个 Bash 检查器通过 `git ls-files -z` 只扫描受版本控制的 shell 脚本并拒绝 carriage return。检查器有独立临时 Git 仓库自测试，并在 `make verify` 最前执行。

**Tech Stack:** Git attributes、Bash、Git CLI、grep、Make、WSL `Ubuntu-24.04`。

## Global Constraints

- 只规范 `*.sh`，不添加其他文件类型的换行符规则。
- 不修改用户或系统级 `core.autocrlf`。
- 不改变现有 shell 脚本命令语义。
- 不新增第三方依赖。
- 最终验收必须在应用 `.gitattributes` 后创建的全新 worktree 中执行。
- 标准验证环境为 WSL `Ubuntu-24.04`。

---

### Task 1: 以 TDD 增加 shell 换行符检查器

**Files:**
- Create: `scripts/check-line-endings-test.sh`
- Create: `scripts/check-line-endings.sh`

**Interfaces:**
- Consumes: 当前 Git 工作树中 `git ls-files -z -- '*.sh'` 返回的路径。
- Produces: 成功输出 `line-endings-ok`；失败输出 `<path>: shell script must use LF line endings` 并返回非零。

- [ ] **Step 1: 先创建检查器自测试**

创建 `scripts/check-line-endings-test.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail

root="$(pwd)"
checker="$root/scripts/check-line-endings.sh"
tmpdir="$(mktemp -d)"

cleanup() {
  rm -rf "$tmpdir"
}
trap cleanup EXIT

git -C "$tmpdir" init -q

printf '#!/usr/bin/env bash\necho ok\n' >"$tmpdir/good.sh"
git -C "$tmpdir" add good.sh

good_output="$(cd "$tmpdir" && bash "$checker")"
grep -q '^line-endings-ok$' <<<"$good_output"

printf '#!/usr/bin/env bash\r\necho bad\r\n' >"$tmpdir/bad.sh"
git -C "$tmpdir" add bad.sh

if bad_output="$(cd "$tmpdir" && bash "$checker" 2>&1)"; then
  echo "check-line-endings unexpectedly accepted CRLF" >&2
  exit 1
fi
grep -q '^bad.sh: shell script must use LF line endings$' <<<"$bad_output"

mkdir "$tmpdir/bin"
printf '#!/usr/bin/env bash\nexit 1\n' >"$tmpdir/bin/git"
printf '#!/usr/bin/env bash\nexit 1\n' >"$tmpdir/bin/git.exe"
chmod +x "$tmpdir/bin/git" "$tmpdir/bin/git.exe"

if git_error_output="$(cd "$tmpdir" && PATH="$tmpdir/bin:$PATH" bash "$checker" 2>&1)"; then
  echo "check-line-endings ignored git ls-files failure" >&2
  exit 1
fi
grep -q '^unable to list tracked shell scripts$' <<<"$git_error_output"

echo "line-endings-test-ok"
```

该测试不会修改真实仓库，只在临时 Git 仓库中创建 LF/CRLF fixture。

- [ ] **Step 2: 运行自测试并确认红灯**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "bash scripts/check-line-endings-test.sh"
```

Expected: FAIL，错误包含 `scripts/check-line-endings.sh: No such file or directory`。

- [ ] **Step 3: 实现最小检查器**

创建 `scripts/check-line-endings.sh`：

```bash
#!/usr/bin/env bash
set -euo pipefail

files="$(mktemp)"
errors="$(mktemp)"
cleanup() {
  rm -f "$files" "$errors"
}
trap cleanup EXIT

if git ls-files -z -- '*.sh' >"$files" 2>"$errors"; then
  :
elif command -v git.exe >/dev/null 2>&1 && git.exe ls-files -z -- '*.sh' >"$files" 2>"$errors"; then
  :
else
  echo "unable to list tracked shell scripts" >&2
  exit 1
fi

failed=0
while IFS= read -r -d '' file; do
  if LC_ALL=C grep -q $'\r' "$file"; then
    echo "$file: shell script must use LF line endings" >&2
    failed=1
  fi
done <"$files"

if [ "$failed" -ne 0 ]; then
  exit 1
fi

echo "line-endings-ok"
```

不得使用 pipe 连接 `git ls-files` 和 `while`，否则 `failed` 会在 subshell 中丢失。

- [ ] **Step 4: 运行自测试并确认绿灯**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "bash scripts/check-line-endings-test.sh"
```

Expected: exit code `0`，输出 `line-endings-test-ok`。

- [ ] **Step 5: 提交检查器红绿循环**

```powershell
git add scripts/check-line-endings.sh scripts/check-line-endings-test.sh
git commit -m "test: add shell line ending check"
```

---

### Task 2: 固化 Git 属性并接入验证入口

**Files:**
- Create: `.gitattributes`
- Modify: `Makefile`
- Modify: `docs/testing-environment.md`
- Modify: `docs/ci.md`
- Create: `tasks/155-shell-line-endings.md`
- Renormalize: all tracked `*.sh` files, with no command-content changes

**Interfaces:**
- Consumes: Task 1 的 `scripts/check-line-endings.sh` 与 `scripts/check-line-endings-test.sh`。
- Produces: `make check-line-endings`、`make test-line-endings`；`verify` 在 Go 格式化和测试前执行两项检查。

- [ ] **Step 1: 新增仅限 shell 的 Git 属性**

创建根目录 `.gitattributes`，内容精确为：

```gitattributes
*.sh text eol=lf
```

Run:

```powershell
git check-attr text eol -- scripts/smoke-azure.sh
```

Expected:

```text
scripts/smoke-azure.sh: text: set
scripts/smoke-azure.sh: eol: lf
```

- [ ] **Step 2: renormalize 所有受版本控制的 shell 脚本并审查**

Run:

```powershell
git add --renormalize -- ':(glob)**/*.sh'
git diff --cached --ignore-space-at-eol --stat
git diff --cached --word-diff=porcelain -- "*.sh"
```

Expected:

- `--ignore-space-at-eol --stat` 不显示现有脚本的命令内容变化。
- `--word-diff=porcelain` 不显示命令 token 增删。
- 新增的两个检查脚本按正常新文件显示。
- 如发现任何非换行符内容变化，停止并调查，不继续提交。

- [ ] **Step 3: 将检查接入 Makefile**

在 `.PHONY` 中加入 `check-line-endings` 和 `test-line-endings`：

```make
.PHONY: fmt check-line-endings test-line-endings test race vet verify build run check-config check-config-examples smoke smoke-rate-limit smoke-azure smoke-deepseek smoke-deepseek-skip release-check docker-build docker-run
```

在 `fmt` 前新增：

```make
check-line-endings:
	bash scripts/check-line-endings.sh

test-line-endings:
	bash scripts/check-line-endings-test.sh
```

将 `verify` 改为：

```make
verify: check-line-endings test-line-endings fmt test race vet
```

- [ ] **Step 4: 运行新的聚焦检查**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make check-line-endings && make test-line-endings"
```

Expected:

```text
line-endings-ok
line-endings-test-ok
```

- [ ] **Step 5: 更新测试环境和 CI 文档**

在 `docs/testing-environment.md` 增加“Shell 脚本换行符”小节，明确：

````markdown
## Shell 脚本换行符

仓库通过根目录 `.gitattributes` 强制所有 `*.sh` 使用 LF。Windows Git 即使启用了
`core.autocrlf=true`，也不应把 shell 脚本 checkout 为 CRLF；无需修改用户的全局 Git 配置。

检查命令：

```bash
make check-line-endings
make test-line-endings
```
````

在 `docs/ci.md` 的 `make release-check` 组成列表中加入：

```markdown
- `make check-line-endings`
- `make test-line-endings`
```

并说明两项检查分别验证受版本控制的 shell 文件和检查器的 LF/CRLF 正反例。

- [ ] **Step 6: 新增 Task 155**

创建 `tasks/155-shell-line-endings.md`：

```markdown
# Task 155 - Shell Script Line Endings

## 状态

Done.

## 背景

Windows Git 在 `core.autocrlf=true` 时可能把 shell 脚本 checkout 为 CRLF，导致 WSL Bash
将 `pipefail\r` 解析为非法选项并阻断 `make release-check`。

## 变更

- 根目录 `.gitattributes` 强制 `*.sh text eol=lf`。
- 新增受版本控制 shell 脚本的 CRLF 检查器及 LF/CRLF 自测试。
- 新增 `make check-line-endings` 和 `make test-line-endings`，并接入 `make verify`。
- 同步测试环境和 CI 文档。

## 验收

- `make check-line-endings`
- `make test-line-endings`
- 在全新 worktree 中执行 `make release-check`
```

- [ ] **Step 7: 运行当前 worktree 的完整验证并提交**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway -- bash -lc "make release-check"
git diff --check
git status --short
```

Expected:

- `make release-check` exit code `0`，开头包含 `line-endings-ok` 和 `line-endings-test-ok`。
- Azure smoke 继续输出 `smoke-azure-ok`。
- diff 只包含 `.gitattributes`、两个检查脚本、Makefile、两份文档、Task 155，以及必要的 `*.sh`
  换行符规范化；不存在脚本命令语义变化。

Commit:

```powershell
git add .gitattributes Makefile docs/testing-environment.md docs/ci.md tasks/155-shell-line-endings.md scripts
git commit -m "build: enforce LF for shell scripts"
```

---

### Task 3: 从全新 worktree 验收 checkout 行为

**Files:**
- No repository content changes expected

**Interfaces:**
- Consumes: Task 2 已提交的 `.gitattributes` 和验证入口。
- Produces: Windows Git 新 checkout 直接可用于 WSL 的验收证据。

- [ ] **Step 1: 从实现提交创建全新验证 worktree**

从主 worktree 外执行：

```powershell
git worktree add .worktrees/shell-line-endings-verify --detach HEAD
```

不得复用实施 worktree，因为其中的脚本可能在开发期间被手工规范化过。

- [ ] **Step 2: 检查新 worktree 的属性和实际字节**

Run:

```powershell
git -C .worktrees/shell-line-endings-verify check-attr text eol -- scripts/smoke-azure.sh
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/shell-line-endings-verify -- bash -lc "make check-line-endings"
```

Expected:

- Git 显示 `text: set` 和 `eol: lf`。
- `make check-line-endings` 输出 `line-endings-ok`。

- [ ] **Step 3: 在全新 worktree 运行完整 release-check**

Run:

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/ai-gateway/.worktrees/shell-line-endings-verify -- bash -lc "make release-check"
```

Expected: exit code `0`；不执行任何预先的 PowerShell 换行符转换。

- [ ] **Step 4: 清理验证 worktree**

```powershell
git worktree remove .worktrees/shell-line-endings-verify
git worktree prune
```

确认实施分支工作区仍干净。
