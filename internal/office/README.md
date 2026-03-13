# OnlyOffice 文档编辑集成模块

## 概述

本模块实现 NAS-OS 与 OnlyOffice Document Server 的集成，提供在线文档编辑、协作编辑等功能。

## 功能特性

- **在线编辑**: 支持 docx, xlsx, pptx 等 Office 格式文档的在线编辑
- **协作编辑**: 多用户实时协作编辑
- **格式转换**: 支持 Office、OpenDocument、PDF 等多种格式
- **权限控制**: 细粒度的文档权限管理
- **自动保存**: 编辑自动保存，防止数据丢失

## 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                        NAS-OS Web UI                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                     Office Module                            │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │
│  │   Manager   │  │   Handlers  │  │   Types     │          │
│  │  (会话管理)  │  │  (API 处理) │  │  (数据结构)  │          │
│  └─────────────┘  └─────────────┘  └─────────────┘          │
└─────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┴───────────────┐
              ▼                               ▼
┌─────────────────────────┐    ┌─────────────────────────────┐
│   OnlyOffice Document   │    │      NAS-OS Storage         │
│       Server            │◄───│     (文件存储服务)           │
│   (Docker 容器)         │    │                             │
└─────────────────────────┘    └─────────────────────────────┘
              │
              ▼
┌─────────────────────────────────────────────────────────────┐
│                     回调处理                                  │
│   POST /api/v1/office/callback/:sessionId                   │
│   - 文档保存通知                                             │
│   - 编辑状态更新                                             │
│   - 协作用户管理                                             │
└─────────────────────────────────────────────────────────────┘
```

## API 接口

### 配置管理

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/office/config` | 获取配置 |
| PUT | `/api/v1/office/config` | 更新配置 |
| POST | `/api/v1/office/config/test` | 测试服务器连接 |

### 编辑会话

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/office/edit/:fileId` | 开始编辑文档 |
| GET | `/api/v1/office/sessions` | 列出会话 |
| GET | `/api/v1/office/sessions/:sessionId` | 获取会话详情 |
| DELETE | `/api/v1/office/sessions/:sessionId` | 关闭会话 |

### 回调处理

| 方法 | 路径 | 描述 |
|------|------|------|
| POST | `/api/v1/office/callback/:sessionId` | OnlyOffice 回调 |

### 文件关联

| 方法 | 路径 | 描述 |
|------|------|------|
| GET | `/api/v1/office/associations` | 获取所有文件关联 |
| GET | `/api/v1/office/associations/:ext` | 获取指定扩展名关联 |

## 配置说明

```json
{
  "server_url": "http://onlyoffice:80",
  "secret_key": "your-jwt-secret-key",
  "enabled": true,
  "callback_auth": true,
  "enabled_types": ["docx", "xlsx", "pptx", "pdf"],
  "editor_config": {
    "lang": "zh-CN",
    "mode": "edit",
    "co_editing": {
      "enabled": true,
      "auto_save": true,
      "save_delay": 5,
      "show_changes": true
    },
    "customization": {
      "hide_right_menu": false,
      "hide_header": false,
      "forcesave": true
    }
  },
  "session_timeout": 3600
}
```

### 配置项说明

| 配置项 | 类型 | 默认值 | 描述 |
|--------|------|--------|------|
| `server_url` | string | `http://localhost:8080` | OnlyOffice Document Server URL |
| `secret_key` | string | `""` | JWT 签名密钥 |
| `enabled` | bool | `false` | 是否启用在线编辑 |
| `callback_auth` | bool | `true` | 回调是否需要认证 |
| `enabled_types` | []string | [...] | 支持的文件类型 |
| `session_timeout` | int | `3600` | 会话超时时间（秒） |

## 支持的文件格式

### 文档类型 (Word)
- `.doc`, `.docx`, `.docm`, `.dotx`, `.dotm`
- `.odt`, `.fodt`, `.ott`
- `.rtf`, `.txt`, `.html`, `.htm`, `.mht`
- `.pdf`, `.djvu`, `.fb2`, `.epub`, `.xps`, `.oxps`

### 电子表格类型 (Excel)
- `.xls`, `.xlsx`, `.xlsm`, `.xlt`, `.xltx`, `.xltm`
- `.ods`, `.fods`, `.ots`
- `.csv`

### 演示文稿类型 (PowerPoint)
- `.ppt`, `.pptx`, `.pptm`, `.pot`, `.potx`, `.potm`
- `.odp`, `.fodp`, `.otp`
- `.ppsx`, `.ppsm`, `.pps`, `.ppam`

## 集成步骤

### 1. 部署 OnlyOffice Document Server

参考 `docker-compose.onlyoffice.yml` 部署 OnlyOffice 服务。

### 2. 配置 NAS-OS

```bash
# 更新配置
curl -X PUT http://localhost:8080/api/v1/office/config \
  -H "Content-Type: application/json" \
  -d '{
    "server_url": "http://onlyoffice:80",
    "secret_key": "your-jwt-secret",
    "enabled": true
  }'

# 测试连接
curl -X POST http://localhost:8080/api/v1/office/config/test
```

### 3. 前端集成

```javascript
// 获取编辑器配置
const response = await fetch('/api/v1/office/edit/' + fileId, {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ mode: 'edit' })
});

const { data } = await response.json();

// 加载 OnlyOffice 编辑器
new DocsAPI.DocEditor("placeholder", {
  document: data.editor_config.document,
  documentType: data.editor_config.document_type,
  editorConfig: data.editor_config.editor,
  height: "100%",
  width: "100%"
});
```

## 回调状态码

| 状态码 | 描述 |
|--------|------|
| 1 | 正在编辑 |
| 2 | 已保存，可下载 |
| 3 | 正在保存 |
| 4 | 文档已关闭 |
| 6 | 强制保存 |
| 7 | 文档错误 |
| 8 | 关闭错误 |

## 安全考虑

1. **JWT 认证**: 建议配置 `secret_key` 启用 JWT 签名验证
2. **HTTPS**: 生产环境必须使用 HTTPS
3. **回调验证**: 验证回调来源和 Token
4. **权限控制**: 集成 NAS-OS 的 RBAC 系统

## 性能优化

1. **会话管理**: 定期清理过期会话
2. **缓存策略**: 缓存文件关联配置
3. **并发控制**: 同一文档的并发编辑数限制
4. **资源隔离**: OnlyOffice 容器独立部署

## 故障排除

### OnlyOffice 服务不可达

```bash
# 检查容器状态
docker ps | grep onlyoffice

# 查看日志
docker logs onlyoffice

# 检查网络
curl http://onlyoffice:80/healthcheck
```

### 回调失败

1. 检查网络连通性
2. 验证 JWT Token 配置
3. 查看 NAS-OS 日志

### 文档无法保存

1. 检查存储权限
2. 验证文件访问 URL
3. 检查磁盘空间

## 未来扩展

- [ ] 版本历史管理
- [ ] 模板管理
- [ ] 水印配置
- [ ] 移动端适配
- [ ] 离线编辑支持