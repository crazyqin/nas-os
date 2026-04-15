# Release Notes v2.458.0

**发布日期**: 2026-04-16
**版本**: v2.458.0

## 🆕 新功能

### Drive 文件同步客户端 (Phase 1)
- 全新 `internal/drive/sync/` 模块，对标 Synology Drive 核心能力
- **双向同步引擎** - 支持 upload_only、download_only、bidirectional 三种模式
- **冲突处理** - 支持 keep_latest、keep_local、keep_remote、keep_both、ask_user 五种策略
- **带宽控制** - Token-bucket 令牌桶限速器，上传/下载独立限速
- **版本历史** - Intelliversioning 算法，智能保留最有价值的版本快照
- **文件变更检测** - SHA-256 校验和 + 修改时间双重判断
- **按需同步** - On-demand 流式传输，节省本地空间
- 参考: Synology Drive 的 sync-on-demand、file locking、Intelliversioning

### Passkey/WebAuthn 无密码登录
- ✅ 修复 `TestVerifyRegistration` 中的 RWMutex 锁不匹配问题
- ✅ 修复 `TestVerifyAuthentication` 中的 RLock/Unlock 类型不匹配

## 🔧 修复

- **fix(test)**: passkey_test.go RWMutex unlock mismatch - `RLock()` 必须使用 `RUnlock()` 释放
- **fix(test)**: 移除未使用的 subject 变量修复 CI 编译失败

## 🏗️ 竞品调研

### Synology Drive 核心特性学习
| 功能 | Synology 实现 | nas-os 进展 |
|------|-------------|------------|
| Sync on Demand | 按需流式传输 | ✅ Phase1 完成 |
| Intelliversioning | 智能版本保留 | ✅ 已实现 |
| File Locking | 多人编辑锁 | 📋 Phase2 |
| Team Folders | 团队共享 | 📋 Phase2 |
| ShareSync | NAS间同步 | 📋 Phase2 |

### Synology Active Backup 对标
| 功能 | Active Backup | nas-os 计划 |
|------|-------------|------------|
| 整机备份 (Bare-metal) | Windows/Mac | 📋 规划中 |
| 全局去重 | Global Dedup | ✅ 已有 |
| 增量备份 | CBT | ✅ 已有 |
| 自助恢复 | Self-service | 📋 计划中 |

### 飞牛 fnOS 对标
| 功能 | fnOS | nas-os 状态 |
|------|------|-----------|
| FN Connect 内网穿透 | 免费 | ✅ FRP 已完成 |
| 网盘挂载 | 115/夸克/百度 | ✅ 6+ 平台 |
| 应用市场 | 30+ 应用 | ✅ 模板已有 |

## 📊 项目统计

- Go 源文件: 1,240+
- 代码行数: 700,000+
- 新增模块: `internal/drive/sync/`
- 新增文件: sync.go, conflict.go, version.go, ratelimiter.go, errors.go
