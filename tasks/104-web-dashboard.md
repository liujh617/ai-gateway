# Task 104: Web Dashboard

## 状态

Todo

## 目标

为 AI Gateway 添加一个简单的 Web Dashboard，提供审计日志查看、PII/内容安全检测结果、告警历史等可视化界面。

## 设计规范

- **Spec**: `docs/superpowers/specs/2026-08-05-web-dashboard-design.md`

## 核心页面

1. **Dashboard Overview**: 网关状态、关键指标、最近告警
2. **Audit Logs**: 审计日志搜索和查看
3. **PII Detection**: PII检测结果和统计
4. **Content Safety**: 内容安全检测结果和统计
5. **Alerts**: 告警历史和投递状态

## 技术选型

- **Backend**: Go HTTP server（复用网关基础设施）
- **Frontend**: React + Ant Design
- **Data Access**: 直接读取 JSONL 文件（无需数据库）
- **Deployment**: 集成到网关（单一二进制、单一端口）

## 实施计划

### 阶段 1：Backend API（3天）

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
- [ ] 实现 `/api/v1/pii/violations` 和 `/api/v1/pii/stats` 端点
- [ ] 实现 `/api/v1/content-safety/*` 端点
- [ ] 实现 `/api/v1/alerts/*` 端点
- [ ] 实现 `/api/v1/config` 端点
- [ ] 添加审计日志解密逻辑（如果启用加密）
- [ ] 添加单元测试

### 阶段 2：Frontend Core（3天）

**目录**：`web/dashboard/`

**任务**：
- [ ] 初始化 React 项目（Vite）
- [ ] 配置路由和布局（侧边栏 + 内容区）
- [ ] 实现 Dashboard Overview 页面
  - [ ] 网关状态卡片
  - [ ] 关键指标卡片
  - [ ] 最近告警列表
- [ ] 实现 Audit Logs 页面
  - [ ] 搜索过滤表单
  - [ ] 结果表格（分页）
  - [ ] 详情模态框
- [ ] 添加 API 客户端封装

### 阶段 3：Frontend Features（2天）

**任务**：
- [ ] 实现 PII Detection 页面
  - [ ] 统计卡片
  - [ ] 趋势图表
  - [ ] 违规列表
- [ ] 实现 Content Safety 页面
  - [ ] 统计卡片
  - [ ] 趋势图表
  - [ ] 违规列表
- [ ] 实现 Alerts 页面
  - [ ] 统计卡片
  - [ ] 趋势图表
  - [ ] 告警列表

### 阶段 4：Integration（1天）

**任务**：
- [ ] 将前端构建产物集成到 Go 二进制
- [ ] 在网关启动时添加 Dashboard 路由
- [ ] 添加配置项（`dashboard.enabled`）
- [ ] 端到端测试所有页面
- [ ] 修复 bug 和边缘情况

### 阶段 5：Documentation（1天）

**任务**：
- [ ] 添加使用文档（`docs/dashboard.md`）
- [ ] 添加部署指南
- [ ] 添加安全注意事项
- [ ] 添加故障排查

## 配置结构

添加到 `config.json`：

```json
{
  "dashboard": {
    "enabled": true,
    "path": "/dashboard",
    "readonly": true
  }
}
```

## API 端点

```
GET  /api/v1/status          # 网关状态和健康检查
GET  /api/v1/stats           # 汇总统计
GET  /api/v1/audit/logs      # 审计日志列表（带过滤）
GET  /api/v1/audit/logs/:id  # 审计日志详情
GET  /api/v1/pii/violations  # PII 违规列表
GET  /api/v1/pii/stats       # PII 统计
GET  /api/v1/content-safety/violations  # 内容安全违规列表
GET  /api/v1/content-safety/stats       # 内容安全统计
GET  /api/v1/alerts          # 告警列表
GET  /api/v1/alerts/stats    # 告警统计
GET  /api/v1/config          # 获取当前配置
```

## 依赖

- **Go**: 标准库 + 现有网关代码
- **Frontend**: React 18, Ant Design 5, Recharts, Axios
- **Build**: Vite, esbuild

## 安全考虑

- **MVP**: 无认证，仅限信任网络访问
- **推荐**: VPN 或私有 VPC
- **警告**: 请勿直接暴露到公网

## 性能目标

- 页面加载: < 2s（首次）, < 500ms（后续）
- API 响应: < 500ms (P99)
- 支持: 10 并发用户（MVP）
- 日志搜索: < 3s（10万条日志）

## 成功标准

- Dashboard 正确显示审计日志数据
- 搜索过滤功能正常
- PII/内容安全结果清晰展示
- 告警历史可见且显示投递状态
- 页面加载性能达标
- 支持 Chrome/Firefox/Safari

## 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 前端开发时间超预期 | 延期 | 使用 Ant Design 组件库减少开发量 |
| JSONL 文件过大导致性能问题 | 性能 | 添加分页、时间范围过滤、缓存 |
| 无认证导致安全风险 | 安全 | 文档中明确警告，提供部署建议 |

## 预计时间

**总计**: 10 天（2 周）

- 阶段 1: 3 天
- 阶段 2: 3 天
- 阶段 3: 2 天
- 阶段 4: 1 天
- 阶段 5: 1 天

## 下一步

开始阶段 1：实现 Backend API