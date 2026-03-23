# NAS-OS Drive 用户指南

## 功能概述

NAS-OS Drive 是多端文件同步与协作解决方案，提供类似 Synology Drive 的核心功能，支持跨平台文件同步、版本控制和团队协作。

### 核心特性

- **多端同步**：支持 Windows、macOS、Linux、iOS、Android 全平台
- **按需同步**：云端优先，本地按需加载，节省存储空间
- **版本控制**：智能版本管理 (Intelliversioning)，保留重要版本
- **文件锁定**：编辑时自动锁定，防止协作冲突
- **共享协作**：灵活的分享权限设置
- **NAS-to-NAS 同步**：跨设备同步 (ShareSync)
- **离线访问**：标记文件为离线可用

---

## 使用场景

### 1. 个人文件备份

将重要文档、照片自动同步到 NAS，实现：

- 自动备份：指定文件夹变更自动同步
- 版本保留：误删误改可随时恢复
- 跨设备访问：手机拍的照片电脑立即可用

### 2. 团队协作

团队共享工作空间，支持：

- 共享文件夹：团队文档集中管理
- 权限控制：只读/编辑/管理分级权限
- 文件锁定：避免同时编辑冲突
- 变更通知：文件更新实时通知

### 3. 远程办公

随时随地访问办公文件：

- 按需同步：只下载需要的文件
- 离线模式：标记文件离线可用
- 移动端访问：手机/平板随时查看

### 4. 多 NAS 同步

跨地点数据同步：

- 主备同步：关键数据异地备份
- 分支协同：多办公室文件共享
- 数据迁移：设备升级数据迁移

---

## 配置说明

### 服务端配置

#### 1. 启用 Drive 服务

进入 **控制面板 > 服务 > Drive**：

```json
{
  "enabled": true,
  "port": 8082,
  "max_connections": 100,
  "max_upload_speed": 0,        // 0 = 不限制 (KB/s)
  "max_download_speed": 0,
  "enable_versioning": true,
  "max_versions": 32,
  "min_interval_minutes": 5
}
```

#### 2. 创建共享文件夹

进入 **Drive 管理控制台 > 共享文件夹**：

| 配置项 | 说明 |
|--------|------|
| 名称 | 共享文件夹名称 |
| 路径 | NAS 存储路径 |
| 权限 | 用户/组访问权限 |
| 配额 | 存储空间限制 |
| 版本策略 | 版本保留规则 |

#### 3. 用户权限配置

```json
{
  "user": "zhangsan",
  "permissions": {
    "home": {
      "read": true,
      "write": true,
      "share": true
    },
    "team-docs": {
      "read": true,
      "write": false,
      "share": false
    }
  }
}
```

### 客户端配置

#### Windows/macOS/Linux 客户端

1. **下载客户端**
   - 从 NAS 控制面板下载对应平台客户端
   - 或访问 `https://<nas-ip>:8082/client`

2. **首次配置**
   ```yaml
   server: https://nas.example.com:8082
   username: your_username
   password: your_password
   sync_folder: ~/NAS-Drive
   selective_sync:
     - home
     - team-docs
   bandwidth_limit:
     upload: 0      # KB/s, 0=不限制
     download: 0
   ```

3. **同步模式选择**
   
   | 模式 | 说明 | 适用场景 |
   |------|------|----------|
   | 标准同步 | 全量同步所有文件 | 文件夹较小 |
   | 按需同步 | 仅创建占位符 | 存储空间有限 |
   | 混合模式 | 部分全量，部分按需 | 重要文件常访问 |

#### 移动端配置

1. **App 下载**
   - iOS: App Store 搜索 "NAS-OS Drive"
   - Android: Google Play 或 APK 下载

2. **配置步骤**
   ```
   1. 打开 App
   2. 输入 NAS 地址 (如 https://nas.local:8082)
   3. 输入用户名密码登录
   4. 选择同步文件夹
   5. 设置自动备份 (照片/视频)
   ```

3. **自动备份设置**
   ```json
   {
     "auto_backup": {
       "photos": true,
       "videos": true,
       "wifi_only": true,
       "backup_path": "/home/mobile-backup"
     }
   }
   ```

---

## 常见问题

### 安装与连接

#### Q: 客户端无法连接 NAS？

**排查步骤**：
1. 检查 NAS Drive 服务是否启动
2. 确认防火墙放行 8082 端口
3. 检查 SSL 证书是否有效
4. 尝试关闭 SSL 验证测试

```bash
# 检查服务状态
systemctl status nasd

# 检查端口
netstat -tlnp | grep 8082

# 测试连接
curl -k https://nas-ip:8082/api/health
```

#### Q: 客户端提示证书错误？

**解决方案**：
- 方案1: 在 NAS 安装有效 SSL 证书
- 方案2: 客户端设置中启用 "忽略证书错误" (仅限测试环境)

