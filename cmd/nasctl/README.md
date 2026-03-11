# nasctl - NAS-OS 命令行管理工具

`nasctl` 是 NAS-OS 的官方命令行管理工具，提供完整的存储管理功能。

## 安装

### 从源码编译

```bash
cd nas-os
go build -o nasctl ./cmd/nasctl
sudo mv nasctl /usr/local/bin/
```

### 使用 Makefile

```bash
make nasctl
sudo make nasctl-install
```

### 多平台构建

```bash
make nasctl-build-all
# 生成：nasctl-linux-amd64, nasctl-linux-arm64, nasctl-linux-armv7
```

## 快速开始

### 查看帮助

```bash
nasctl --help
nasctl <command> --help
```

### 查看版本

```bash
nasctl --version
```

## 功能特性

### 1. 卷管理

```bash
# 列出所有卷
nasctl volume list

# 创建卷
nasctl volume create mydata --devices /dev/sda1,/dev/sdb1 --raid raid1

# 查看卷详情
nasctl volume show mydata

# 删除卷
nasctl volume delete mydata
```

### 2. 子卷管理

```bash
# 列出子卷
nasctl subvolume list mydata

# 创建子卷
nasctl subvolume create mydata/documents

# 删除子卷
nasctl subvolume delete mydata/documents
```

### 3. 快照管理

```bash
# 列出快照
nasctl snapshot list mydata

# 创建快照
nasctl snapshot create mydata/documents --name backup-2026-03-11

# 恢复快照
nasctl snapshot restore mydata/backup-2026-03-11 --target mydata/restored

# 删除快照
nasctl snapshot delete mydata/backup-2026-03-11
```

### 4. 共享管理

```bash
# 列出所有共享
nasctl share list

# 创建 SMB 共享
nasctl share create smb public --path /data/public --guest-ok

# 创建 NFS 共享
nasctl share create nfs backup --path /data/backup --network 192.168.1.0/24

# 删除共享
nasctl share delete public
```

### 5. 系统管理

```bash
# 查看系统状态
nasctl status

# 查看详细状态
nasctl status --verbose

# 查看日志
nasctl logs --tail 100 --level info

# 实时日志
nasctl logs -f

# 重启服务
nasctl restart
```

## 全局选项

| 选项 | 说明 | 默认值 |
|------|------|--------|
| `--config`, `-c` | 配置文件路径 | `/etc/nas-os/config.yaml` |
| `--output`, `-o` | 输出格式 (text/json) | `text` |
| `--verbose`, `-v` | 详细输出 | `false` |
| `--quiet`, `-q` | 静默模式 | `false` |

## 输出格式

### 文本格式（默认）

```bash
$ nasctl volume list
NAME      SIZE     USED     FREE     PROFILE  STATUS
mydata    1.00 TB  256.00 GB  768.00 GB  raid1    ✓
```

### JSON 格式

```bash
$ nasctl volume list -o json
[
  {
    "name": "mydata",
    "size": 1099511627776,
    "used": 274877906944,
    "free": 824633720832,
    "dataProfile": "raid1",
    "status": {
      "healthy": true
    }
  }
]
```

## API 配置

默认连接本地 NAS-OS 服务：`http://localhost:8080/api/v1`

如需连接远程服务，可修改源码中的 `apiBaseURL` 变量或通过配置文件设置。

## 认证

通过配置文件或环境变量设置 API 令牌：

```bash
export NASCTL_API_TOKEN="your-jwt-token"
```

## 示例

### 完整初始化流程

```bash
# 1. 创建存储卷
nasctl volume create main --devices /dev/sda1,/dev/sdb1 --raid raid1

# 2. 创建子卷
nasctl subvolume create main/documents
nasctl subvolume create main/photos
nasctl subvolume create main/backups

# 3. 创建共享
nasctl share create smb documents --path /main/documents --users admin,user1
nasctl share create smb photos --path /main/photos --guest-ok

# 4. 创建快照
nasctl snapshot create main/documents --name daily-backup

# 5. 查看状态
nasctl status --verbose
```

### 故障排查

```bash
# 查看详细日志
nasctl logs -f --level debug

# 检查系统状态
nasctl status --verbose

# JSON 格式输出（便于脚本处理）
nasctl volume list -o json | jq '.[] | select(.status.healthy == false)'
```

## 开发

### 构建

```bash
go build -o nasctl ./cmd/nasctl
```

### 测试

```bash
go test ./cmd/nasctl/...
```

### 添加新命令

在 `main.go` 中添加新的 `cobra.Command`，参考现有命令结构。

## 注意事项

1. **权限要求**: 大部分命令需要 root 权限
   ```bash
   sudo nasctl volume create ...
   ```

2. **数据安全**: 删除操作不可恢复，请谨慎使用

3. **RAID 配置**: 创建 RAID 卷前请确保了解 RAID 级别特性

## 相关文档

- [NASCTL-CLI.md](../../docs/NASCTL-CLI.md) - 完整使用指南
- [API_REFERENCE_v1.0.md](../../docs/API_REFERENCE_v1.0.md) - API 参考
- [ADMIN_GUIDE_v1.0.md](../../docs/ADMIN_GUIDE_v1.0.md) - 管理员指南

## 许可证

MIT License
