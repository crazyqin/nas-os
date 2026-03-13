# NAS-OS v1.8.0 开发计划

**创建日期**: 2026-03-13  
**目标发布日期**: 2026-03-20  
**基于版本**: v1.7.0

---

## 🎯 版本目标

v1.8.0 聚焦于 **数据安全与智能管理**，重点实现文件版本控制、云同步增强、数据去重等企业级功能。

---

## 📋 新增功能

### 🔴 高优先级

#### 1. 文件版本控制 (File Versioning) - 3-4 天
**描述**: 自动保存文件历史版本，支持版本恢复

**功能**:
- [ ] 自动版本快照 (基于时间/变更触发)
- [ ] 版本对比 (diff 显示)
- [ ] 版本恢复 (一键还原)
- [ ] 版本保留策略 (按数量/时间/空间)
- [ ] 版本清理 (自动过期删除)

**模块**: `internal/versioning/`

**API 端点**:
```
GET    /api/v1/files/:path/versions      # 列出版本历史
GET    /api/v1/versions/:id              # 获取版本详情
POST   /api/v1/versions/:id/restore     # 恢复到指定版本
DELETE /api/v1/versions/:id             # 删除版本
GET    /api/v1/versions/:id/diff        # 版本对比
```

---

#### 2. 云同步增强 (Cloud Sync Enhanced) - ✅ 已完成
**描述**: 扩展云存储支持，实现双向同步

**新增支持**:
- [x] 阿里云 OSS
- [x] 腾讯云 COS
- [x] AWS S3 (增强)
- [x] Google Drive
- [x] OneDrive
- [x] Backblaze B2

**功能**:
- [x] 双向同步 (本地↔云端)
- [x] 增量同步 (仅传输变更)
- [x] 冲突检测与解决
- [x] 同步计划 (定时/实时)
- [x] 同步状态监控

**模块**: `internal/cloudsync/`

**API 端点**:
```
POST   /api/v1/cloudsync/providers        # 添加云存储 ✅
GET    /api/v1/cloudsync/providers        # 列出云存储 ✅
DELETE /api/v1/cloudsync/providers/:id    # 删除云存储 ✅
POST   /api/v1/cloudsync/tasks            # 创建同步任务 ✅
GET    /api/v1/cloudsync/tasks            # 列出同步任务 ✅
POST   /api/v1/cloudsync/tasks/:id/run    # 执行同步 ✅
GET    /api/v1/cloudsync/tasks/:id/status # 同步状态 ✅
```

---

#### 3. 数据去重 (Data Deduplication) - ✅ 已完成
**描述**: 检测和删除重复数据，节省存储空间

**功能**:
- [ ] 文件级去重 (相同文件检测)
- [ ] 块级去重 (内容寻址存储)
- [ ] 跨用户去重 (共享数据)
- [ ] 去重报告 (节省空间统计)
- [ ] 去重策略配置 (自动/手动)

**模块**: `internal/dedup/`

**API 端点**:
```
POST   /api/v1/dedup/scan                # 扫描重复文件
GET    /api/v1/dedup/duplicates          # 列出重复文件
POST   /api/v1/dedup/deduplicate         # 执行去重
GET    /api/v1/dedup/report              # 去重报告
GET    /api/v1/dedup/stats               # 去重统计
```

---

### 🟡 中优先级

#### 4. 存储分层 (Storage Tiering) - 5-7 天
**描述**: 热数据/冷数据自动分层，优化性能与成本

**功能**:
- [ ] SSD 缓存层配置
- [ ] HDD 存储层配置
- [ ] 云归档层配置
- [ ] 自动数据迁移 (基于访问频率)
- [ ] 分层策略管理
- [ ] 分层状态报告

**模块**: `internal/tiering/`

**API 端点**:
```
POST   /api/v1/tiering/tiers             # 创建存储层
GET    /api/v1/tiering/tiers             # 列出存储层
PUT    /api/v1/tiering/tiers/:id         # 更新存储层
POST   /api/v1/tiering/policies          # 创建分层策略
GET    /api/v1/tiering/policies           # 列出分层策略
POST   /api/v1/tiering/migrate            # 手动迁移数据
GET    /api/v1/tiering/stats              # 分层统计
```

---

#### 5. FTP/SFTP 服务器 (FTP/SFTP Server) - 3-4 天
**描述**: 传统文件传输协议支持

**功能**:
- [ ] FTP 服务器 (主动/被动模式)
- [ ] SFTP 服务器 (SSH 文件传输)
- [ ] 用户权限集成
- [ ] 带宽限制
- [ ] 传输日志记录

**模块**: `internal/ftp/`, `internal/sftp/`

