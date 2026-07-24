# Task 176 - Files API

## 背景

Files API 是 Fine-tuning、Assistants、Vision 等高级功能的前置基础设施。当前未实现。

## 端点

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | /v1/files | 上传文件（multipart） |
| GET | /v1/files | 列出文件 |
| GET | /v1/files/{file_id} | 获取文件元数据 |
| DELETE | /v1/files/{file_id} | 删除文件 |
| GET | /v1/files/{file_id}/content | 下载文件内容 |

## 变更

- `internal/compat/types.go`：FileObject、FileList、FileDelete 类型
- `internal/provider/provider.go`：UploadFile、ListFiles、RetrieveFile、DeleteFile、DownloadFile
- `internal/provider/fake/fake.go`：桩实现
- `internal/provider/openai/openai.go`：multipart 上传 + JSON 列表/检索/删除
- `internal/provider/azureopenai/azureopenai.go`：同上（Azure 路径）
- `internal/routes/routes.go`：路由常量
- `internal/api/files.go`：处理器
- `internal/api/server.go`：路由注册
- `internal/api/files_test.go`：测试
- `internal/config/config.go`：capability 验证

## 验证

- `go test ./...`
