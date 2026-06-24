# Testing Environment

本项目的标准测试与验证环境定义为 WSL 中的 `Ubuntu-24.04`。

## 标准环境

- Host OS: Windows
- WSL distro: `Ubuntu-24.04`
- Shell: bash
- Working directory: Windows 仓库路径在 WSL 中的挂载路径，例如 `/mnt/e/code/open-ai-gateway`
- Go runtime: 使用 Ubuntu-24.04 内安装的 Go 工具链，当前基线为 Go 1.22

除非任务明确说明，否则所有构建、测试、lint、集成验证和手工 curl 验证都应在 `Ubuntu-24.04` 中执行。

## 进入环境

从 Windows PowerShell 进入项目目录：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway
```

或直接执行单条命令：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "go test ./..."
```

## 基础验证命令

```bash
go version
go test ./...
go test -race ./...
go vet ./...
```

标准验证入口：

```bash
make test
make verify
```

其中 `make verify` 应至少包含格式检查、单元测试、race 测试和 vet。

## 服务验证

启动本地网关时，默认监听 `127.0.0.1`，避免开发环境中意外暴露端口。

示例：

```bash
go run ./cmd/gateway
```

非流式 chat completions 验证：

```bash
curl -sS http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

流式 chat completions 验证：

```bash
curl -N http://127.0.0.1:8080/v1/chat/completions \
  -H "Authorization: Bearer test-gateway-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "test-model",
    "stream": true,
    "messages": [
      {"role": "user", "content": "hello"}
    ]
  }'
```

## Agent 执行约束

Coding agent 在验证本项目时应优先使用：

```powershell
wsl.exe -d Ubuntu-24.04 --cd /mnt/e/code/open-ai-gateway -- bash -lc "<command>"
```

不要把 Windows PowerShell 作为 Go 测试的标准环境。PowerShell 可以用于文件查看和轻量目录检查，但最终测试结果以 WSL `Ubuntu-24.04` 为准。

## 结果记录

提交或交付实现时，应说明实际执行过的验证命令，例如：

```text
Verified in WSL Ubuntu-24.04:
- go test ./...
- go test -race ./...
- go vet ./...
```

如果因为本地环境缺少 Go、WSL distro 不存在或依赖不可用而无法验证，必须在交付说明中明确写出原因。
