# AI Gateway Dashboard

Web Dashboard 为 AI Gateway 提供可视化管理界面，用于查看审计日志、PII/内容安全检测结果、告警历史和配置管理。

## 功能概览

### 核心功能

- **Dashboard Overview**: 网关状态、关键指标、请求分布图表
- **Audit Logs**: 审计日志搜索、过滤、分页、详情查看
- **PII Detection**: PII 统计、类型分布、违规列表
- **Content Safety**: 内容安全统计、类别分布、违规列表
- **Alerts**: 告警统计、渠道分布、告警历史
- **Configuration**: Provider/Audit/Detection 配置管理

### 配置管理

- **模型配置**: Provider CRUD、API Key 管理、模型配置
- **审计配置**: 审计开关、加密设置、PII 审计选项
- **检测配置**: PII/内容安全检测设置、关键词管理

---

## 快速开始

### 前置条件

- Go 1.21+
- Node.js 18+ (仅开发时需要)
- AI Gateway 服务运行中

### 访问 Dashboard

Dashboard 集成在 AI Gateway 中，访问地址：

```
http://<gateway-host>:<gateway-port>/dashboard/
```

默认：`http://localhost:8080/dashboard/`

---

## 页面说明

### 1. Dashboard Overview (概览)

**功能**：
- 显示网关关键指标（总请求数、成功请求、PII 违规、内容安全违规）
- Provider 请求分布图表
- Model 请求分布图表

**实时刷新**：每 30 秒自动更新数据

---

### 2. Audit Logs (审计日志)

**功能**：
- 搜索过滤：
  - Provider 选择（OpenAI/Anthropic/Azure）
  - Model 名称
  - 状态码（200/400/401/429/500）
  - 时间范围
- 分页显示（默认 20 条/页）
- 详情查看：
  - Request ID
  - Provider/Model
  - Method/Path
  - 状态码、耗时
  - PII/内容安全标记

**使用场景**：
- 追踪特定请求
- 分析失败请求
- 查看 PII/内容安全违规详情

---

### 3. PII Detection (PII检测)

**功能**：
- 统计卡片：
  - 总违规数
  - 手机号违规数
  - 身份证违规数
  - 银行卡违规数
- 类型分布饼图
- 最近违规列表（显示类型、处理模式、样本）

**实时刷新**：每 30 秒自动更新数据

---

### 4. Content Safety (内容安全)

**功能**：
- 统计卡片：
  - 总违规数
  - 政治违规数
  - 色情违规数
  - 暴力违规数
  - 广告违规数
- 类别分布饼图
- 最近违规列表（显示类别、严重级别、样本）

**实时刷新**：每 30 秒自动更新数据

---

### 5. Alerts (告警)

**功能**：
- 统计卡片：
  - 总告警数
  - Webhook 告警数
  - Email 告警数
  - Slack 告警数
  - 告警成功率
- 渠道分布饼图
- 最近告警列表（显示来源、严重级别、渠道、状态）

**实时刷新**：每 30 秒自动更新数据

---

### 6. Configuration (配置)

#### 6.1 模型配置

**功能**：
- 查看所有 Provider 列表
- 添加新 Provider：
  - 名称
  - Base URL
  - API Key
  - 默认模型
- 编辑 Provider：
  - 修改 Base URL
  - 更新 API Key（留空保持不变）
  - 调整默认模型
- 删除 Provider（需确认）

**API Key 安全**：
- 显示为掩码格式：`sk-****abcd`
- 编辑时可选择保持不变（留空）
- 保存后可能需要重启网关

---

#### 6.2 审计配置

**功能**：
- 审计开关
- 审计日志路径
- 最大文件大小（bytes）
- 加密设置：
  - 加密开关
  - 密钥路径（只读）
  - 生成新密钥（需确认）
- PII 审计设置：
  - 记录 PII
  - 脱敏 PII

**热重载**：
- 审计路径、文件大小：支持热重载
- 加密开关：需要重启

---

#### 6.3 检测配置

**PII 检测**：
- 启用开关
- 处理模式（alert/reject/allow）
- 检测类型：
  - 手机号
  - 身份证
  - 银行卡

**内容安全检测**：
- 启用开关
- 处理模式（alert/reject/allow）
- 阈值级别（low/medium/high）

