# nasctl CLI 使用指南

> **版本**: v1.0.0  
> **最后更新**: 2026-03-11

---

## 📦 安装

### 方式一：下载二进制文件

```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v1.0.0/nasctl-linux-amd64
chmod +x nasctl-linux-amd64
sudo mv nasctl-linux-amd64 /usr/local/bin/nasctl

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v1.0.0/nasctl-linux-arm64
chmod +x nasctl-linux-arm64
sudo mv nasctl-linux-arm64 /usr/local/bin/nasctl

# 验证安装
nasctl --version
```

### 方式二：源码编译

```bash
cd nas-os
go build -o nasctl ./cmd/nasctl
sudo mv nasctl /usr/local/bin/
```

---

## 🚀 快速开始

### 查看帮助

```bash
nasctl --help
nasctl <command> --help
```

### 查看版本

```bash
nasctl --version
```

---

## 📋 命令参考

### 存储管理

#### 创建卷

```bash
# 创建单盘卷
nasctl volume create mydata --device /dev/sda1

# 创建 RAID1 卷
nasctl volume create data --devices /dev/sda1,/dev/sdb1 --raid raid1

# 创建 RAID5 卷（需要至少 3 块盘）
nasctl volume create storage --devices /dev/sda1,/dev/sdb1,/dev/sdc1 --raid raid5
```

#### 查看卷列表

```bash
nasctl volume list
```

#### 查看卷详情

```bash
nasctl volume show mydata
```

#### 删除卷

```bash
nasctl volume delete mydata
```

#### 创建子卷

```bash
nasctl subvolume create mydata/documents
```

#### 创建快照

```bash
# 创建可写快照
nasctl snapshot create mydata/documents --name backup-2026-03-11

# 创建只读快照
nasctl snapshot create mydata/documents --name backup-2026-03-11 --readonly
```

#### 查看快照列表

```bash
nasctl snapshot list mydata
```

#### 恢复快照

```bash
nasctl snapshot restore mydata/backup-2026-03-11 --target mydata/restored
```

#### 数据平衡

```bash
nasctl balance start mydata
nasctl balance status mydata
nasctl balance cancel mydata
```

#### 数据校验

```bash
nasctl scrub start mydata
nasctl scrub status mydata
nasctl scrub cancel mydata
```

---

### 共享管理

#### 创建 SMB 共享

```bash
nasctl share create smb public --path /data/public --guest-ok
nasctl share create smb home --path /data/home --users admin,user1
```

#### 创建 NFS 共享

```bash
nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24
```

#### 查看共享列表

```bash
nasctl share list
nasctl share list --type smb
nasctl share list --type nfs
```

#### 删除共享

```bash
nasctl share delete public
```

---

### 用户管理

#### 创建用户

```bash
nasctl user create john --password
nasctl user create admin --password --admin
```

#### 查看用户列表

```bash
nasctl user list
```

#### 修改用户

```bash
nasctl user modify john --new-password
nasctl user modify john --add-role editor
```

#### 删除用户

```bash
nasctl user delete john
```

---

### 系统管理

#### 查看系统状态

```bash
nasctl status
```

#### 查看日志

```bash
# 实时日志
nasctl logs -f

# 查看最近 100 行
nasctl logs --tail 100

# 查看错误日志
nasctl logs --level error
```

#### 重启服务

```bash
nasctl restart
```

#### 停止服务

```bash
nasctl stop
```

#### 启动服务

```bash
nasctl start
```

---

## 🔧 全局选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--config`, `-c` | 配置文件路径 | `/etc/nas-os/config.yaml` |
| `--verbose`, `-v` | 详细输出 | `false` |
| `--quiet`, `-q` | 静默模式 | `false` |
| `--output`, `-o` | 输出格式 (text/json) | `text` |

---

## 📝 示例

### 完整初始化流程

```bash
# 1. 创建存储卷
nasctl volume create main --device /dev/sda1

# 2. 创建子卷
nasctl subvolume create main/documents
nasctl subvolume create main/photos
nasctl subvolume create main/backups

# 3. 创建 SMB 共享
nasctl share create smb documents --path /main/documents --users admin,user1
nasctl share create smb photos --path /main/photos --guest-ok

# 4. 创建用户
nasctl user create user1 --password

# 5. 创建定时快照
nasctl snapshot create main/documents --name daily-backup --schedule "0 2 * * *"

# 6. 查看状态
nasctl status
```

### 故障排查

```bash
# 查看详细日志
nasctl logs -f --level debug

# 检查存储健康状态
nasctl scrub start main
nasctl scrub status main

# 查看系统资源使用
nasctl status --verbose
```

---

## ⚠️ 注意事项

1. **权限要求**: 大部分命令需要 root 权限
   ```bash
   sudo nasctl volume create ...
   ```

2. **数据安全**: 删除操作不可恢复，请谨慎使用
   ```bash
   nasctl volume delete  # ⚠️ 危险操作
   nasctl subvolume delete  # ⚠️ 危险操作
   ```

3. **RAID 配置**: 创建 RAID 卷前请确保了解 RAID 级别特性
   - `single`: 单盘，无冗余
   - `raid0`: 条带化，性能最佳，无冗余
   - `raid1`: 镜像，50% 利用率，高安全
   - `raid5`: 分布式奇偶校验，至少 3 盘
   - `raid6`: 双奇偶校验，至少 4 盘
   - `raid10`: 镜像 + 条带，至少 4 盘

---

## 🔗 相关文档

- [用户手册](USER_MANUAL_v1.0.md)
- [管理员指南](ADMIN_GUIDE_v1.0.md)
- [API 参考](API_REFERENCE_v1.0.md)
- [部署指南](DEPLOYMENT_GUIDE_v1.0.md)

---

*最后更新：2026-03-11*
