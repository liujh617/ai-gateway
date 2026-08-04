# Web Dashboard

AI Gateway 的 Web 管理界面，提供审计日志查看、PII/内容安全检测结果、告警历史和配置管理等功能。

## 技术栈

- **React 18** + TypeScript
- **Ant Design 5** - UI 组件库
- **React Router 6** - 路由管理
- **Recharts** - 数据可视化
- **Axios** - HTTP 客户端
- **Vite** - 构建工具

## 开发

### 安装依赖

```bash
cd web/dashboard
npm install
```

### 启动开发服务器

```bash
npm run dev
```

访问 http://localhost:3001/dashboard/

### 构建生产版本

```bash
npm run build
```

构建产物将输出到 `internal/dashboard/web/` 目录，并嵌入到 Go 二进制中。

## 页面说明

### 1. Dashboard Overview (概览)
- 网关状态和关键指标
- Provider/Model 请求分布图表
- 最近告警列表

### 2. Audit Logs (审计日志)
- 搜索过滤（Provider、Model、状态码、时间范围）
- 分页列表
- 详情查看（PII/内容安全标记）

### 3. PII Detection (PII检测)
- 统计卡片（总数、手机号、身份证、银行卡）
- 类型分布图表
- 违规列表

### 4. Content Safety (内容安全)
- 统计卡片（总数、政治、色情、暴力、广告）
- 类别分布图表
- 违规列表

### 5. Alerts (告警)
- 统计卡片（总数、Webhook、Email、Slack）
- 渠道分布图表
- 告警历史

### 6. Configuration (配置)
- **模型配置**: Provider 添加/编辑/删除、API Key 管理
- **审计配置**: 审计开关、加密设置、PII 审计
- **检测配置**: PII/内容安全检测设置

## API 端点

Dashboard 通过以下 API 端点获取数据：

### Core Endpoints (阶段1)
- `GET /api/v1/status` - 网关状态
- `GET /api/v1/stats` - 统计数据
- `GET /api/v1/audit/logs` - 审计日志列表
- `GET /api/v1/audit/logs/:id` - 审计日志详情
- `GET /api/v1/pii/violations` - PII 违规列表
- `GET /api/v1/pii/stats` - PII 统计
- `GET /api/v1/content-safety/violations` - 内容安全违规
- `GET /api/v1/content-safety/stats` - 内容安全统计
- `GET /api/v1/alerts` - 告警列表
- `GET /api/v1/alerts/stats` - 告警统计

### Config Endpoints (阶段2)
- `GET/POST/PUT/DELETE /api/v1/config/providers` - Provider 管理
- `GET/PUT /api/v1/config/audit` - 审计配置
- `POST /api/v1/config/audit/key` - 生成加密密钥
- `GET/PUT /api/v1/config/pii` - PII 配置
- `GET/PUT /api/v1/config/content-safety` - 内容安全配置
- `GET/POST/DELETE /api/v1/config/keywords` - 关键词管理

## 安全考虑

⚠️ **重要**: MVP 版本没有认证机制

- **请勿**直接暴露到公网
- **推荐**通过 VPN 或私有 VPC 访问
- 配置网络层安全（如防火墙、IP 白名单）

## 实时刷新

所有页面每 30 秒自动刷新数据，可通过修改各页面组件中的 `setInterval` 参数调整。

## 开发说明

### 添加新页面

1. 在 `src/pages/` 创建新的页面组件
2. 在 `src/App.tsx` 添加路由
3. 在 `src/layouts/MainLayout.tsx` 添加菜单项

### 添加新 API

1. 在对应页面组件中使用 `axios` 调用 API
2. 确保后端 API 端点已实现（`internal/dashboard/api.go`）

### 自定义主题

修改 `src/index.css` 或使用 Ant Design 的 ConfigProvider 进行主题定制。

## 故障排查

### 页面空白
- 检查浏览器控制台错误
- 确认 API 端点可访问
- 检查路由配置（basename="/dashboard"）

### API 请求失败
- 检查后端服务是否启动
- 确认 CORS 配置正确
- 查看网络请求详情（浏览器开发者工具）

### 图表不显示
- 检查数据格式是否符合 Recharts 要求
- 确认 API 返回的数据结构正确