**热重载**：所有检测配置支持热重载

---

## 部署指南

### MVP 部署（简单）

Dashboard 集成在网关中，无需单独部署：

1. 启动 AI Gateway：
   ```bash
   ./gateway -config config.json
   ```

2. 访问 Dashboard：
   ```
   http://localhost:8080/dashboard/
   ```

### 生产部署

**安全要求**：

⚠️ **重要**: Dashboard 没有内置认证机制

- **禁止**直接暴露到公网
- **必须**通过以下方式保护：
  - VPN 访问
  - 私有 VPC 部署
  - 反向代理 + 认证（如 Nginx + Basic Auth）
  - IP 白名单

**推荐架构**：

```
Internet
    ↓
[Firewall/IP Whitelist]
    ↓
[Reverse Proxy with Auth]
    ↓
[AI Gateway Dashboard]
```

---

## 安全最佳实践

### 1. 网络层安全

- 使用防火墙限制访问 IP
- 仅允许可信网络访问
- 使用 VPN 或专线连接

### 2. 反向代理认证

使用 Nginx 添加 Basic Auth：

```nginx
location /dashboard/ {
    auth_basic "AI Gateway Dashboard";
    auth_basic_user_file /etc/nginx/.htpasswd;
    proxy_pass http://localhost:8080;
}
```

生成密码文件：

```bash
sudo htpasswd -c /etc/nginx/.htpasswd admin
```

### 3. HTTPS 加密

使用 TLS 证书加密传输：

```nginx
server {
    listen 443 ssl;
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location /dashboard/ {
        proxy_pass http://localhost:8080;
    }
}
```

---

## 故障排查

### 问题：页面空白

**可能原因**：
1. API 端点不可访问
2. 路由配置错误
3. 前端构建失败

**解决方法**：
1. 检查浏览器控制台错误
2. 确认 API 端点：`GET /api/v1/status`
3. 检查路由 basename 配置

---

### 问题：API 请求失败

**可能原因**：
1. 后端服务未启动
2. CORS 配置错误
3. API 端点未实现

**解决方法**：
1. 检查网关日志
2. 确认 CORS 配置：
   ```json
   {
     "dashboard": {
       "enabled": true,
       "cors": {
         "allowed_origins": ["*"]
       }
     }
   }
   ```

---

### 问题：图表不显示

**可能原因**：
1. 数据格式错误
2. API 返回空数据
3. Recharts 配置错误

**解决方法**：
1. 检查 API 返回数据结构
2. 确认统计数据存在
3. 查看浏览器控制台错误

---

### 问题：配置保存失败

**可能原因**：
1. 配置文件权限不足
2. 配置验证失败
3. 热重载失败

**解决方法**：
1. 检查配置文件权限（600）
2. 查看网关日志验证错误信息
3. 手动重启网关应用配置

---

## 开发指南

### 本地开发

1. 安装依赖：
   ```bash
   cd web/dashboard
   npm install
   ```

2. 启动开发服务器：
   ```bash
   npm run dev
   ```

3. 访问开发环境：
   ```
   http://localhost:3001/dashboard/
   ```

### 构建

构建前端并嵌入到 Go 二进制：

```bash
cd web/dashboard
npm run build
```

构建产物输出到 `internal/dashboard/web/`，然后重新编译网关。

### 自定义主题

修改 `src/index.css` 或使用 Ant Design ConfigProvider：

```tsx
import { ConfigProvider } from 'antd'

<ConfigProvider
  theme={{
    token: {
      colorPrimary: '#1890ff',
    },
  }}
>
  <App />
</ConfigProvider>
```

---

## 性能优化

### 前端优化

- 使用 React.memo 减少重渲染
- 使用 React.lazy 懒加载页面
- 压缩构建产物（Vite 自动处理）

### 后端优化

- JSONL 文件缓存
- 统计数据缓存（5 分钟）
- 分页限制（最大 1000 条）

---

## 未来规划

- [ ] 用户认证（JWT/OAuth）
- [ ] 更细粒度的权限控制
- [ ] 自定义 Dashboard 布局
- [ ] 更多图表类型（折线图、热力图）
- [ ] 告警规则配置 UI
- [ ] 导出报表（PDF/Excel）
- [ ] 多语言支持（i18n）