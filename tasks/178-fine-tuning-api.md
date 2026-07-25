# Task 178 - Fine-tuning API ✅

## 背景

Files API 已就绪，Fine-tuning API 构建完成。

## 端点

| 方法 | 路径 | 状态 |
|---|---|---|
| POST | /v1/fine_tuning/jobs | ✅ |
| GET | /v1/fine_tuning/jobs | ✅ |
| GET | /v1/fine_tuning/jobs/{job_id} | ✅ |
| GET | /v1/fine_tuning/jobs/{job_id}/events | ✅ |
| POST | /v1/fine_tuning/jobs/{job_id}/cancel | ✅ |
| GET | /v1/fine_tuning/jobs/{job_id}/checkpoints | ✅ |

## 变更

- `internal/compat/types.go`：FineTuningJobRequest, FineTuningJob, FineTuningJobEvent, FineTuningJobList, FineTuningJobEventList, FineTuningJobCheckpoint, FineTuningJobCheckpointList
- `internal/provider/provider.go`：6 个新接口方法
- `internal/provider/fake/fake.go`：桩实现
- `internal/provider/openai/finetuning.go`：真实实现 + fineTuningListPath helper
- `internal/provider/azureopenai/finetuning.go`：Azure 实现
- `internal/routes/routes.go`：5 个路由常量 + 动态匹配
- `internal/api/finetuning.go`：6 个处理器 + fineTuningJobID/parseLimit helpers
- `internal/api/server.go`：路由注册
- `internal/api/finetuning_test.go`：10 个测试
- `internal/config/config.go`：fine_tuning 能力
- `schema/config.schema.json`：schema 更新
- `openai-compatible-proxy-spec.md`：Batches/Files/Fine-tuning 文档

## 验证

- `go test ./...` ✅