**API 端点**:
```
GET    /api/v1/ftp/config                # FTP 配置
PUT    /api/v1/ftp/config                # 更新 FTP 配置
POST   /api/v1/ftp/restart               # 重启 FTP 服务
GET    /api/v1/sftp/config               # SFTP 配置
PUT    /api/v1/sftp/config               # 更新 SFTP 配置
POST   /api/v1/sftp/restart              # 重启 SFTP 服务
```

---

#### 6. 压缩存储 (Compressed Storage) - 3-4 天
**描述**: 透明压缩，节省存储空间

**功能**:
- [ ] 实时透明压缩
- [ ] 压缩算法选择 (zstd/lz4/gzip)
- [ ] 压缩率统计
- [ ] 选择性压缩 (按目录/文件类型)
- [ ] 压缩性能监控

**模块**: `internal/compress/`

**API 端点**:
```
GET    /api/v1/compress/config            # 压缩配置
PUT    /api/v1/compress/config            # 更新压缩配置
POST   /api/v1/compress/enable            # 启用压缩
POST   /api/v1/compress/disable           # 禁用压缩
GET    /api/v1/compress/stats              # 压缩统计
```

---

### 🟢 低优先级

#### 7. 在线文档编辑 (Online Document Editor) - 6-8 天
**描述**: 集成 OnlyOffice/Collabora，支持在线编辑

**功能**:
- [ ] OnlyOffice 集成
- [ ] Collabora 集成
- [ ] Office 文档在线编辑
- [ ] 实时协作编辑
- [ ] 版本历史
- [ ] 评论和批注

**模块**: `internal/office/`

---

#### 8. 智能搜索增强 (Smart Search Enhanced) - 4-5 天
**描述**: 语义搜索和图像搜索

**功能**:
- [ ] 自然语言搜索
- [ ] 图像内容搜索
- [ ] OCR 文字识别搜索
- [ ] 搜索结果排序优化
- [ ] 搜索历史

**模块**: `internal/search/` (增强)

---

## 📊 模块统计

| 类别 | 现有模块 | v1.8.0 新增 |
|------|----------|-------------|
| 存储 | 4 | +3 (versioning, dedup, tiering) |
| 网络 | 2 | +1 (cloudsync) |
| 共享 | 3 | +2 (ftp, sftp) |
| 工具 | 13 | +1 (compress) |
| **总计** | **33** | **+7** |

---

## 🗓️ 开发排期

### 第 1 周 (2026-03-14 ~ 2026-03-20)

| 日期 | 任务 | 负责人 |
|------|------|--------|
| 03-14 | 文件版本控制 (versioning) | 兵部 |
| 03-15 | 文件版本控制 (完成) | 兵部 |
| 03-16 | 云同步增强 (cloudsync) | 工部 |
| 03-17 | 云同步增强 (继续) | 工部 |
| 03-18 | 数据去重 (dedup) | 兵部 |
| 03-19 | 数据去重 (完成) + 测试 | 兵部 |
| 03-20 | 集成测试 + 发布准备 | 全部 |

### 第 2 周 (2026-03-21 ~ 2026-03-27) - 可选功能

| 日期 | 任务 | 负责人 |
|------|------|--------|
| 03-21 | 存储分层 (tiering) | 工部 |
| 03-22 | 存储分层 (继续) | 工部 |
| 03-23 | FTP/SFTP 服务器 | 兵部 |
| 03-24 | 压缩存储 (compress) | 兵部 |
| 03-25 | WebUI 更新 | 礼部 |
| 03-26 | 文档更新 | 礼部 |
| 03-27 | v1.8.0 发布 | 全部 |

---

## ✅ 发布检查清单

- [ ] 所有高优先级功能完成
- [ ] 单元测试覆盖率 > 80%
- [ ] 集成测试通过
- [ ] 更新 README.md 版本号
- [ ] 更新 MILESTONES.md
- [ ] 更新 docs/CHANGELOG.md
- [ ] 生成发布说明
- [ ] 构建多架构二进制 (amd64/arm64/armv7)
- [ ] 构建 Docker 镜像
- [ ] 创建 GitHub Release
- [ ] 更新 API 文档

---

## 🔗 相关文档

- [新功能提案](docs/NEW_FEATURES_PROPOSAL.md)
- [里程碑规划](MILESTONES.md)
- [API 文档](docs/API_GUIDE.md)
- [开发指南](docs/DEVELOPER.md)

---

## 📝 备注

- v1.8.0 是 v1.x 系列的最后一个大版本
- v2.0.0 将引入插件系统和完整的企业功能
- 建议优先完成高优先级功能，确保核心稳定性

---

*创建者：吏部*  
*创建日期：2026-03-13*