# Task 149 - Ignore Agent Audit Output

## 背景

Agent audit JSONL 模式会在本地写入完整请求体、响应体、流式 chunk 和错误响应。这些文件可能包含 prompt、completion、tool schema、embedding input 和 embedding vector。

## 变更

- `.gitignore` 增加 `audit/`，避免默认审计输出目录被误提交。

## 验证

- `git status --short`