### 同步问题

#### Q: 文件同步速度很慢？

**优化方法**：

1. **检查带宽限制**
   ```json
   // 关闭带宽限制
   "bandwidth_limit": {
     "upload": 0,
     "download": 0
   }
   ```

2. **启用增量同步**
   ```json
   "sync_options": {
     "incremental": true,
     "chunk_size": 4194304  // 4MB
   }
   ```

3. **调整并发连接**
   ```json
   "max_connections": 10
   ```

#### Q: 出现文件冲突怎么办？

**冲突原因**：多人同时编辑同一文件

**解决方案**：
1. 启用文件锁定功能
2. 冲突文件会自动生成副本：`file (conflict copy).doc`
3. 手动合并后删除冲突副本

#### Q: 如何恢复误删的文件？

**恢复步骤**：
1. 打开 Drive 客户端
2. 右键点击文件夹 > 版本历史
3. 选择要恢复的版本
4. 点击 "恢复"

**服务端恢复**：
```bash
# 查看版本历史
nasctl drive versions /home/user/document.doc

# 恢复特定版本
nasctl drive restore /home/user/document.doc --version 5
```

### 存储管理

#### Q: 如何节省本地存储空间？

**使用按需同步**：
1. 客户端设置 > 同步模式 > 按需同步
2. 文件显示为占位符，双击时下载
3. 右键文件 > "释放本地空间" 可移除本地副本

#### Q: 版本历史占用太多空间？

**清理策略**：

1. **自动清理**
   ```json
   "versioning": {
     "max_versions": 32,
     "min_interval_minutes": 5,
     "keep_days": 90
   }
   ```

2. **手动清理**
   ```bash
   # 清理超过 30 天的版本
   nasctl drive prune --older-than 30d
   ```

### 团队协作

#### Q: 如何设置共享文件夹权限？

**权限级别**：

| 权限 | 说明 |
|------|------|
| 只读 | 仅查看文件，不能修改 |
| 读写 | 可查看、编辑、上传文件 |
| 管理 | 完全控制，包括权限管理 |

**设置方法**：
1. 进入 Drive 管理控制台
2. 选择共享文件夹 > 权限设置
3. 添加用户/组并设置权限级别

#### Q: 如何启用文件锁定？

**服务端配置**：
```json
{
  "file_lock": {
    "enabled": true,
    "auto_lock_on_edit": true,
    "lock_timeout_minutes": 30,
    "allow_force_unlock": true
  }
}
```

**客户端使用**：
- 编辑文件时自动锁定
- 右键文件 > 锁定/解锁
- 查看锁定状态和锁定者

---

## 高级功能

### 1. 选择性同步

仅同步需要的文件夹：

```json
{
  "selective_sync": {
    "enabled": true,
    "include": [
      "work/projects",
      "personal/photos"
    ],
    "exclude": [
      "work/archive",
      "*.tmp"
    ]
  }
}
```

### 2. NAS-to-NAS 同步 (ShareSync)

**配置方法**：
```json
{
  "sharesync": {
    "enabled": true,
    "remote_nas": "https://nas2.example.com:8082",
    "sync_pairs": [
      {
        "local": "/team-docs",
        "remote": "/team-docs-backup",
        "direction": "bidirectional",
        "schedule": "0 */6 * * *"
      }
    ]
  }
}
```

### 3. 审计日志

查看文件操作记录：

```bash
# 查看最近 100 条操作
nasctl drive audit-log --limit 100

# 过滤特定用户
nasctl drive audit-log --user zhangsan

# 导出审计日志
nasctl drive audit-export --output audit.csv
```

**审计事件类型**：
- 文件上传/下载
- 文件创建/删除/重命名
- 文件共享/取消共享
- 权限变更
- 版本恢复

---

## 技术规格

### 同步协议

| 特性 | 实现 |
|------|------|
| 传输协议 | HTTPS (TLS 1.3) |
| 数据格式 | JSON + 二进制流 |
| 增量同步 | 块级差异检测 (rsync 算法) |
| 断点续传 | 支持 |
| 文件监听 | inotify / FSEvents |

### 性能指标

| 指标 | 数值 |
|------|------|
| 最大同步文件数 | 1,000,000+ |
| 单文件大小限制 | 16 TB |
| 并发连接数 | 100 |
| 同步延迟 | < 5s |

### 存储要求

| 组件 | 要求 |
|------|------|
| 服务端存储 | 根据用户数据量规划 |
| 版本存储 | 原始数据的 10-20% |
| 客户端缓存 | 1-5 GB (可配置) |

---

## 相关文档

- [API 参考文档](./API_GUIDE.md)
- [Audio Station 功能规划](./audio-station-guide.md)
- [Calendar & Contacts 规格说明](./calendar-contacts-spec.md)

---

**文档版本**: v1.0.0  
**最后更新**: 2026-03-23