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
