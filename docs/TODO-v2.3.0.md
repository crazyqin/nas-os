# NAS-OS v2.3.0 开发计划

**版本**: v2.3.0  
**目标**: 智能存储管理增强  
**启动日期**: 2026-03-22

---

## ✅ 吏部任务完成情况

### 1. v2.3.0 文档 ✅

- [x] 更新 `README.md` 版本号到 v2.3.0
- [x] 更新 `docs/CHANGELOG.md` 添加 v2.3.0 版本
- [x] 更新 `docs/API_GUIDE.md` 添加新端点
- [x] 创建 `docs/RELEASE-v2.3.0.md` 发布说明
- [x] 创建 `docs/TODO-v2.3.0.md` 任务清单

### 2. 功能说明文档 ✅

- [x] 存储分层使用指南（集成到 API_GUIDE）
- [x] FTP/SFTP 配置指南（集成到 API_GUIDE）
- [x] 文件标签系统说明（集成到 API_GUIDE）

### 3. WebUI 页面 ✅

- [x] `webui/pages/tiering.html` - 存储分层管理页面
- [x] `webui/pages/ftp.html` - FTP 服务器配置页面
- [x] `webui/pages/sftp.html` - SFTP 服务器配置页面
- [x] `webui/pages/tags.html` - 文件标签管理页面

---

## 📊 新增功能清单

| 功能 | 模块 | API | WebUI | 状态 |
|------|------|-----|-------|------|
| 存储分层 | internal/tiering/ | ✅ | ✅ | 完成 |
| FTP 服务器 | internal/ftp/ | ✅ | ✅ | 完成 |
| SFTP 服务器 | internal/sftp/ | ✅ | ✅ | 完成 |
| 文件标签 | internal/tags/ | ✅ | ✅ | 完成 |
| 压缩存储 | internal/files/compress/ | ✅ | - | 完成 |

---

## 📁 新增文件清单

```
docs/
├── RELEASE-v2.3.0.md    # 发布说明
└── TODO-v2.3.0.md       # 本文件

webui/pages/
├── tiering.html         # 存储分层管理页面
├── ftp.html             # FTP 服务器配置页面
├── sftp.html            # SFTP 服务器配置页面
└── tags.html            # 文件标签管理页面
```

---

## 🔄 修改文件清单

```
README.md                # 版本信息更新 → v2.3.0
docs/CHANGELOG.md        # 添加 v2.3.0 版本
docs/API_GUIDE.md        # 添加新端点文档
```

---

## 📋 API 端点清单

### 存储分层 API
- `GET /api/v1/tiering/tiers` - 获取存储层列表
- `GET /api/v1/tiering/tiers/:type` - 获取存储层配置
- `PUT /api/v1/tiering/tiers/:type` - 更新存储层配置
- `GET /api/v1/tiering/policies` - 获取策略列表
- `POST /api/v1/tiering/policies` - 创建策略
- `POST /api/v1/tiering/policies/:id/execute` - 执行策略
- `POST /api/v1/tiering/migrate` - 手动迁移
- `GET /api/v1/tiering/status` - 获取状态

### FTP API
- `GET /api/v1/ftp/config` - 获取配置
- `PUT /api/v1/ftp/config` - 更新配置
- `GET /api/v1/ftp/status` - 获取状态
- `POST /api/v1/ftp/restart` - 重启服务

### SFTP API
- `GET /api/v1/sftp/config` - 获取配置
- `PUT /api/v1/sftp/config` - 更新配置
- `GET /api/v1/sftp/status` - 获取状态
- `POST /api/v1/sftp/restart` - 重启服务

### 文件标签 API
- `GET /api/v1/tags` - 获取标签列表
- `POST /api/v1/tags` - 创建标签
- `PUT /api/v1/tags/:id` - 更新标签
- `DELETE /api/v1/tags/:id` - 删除标签
- `POST /api/v1/tags/:id/files` - 添加文件标签
- `GET /api/v1/tags/:id/files` - 按标签搜索
- `GET /api/v1/tags/cloud` - 获取标签云

---

**完成日期**: 2026-03-28  
**负责人**: 礼部