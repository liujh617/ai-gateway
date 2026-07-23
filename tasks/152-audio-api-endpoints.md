# Task 152 - Audio API Endpoints

## 背景

OpenAI 兼容网关新增 `/v1/audio/transcriptions`、`/v1/audio/translations` 和 `/v1/audio/speech` 三个音频端点。Transcriptions 和 translations 接受 multipart/form-data 上传的音频文件，speech 接受 JSON body 并返回音频数据。

## 变更

### 类型定义 (`internal/compat/types.go`)
- `AudioTranscriptionRequest` / `AudioTranscriptionResponse` — 转录请求/响应
- `AudioTranslationRequest` / `AudioTranslationResponse` — 翻译请求/响应
- `SpeechRequest` / `SpeechResponse` — 语音合成请求/响应
- `ValidateTextOnly()` 用于 multipart 解析后的文本字段校验
- `Validate()` 用于 JSON 反序列化后的完整校验
- `SpeechRequest` 支持 `UnmarshalJSON`/`MarshalJSON` 保留额外字段

### Provider 接口 (`internal/provider/provider.go`)
- `CreateTranscription(ctx, AudioTranscriptionRequest) (*AudioTranscriptionResponse, error)`
- `CreateTranslation(ctx, AudioTranslationRequest) (*AudioTranslationResponse, error)`
- `CreateSpeech(ctx, SpeechRequest) (*SpeechResponse, error)`

### Provider 实现
- **fake** (`internal/provider/fake/fake.go`): 返回硬编码响应，transcription/translation 使用 `ResponseText`，speech 返回 `fake-audio-data`
- **openai** (`internal/provider/openai/openai.go`): 通过 multipart form body 上传音频文件，speech 返回音频二进制数据
- **azureopenai** (`internal/provider/azureopenai/azureopenai.go`): 与 OpenAI 实现一致，使用 Azure 部署端点

### 共享工具 (`internal/provider/httpx/audio.go`)
- `BuildAudioMultipartBody` — 构建 multipart/form-data body，供 openai 和 azureopenai 共用
- `Ftoa` — 格式化 float64 字符串，供 multipart form 字段使用

### 路由 (`internal/routes/routes.go`)
- `/v1/audio/transcriptions` (POST)
- `/v1/audio/translations` (POST)
- `/v1/audio/speech` (POST)

### Handler (`internal/api/audio.go`)
- `handleAudioTranscriptions`: multipart 解析 → 校验 → 路由解析 → 带 fallback 调 provider → audit 记录
- `handleAudioTranslations`: 同上模式
- `handleAudioSpeech`: JSON 解析 → 校验 → 路由解析 → 带 fallback 调 provider → 写音频 response
- `maxAudioUploadBytes = 25 MiB`
- 辅助函数: `parseAudioMultipartForm`, `audioReqSummary`, `speechReqSummary`, `speechRespSummary`

### 配置与 Schema
- 新增 capability: `transcriptions`, `translations`, `speech`
- `schema/config.schema.json` 同步更新 capabilities enum

### 测试 (`internal/api/audio_test.go`)
- 18 个测试覆盖三个端点：happy path、缺失字段、auth、capability 检查、method not allowed、provider 错误、fallback
- 所有测试用 provider stub 均新增三个 audio 方法以满足 Provider 接口

## 验证

- `go test ./...`
- `make verify`
- `make release-check`
