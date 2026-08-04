# Task 104: Web Dashboard

## 状态

Todo

## 目标

为 AI Gateway 添加一个完整的 Web Dashboard，提供审计日志查看、PII/内容安全检测结果、告警历史、以及配置管理功能。

## 设计规范

- **Spec**: `docs/superpowers/specs/2026-08-05-web-dashboard-design.md`

## 核心页面

1. **Dashboard Overview**: 网关状态、关键指标、最近告警
2. **Audit Logs**: 审计日志搜索和查看
3. **PII Detection**: PII检测结果和统计
4. **Content Safety**: 内容安全检测结果和统计
5. **Alerts**: 告警历史和投递状态
6. **Configuration Management**: 配置管理（模型、审计、检测）

## 配置管理功能（必须）

### 模型配置
- Provider管理（添加/编辑/删除）
- API Key配置（加密存储、掩码显示）
- 模型启用/禁用
- 限流配置

### 审计配置
- 审计开关、日志路径、轮转设置
- 加密开关、密钥管理
- PII审计设置（记录、脱敏）

### 检测配置
- PII检测：开关、类型、处理模式
- 内容安全：开关、类别、阈值
- 自定义关键词库：上传、编辑、删除

## 技术选型

- **Backend**: Go HTTP server（复用网关基础设施）
- **Frontend**: React + Ant Design
- **Data Access**: 直接读取JSONL文件 + config.json
- **Deployment**: 集成到网关（单一二进制、单一端口）

## 实施计划

### 阶段 1：Backend API Core（3天）

**文件**：
- `internal/dashboard/api.go` - API 路由和处理器
- `internal/dashboard/handlers.go` - 各端点的处理逻辑
- `internal/dashboard/log_reader.go` - JSONL 日志读取器
- `internal/dashboard/stats.go` - 统计聚合逻辑

**任务**：
- [ ] 实现 `/api/v1/status` 端点
- [ ] 实现 `/api/v1/stats` 端点
- [ ] 实现 `/api/v1/audit/logs` 端点（带过滤和分页）
- [ ] 实现 `/api/v1/audit/logs/:id` 端点
- [ ] 实现 `/api/v1/pii/*` 端点
- [ ] 实现 `/api/v1/content-safety/*` 端点
- [ ] 实现 `/api/v1/alerts/*` 端点
- [ ] 添加审计日志解密逻辑（如果启用加密）
- [ ] 添加单元测试

### 阶段 2：Backend API Config（2天）

**文件**：
- `internal/dashboard/config_handlers.go` - 配置管理端点
- `internal/dashboard/config_validator.go` - 配置验证逻辑
- `internal/dashboard/config_saver.go` - 配置保存和重载

**任务**：
- [ ] 实现 `/api/v1/config` 端点（获取完整配置）
- [ ] 实现 Provider CRUD 端点
- [ ] 实现 Audit 配置端点
- [ ] 实现 PII 配置端点
- [ ] 实现 Content Safety 配置端点
- [ ] 实现关键词库管理端点
- [ ] 添加配置验证逻辑
- [ ] 实现配置热重载
- [ ] 实现 API Key 加密和掩码
- [ ] 添加单元测试

### 阶段 3：Frontend Core（3天）

**目录**：`web/dashboard/`

**任务**：
- [ ] 初始化 React 项目（Vite）
- [ ] 配置路由和布局（侧边栏 + 内容区）
- [ ] 实现 Dashboard Overview 页面
- [ ] 实现 Audit Logs 页面
- [ ] 添加 API 客户端封装

### 阶段 4：Frontend Features（2天）

**任务**：
- [ ] 实现 PII Detection 页面
- [ ] 实现 Content Safety 页面
- [ ] 实现 Alerts 页面
- [ ] 添加图表和可视化

### 阶段 5：Frontend Configuration（2天）

**任务**：
- [ ] 实现 Configuration 页面布局（3个Tab）
- [ ] 实现 Model Configuration Tab
  - [ ] Provider列表和添加表单
  - [ ] Provider编辑和删除
  - [ ] API Key掩码显示
- [ ] 实现 Audit Configuration Tab
  - [ ] 审计设置表单
  - [ ] 加密密钥管理
- [ ] 实现 Detection Configuration Tab
  - [ ] PII检测设置表单
  - [ ] 内容安全设置表单
  - [ ] 关键词库上传和管理

