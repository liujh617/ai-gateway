# Shell Script Line Endings Design

## 背景

项目将 WSL `Ubuntu-24.04` 作为标准测试环境，但 Windows Git 在 `core.autocrlf=true` 时会把
checkout 中的 shell 脚本转换为 CRLF。Bash 随后把 `set -euo pipefail` 解析为包含 `\r` 的参数，
导致 `make release-check` 在 `scripts/check-config-examples.sh` 启动时失败：

```text
set: pipefail\r: invalid option name
```

该问题已在主工作区和隔离 worktree 中重复出现，且必须先手工把 `scripts/*.sh` 转为 LF 才能
完成 WSL 验证。仓库需要独立于个人 Git 配置的换行符契约。

## 目标

- 强制仓库内所有当前及未来 `*.sh` 文件在工作树使用 LF。
- 不受开发者本机 `core.autocrlf` 设置影响。
- 自动检查受版本控制的 shell 脚本不包含 CRLF。
- 让全新 Windows Git worktree 无需手工转换即可在 WSL 执行 `make release-check`。
- 失败时给出明确文件名和修复方向。

## 非目标

- 不规范 `Makefile`、`*.go`、`*.md`、JSON 或其他文本文件的换行符。
- 不修改用户或系统级 Git 配置。
- 不改变现有 shell 脚本行为。
- 不处理 WSL 服务偶发的 `WSL_E_DISTRO_NOT_FOUND` 启动错误。

## Git 属性

在仓库根目录新增 `.gitattributes`：

```gitattributes
*.sh text eol=lf
```

`text` 让 Git 对 blob 做文本规范化，`eol=lf` 强制受匹配文件在工作树使用 LF。根级规则适用于
`scripts/` 以及未来其他目录中的 shell 脚本。

添加属性后，对当前受版本控制的 `*.sh` 执行一次 `git add --renormalize`。预期 Git blob 已是
LF，因此 renormalize 不应产生脚本语义 diff；若出现 diff，只允许换行符规范化，不允许命令内容变化。

## 自动检查

新增 `scripts/check-line-endings.sh`，使用 `git ls-files -z -- '*.sh'` 枚举受版本控制的 shell
脚本，而不是扫描未跟踪文件或构建输出。脚本逐个检查文件是否包含 carriage return byte `\r`。

行为：

- 所有文件为 LF 时输出 `line-endings-ok` 并返回 0。
- 发现 CRLF 时为每个文件输出 `<path>: shell script must use LF line endings`，最终返回非零。
- 文件名通过 NUL 分隔读取，正确处理空格。
- 只依赖 Bash、Git 和标准 Unix 工具；不引入 Python 或第三方程序。
- Git 枚举失败必须使检查失败，不能因 process substitution 或 subshell 吞掉退出状态。
- Windows Git 创建的 linked worktree 可能包含 WSL Git 无法解析的 Windows `.git` 指针；此时
  检查器在可用时回退到 `git.exe ls-files`。Linux CI 仍直接使用 `git`。

检查脚本自身也匹配 `.gitattributes` 规则。

## Makefile 接入

新增：

```make
check-line-endings:
	bash scripts/check-line-endings.sh
```

将 `check-line-endings` 加入 `.PHONY`，并让 `verify` 在 `fmt` 前执行该检查：

```make
verify: check-line-endings fmt test race vet
```

检查必须在 `fmt` 前执行，避免其他写操作掩盖 checkout 中的换行符问题。`release-check` 已依赖
`verify`，因此 GitHub Actions、Gitee CI 和本地完整验证自动获得该检查。

## 测试策略

为检查脚本增加自测试 `scripts/check-line-endings-test.sh`。测试在临时 Git 仓库中复制检查脚本，
分别创建：

1. LF shell 文件，预期检查成功并输出 `line-endings-ok`。
2. CRLF shell 文件，预期检查失败并包含文件路径和 `must use LF line endings`。
3. `git` 与 `git.exe` 都无法枚举文件，预期检查失败并输出稳定错误。

测试通过环境变量向检查脚本传入临时仓库位置，或在临时仓库内直接执行复制后的脚本；生产检查
默认仍检查当前 Git 工作树。测试不得修改真实仓库中的脚本换行符。

`make check-line-endings` 运行生产检查；脚本自测试由 `make test-line-endings` 调用，并纳入
`verify`。最终依赖顺序为：

```make
verify: check-line-endings test-line-endings fmt test race vet
```

## 文档与任务同步

- 更新 `docs/testing-environment.md`：说明 `.gitattributes` 强制 `*.sh` 使用 LF，且无需关闭全局
  `core.autocrlf`。
- 更新 `docs/ci.md`：在 `release-check` 组成中加入换行符检查。
- 新增 `tasks/155-shell-line-endings.md`。

该变更不涉及外部 API 契约，无需更新兼容 spec 或 architecture 文档。

## 验证

最终验证必须从应用新 `.gitattributes` 后创建的全新 worktree 执行，不能在已经手工转换过脚本的
工作树中冒充成功。验证命令：

```bash
make check-line-endings
make test-line-endings
make release-check
```

验收条件：

- `git check-attr text eol -- scripts/smoke-azure.sh` 显示 `text: set`、`eol: lf`。
- 全新 worktree 中所有受版本控制的 `*.sh` 不含 `\r`。
- 检查脚本的 LF 正例通过、CRLF 反例失败。
- `make release-check` 无需任何手工换行转换即可通过。
- diff 不包含非 shell 文件的批量换行符变化。

## 风险与控制

- `git add --renormalize` 可能制造大 diff。提交前使用 `git diff --word-diff=porcelain` 和
  `git diff --ignore-space-at-eol` 确认没有命令内容变化。
- 检查脚本若扫描未跟踪文件会产生环境噪声，因此只使用 `git ls-files`。
- 将来其他文件类型仍可能遇到 CRLF 问题，但本任务严格限制为 `*.sh`，后续按实际证据扩展。
