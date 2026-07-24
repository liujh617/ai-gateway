# Task 178 - Fine-tuning API

## 背景

Files API 已就绪，可以构建 Fine-tuning API。允许用户上传训练数据、创建微调任务、监控进度。

## 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /v1/fine_tuning/jobs | 创建微调任务 |
| GET | /v1/fine_tuning/jobs | 列出微调任务（支持 after, limit） |
| GET | /v1/fine_tuning/jobs/{job_id} | 获取任务详情 |
| GET | /v1/fine_tuning/jobs/{job_id}/events | 获取任务事件流（支持 after, limit） |
| POST | /v1/fine_tuning/jobs/{job_id}/cancel | 取消任务 |
| GET | /v1/fine_tuning/jobs/{job_id}/checkpoints | 获取检查点列表 |

## 类型

- `FineTuningJobRequest`：model, training_file, validation_file, hyperparameters, suffix, method
- `FineTuningJob`：id, object, model, created_at, status, training_file, validation_file, hyperparameters, error
- `FineTuningJobEvent`：id, object, created_at, level, message
- `FineTuningJobList`：object, data[], has_more
- `FineTuningJobCheckpointList`：object, data[], has_more

## 变更

- `internal/compat/types.go`：新增类型定义
- `internal/provider/provider.go`：新增接口方法
- `internal/provider/fake/fake.go`：桩实现
- `internal/provider/openai/openai.go`：真实实现
- `internal/provider/azureopenai/azureopenai.go`：Azure 实现
- `internal/routes/routes.go`：路由常量和动态匹配
- `internal/api/finetuning.go`：处理器
- `internal/api/server.go`：路由注册
- `internal/api/finetuning_test.go`：测试
- `internal/config/config.go`：capability 验证
- `schema/config.schema.json`：schema 更新

## 验证

- `go test ./...`
- `make verify`
