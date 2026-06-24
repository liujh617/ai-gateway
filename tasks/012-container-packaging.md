# Task 012: Container Packaging

## 状态

Done

## 背景

网关已经具备 API、配置、日志、metrics 和 runtime hardening。Task 012 增加标准容器化入口和部署文档，让服务可以被构建为镜像并在容器环境中运行。

## 范围

实现：

- `Dockerfile`。
- `.dockerignore`。
- `make build`。
- `make docker-build`。
- `make docker-run`。
- `docs/deployment.md`。
- README 增加部署文档入口。

暂不实现：

- Kubernetes manifests。
- Helm chart。
- Gitee/GitHub CI。
- 镜像发布流水线。

## 验收标准

- `make build` 可以生成本地二进制。
- `make verify` 和 `make smoke` 在 WSL `Ubuntu-24.04` 中通过。
- Dockerfile 不复制本地密钥配置。
- 部署文档说明 fake provider 和真实 upstream 的运行方式。