### 阶段 6：Integration（1天）

**任务**：
- [ ] 将前端构建产物集成到 Go 二进制
- [ ] 在网关启动时添加 Dashboard 路由
- [ ] 添加配置项（`dashboard.enabled`）
- [ ] 端到端测试所有页面
- [ ] 测试配置保存和重载
- [ ] 修复 bug 和边缘情况

### 阶段 7：Documentation（1天）

**任务**：
- [ ] 添加使用文档（`docs/dashboard.md`）
- [ ] 添加配置管理指南
- [ ] 添加安全注意事项
- [ ] 添加故障排查

## API 端点

```
# Status and Stats
GET  /api/v1/status          # 网关状态
GET  /api/v1/stats           # 汇总统计

# Audit Logs
GET  /api/v1/audit/logs      # 审计日志列表（带过滤）
GET  /api/v1/audit/logs/:id  # 审计日志详情

# PII Detection
GET  /api/v1/pii/violations  # PII违规列表
GET  /api/v1/pii/stats       # PII统计

# Content Safety
GET  /api/v1/content-safety/violations  # 内容安全违规列表
GET  /api/v1/content-safety/stats       # 内容安全统计

# Alerts
GET  /api/v1/alerts          # 告警列表
GET  /api/v1/alerts/stats    # 告警统计

# Configuration Management
GET  /api/v1/config          # 获取完整配置
GET  /api/v1/config/providers         # Provider列表
POST /api/v1/config/providers         # 添加Provider
PUT  /api/v1/config/providers/:name   # 更新Provider
DELETE /api/v1/config/providers/:name # 删除Provider
GET  /api/v1/config/audit             # 获取审计配置
PUT  /api/v1/config/audit             # 更新审计配置
POST /api/v1/config/audit/key         # 生成新加密密钥
GET  /api/v1/config/pii               # 获取PII配置
PUT  /api/v1/config/pii               # 更新PII配置
GET  /api/v1/config/content-safety    # 获取内容安全配置
PUT  /api/v1/config/content-safety    # 更新内容安全配置
GET  /api/v1/config/keywords          # 关键词文件列表
POST /api/v1/config/keywords          # 上传关键词文件
DELETE /api/v1/config/keywords/:file  # 删除关键词文件
```

## 配置结构

添加到 `config.json`：

```json
{
  "dashboard": {
    "enabled": true,
    "path": "/dashboard",
    "readonly": false
  }
}
```

## 依赖

- **Go**: 标准库 + 现有网关代码
- **Frontend**: React 18, Ant Design 5, Recharts, Axios
- **Build**: Vite, esbuild

## 安全考虑

- **MVP**: 无认证，仅限信任网络访问
- **推荐**: VPN 或私有 VPC
- **警告**: 请勿直接暴露到公网
- **API Key保护**: 掩码显示、加密存储（可选）
- **配置验证**: 前端+后端双重验证

## 性能目标

- 页面加载: < 2s（首次）, < 500ms（后续）
- API 响应: < 500ms (P99)
- 支持: 10 并发用户（MVP）
- 日志搜索: < 3s（10万条日志）
- 配置保存: < 1s

## 成功标准

- Dashboard 正确显示审计日志数据
- 搜索过滤功能正常
- PII/内容安全结果清晰展示
- 告警历史可见且显示投递状态
- 配置管理功能完整可用
- Provider/Audit/Detection配置可编辑并生效
- 页面加载性能达标
- 支持 Chrome/Firefox/Safari

## 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 前端开发时间超预期 | 延期 | 使用 Ant Design 组件库减少开发量 |
| JSONL 文件过大导致性能问题 | 性能 | 添加分页、时间范围过滤、缓存 |
| 无认证导致安全风险 | 安全 | 文档中明确警告，提供部署建议 |
| 配置保存失败导致服务异常 | 稳定性 | 配置验证、备份机制、回滚功能 |

## 预计时间

**总计**: 14 天（约 3 周）

- 阶段 1: 3 天（Backend API Core）
- 阶段 2: 2 天（Backend API Config）
- 阶段 3: 3 天（Frontend Core）
- 阶段 4: 2 天（Frontend Features）
- 阶段 5: 2 天（Frontend Configuration）
- 阶段 6: 1 天（Integration）
- 阶段 7: 1 天（Documentation）

## 下一步

开始阶段 1：实现 Backend API Core