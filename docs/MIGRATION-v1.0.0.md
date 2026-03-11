# NAS-OS v1.0.0 迁移指南

**适用场景**: 从其他 NAS 系统迁移到 NAS-OS v1.0.0  
**预计时间**: 1-4 小时（取决于数据量）  
**风险等级**: 中（建议完整备份）

---

## 📋 迁移概览

### 支持的源系统

| 源系统 | 支持程度 | 自动化程度 | 说明 |
|--------|----------|------------|------|
| 群晖 DSM | ✅ 完全支持 | 半自动 | 支持配置和数据迁移 |
| 威联通 QTS | ✅ 完全支持 | 半自动 | 支持配置和数据迁移 |
| TrueNAS Core | ✅ 完全支持 | 半自动 | 支持 ZFS→btrfs 迁移 |
| TrueNAS SCALE | ✅ 完全支持 | 半自动 | 支持配置和数据迁移 |
| OpenMediaVault | ✅ 完全支持 | 半自动 | 支持配置和数据迁移 |
| Unraid | ⚠️ 部分支持 | 手动 | 支持数据迁移 |
| 手动 SMB/NFS | ✅ 完全支持 | 手动 | 通用迁移方案 |
| Windows 文件服务器 | ✅ 完全支持 | 手动 | 支持数据迁移 |

### 迁移内容

| 内容类型 | 可迁移 | 说明 |
|----------|--------|------|
| 用户账户 | ✅ | 用户名、密码（需重置）、邮箱 |
| 用户组 | ✅ | 组名、成员关系 |
| 共享文件夹 | ✅ | 共享名、路径、权限 |
| 访问权限 (ACL) | ✅ | 用户/组对共享的访问权限 |
| 本地数据 | ✅ | 文件、文件夹完整迁移 |
| 系统配置 | ⚠️ | 部分配置可迁移（网络、时间等） |
| 应用配置 | ❌ | 需重新配置（Docker 容器可迁移） |
| 快照 | ❌ | 不支持跨系统快照迁移 |

---

## 🛠️ 迁移前准备

### 1. 源系统检查

```bash
# 记录源系统信息
# - 系统版本
# - 存储配置（RAID 类型、卷大小）
# - 共享列表
# - 用户列表
# - IP 地址和网络配置
```

### 2. 目标系统准备

```bash
# 安装 NAS-OS v1.0.0
# 参考：INSTALL.md

# 验证安装
nasd --version
# 应显示：nasd version 1.0.0

# 检查磁盘空间（应大于源系统已用空间）
df -h /data
```

### 3. 备份策略

**强烈建议**在迁移前对源系统和目标系统都进行完整备份。

```bash
# 源系统备份（如支持）
# 群晖：Hyper Backup
# TrueNAS：zfs send | zfs receive

# 目标系统备份
nasd backup create --name pre-migration
```

### 4. 网络准备

- 确保源系统和目标系统在同一局域网
- 确保网络带宽充足（建议千兆以上）
- 记录双方 IP 地址和访问凭证

---

## 📤 迁移方法

### 方法一：使用迁移工具（推荐）

适用于群晖 DSM、威联通 QTS、TrueNAS。

#### 步骤 1：下载迁移工具

```bash
# 在目标系统（NAS-OS）上下载
wget https://github.com/nas-os/nasd/releases/download/v1.0.0/nas-migrate-tool

chmod +x nas-migrate-tool
sudo mv nas-migrate-tool /usr/local/bin/
```

#### 步骤 2：运行迁移向导

```bash
sudo nas-migrate-tool

# 交互式向导：
# 1. 选择源系统类型
# 2. 输入源系统 IP 地址
# 3. 输入源系统管理员凭证
# 4. 选择要迁移的内容
# 5. 确认迁移配置
# 6. 开始迁移
```

#### 步骤 3：监控迁移进度

```bash
# 查看迁移状态
nas-migrate-tool status

# 查看迁移日志
tail -f /var/log/nas-os/migration.log
```

#### 步骤 4：验证迁移结果

```bash
# 检查用户迁移
nasd auth users list

# 检查共享迁移
nasd services shares list

# 检查数据完整性
nasd storage check --path /data
```

### 方法二：rsync 数据迁移（通用）

适用于所有支持 rsync 的系统。

#### 步骤 1：在源系统启用 rsync

**群晖 DSM**:
```bash
# 控制面板 → 文件服务 → rsync → 启用
# 设置端口：873
# 设置账户权限
```

**TrueNAS**:
```bash
# Services → Rsync → 启用
# 设置允许账户
```

**通用 Linux**:
```bash
# 安装 rsync
sudo apt install rsync  # Debian/Ubuntu
sudo yum install rsync  # CentOS/RHEL

# 启动服务
sudo systemctl start rsync
```

#### 步骤 2：执行数据同步

```bash
# 在 NAS-OS 系统上执行

# 创建目标目录
sudo mkdir -p /data/migrated

# 执行 rsync（保留权限、时间戳、符号链接）
sudo rsync -avzH --progress \
  user@source-ip:/volume1/data/ \
  /data/migrated/

# 增量同步（第二次及以后）
sudo rsync -avzH --delete --progress \
  user@source-ip:/volume1/data/ \
  /data/migrated/
```

**参数说明**:
- `-a`: 归档模式（保留权限、时间戳等）
- `-v`: 详细输出
- `-z`: 压缩传输
- `-H`: 保留硬链接
- `--delete`: 删除目标端多余文件
- `--progress`: 显示进度

#### 步骤 3：验证数据完整性

```bash
# 比较文件数量
find /data/migrated -type f | wc -l

# 比较总大小
du -sh /data/migrated

# 抽样检查文件内容
md5sum /data/migrated/somefile
# 与源系统对比
```

### 方法三：SMB/CIFS 挂载迁移

适用于 Windows、macOS 或任何支持 SMB 的系统。

#### 步骤 1：挂载源系统共享

```bash
# 创建挂载点
sudo mkdir -p /mnt/source

# 挂载 SMB 共享
sudo mount -t cifs //source-ip/share /mnt/source \
  -o username=admin,password=yourpass,iocharset=utf8

# 验证挂载
ls /mnt/source
```

#### 步骤 2：复制数据

```bash
# 使用 cp 复制（保留属性）
sudo cp -av /mnt/source/* /data/migrated/

# 或使用 rsync
sudo rsync -avH /mnt/source/ /data/migrated/
```

#### 步骤 3：卸载共享

```bash
sudo umount /mnt/source
```

### 方法四：NFS 挂载迁移

适用于 Linux、Unix 系统。

#### 步骤 1：挂载源系统 NFS

```bash
# 创建挂载点
sudo mkdir -p /mnt/source

# 挂载 NFS 共享
sudo mount -t nfs source-ip:/export/data /mnt/source

# 验证挂载
ls /mnt/source
```

#### 步骤 2：复制数据

```bash
sudo rsync -avH /mnt/source/ /data/migrated/
```

#### 步骤 3：卸载共享

```bash
sudo umount /mnt/source
```

---

## 👥 用户和权限迁移

### 方法一：CSV 导入（推荐）

#### 步骤 1：导出源系统用户

**群晖 DSM**:
```bash
# 通过 SSH 导出
ssh admin@source-ip
cat /etc/passwd | grep -v nologin > users.csv
```

**TrueNAS**:
```bash
# 通过 Web UI
# Accounts → Users → Export
```

**通用格式**:
```csv
username,password_hash,uid,gid,home,shell,email
admin,$6$...,1000,1000,/home/admin,/bin/bash,admin@example.com
user1,$6$...,1001,100,/home/user1,/bin/bash,user1@example.com
```

#### 步骤 2：转换格式

```bash
# 使用转换脚本
wget https://raw.githubusercontent.com/nas-os/nasd/v1.0.0/scripts/convert-users.sh
chmod +x convert-users.sh

./convert-users.sh --input users.csv --output nas-os-users.json
```

#### 步骤 3：导入到 NAS-OS

```bash
# 导入用户
sudo nasd auth users import --file nas-os-users.json

# 验证导入
nasd auth users list
```

### 方法二：手动创建

适用于用户数量较少的情况。

#### 通过 Web UI

1. 登录 NAS-OS Web UI
2. 进入 **系统设置 → 用户管理**
3. 点击 **添加用户**
4. 填写用户信息
5. 设置密码（需通知用户重置）
6. 分配到相应用户组
7. 重复直到所有用户创建完成

#### 通过命令行

```bash
# 创建用户
sudo nasd auth users create \
  --username admin \
  --email admin@example.com \
  --password "TempPassword123!" \
  --groups administrators

# 批量创建（脚本）
cat <<EOF | while read username email; do
  sudo nasd auth users create \
    --username "$username" \
    --email "$email" \
    --password "TempPassword123!" \
    --groups users
done
admin admin@example.com
user1 user1@example.com
user2 user2@example.com
EOF
```

### 权限迁移

```bash
# 1. 导出源系统权限（示例）
# 记录每个共享的用户/组访问权限

# 2. 在 NAS-OS 上配置权限
sudo nasd services shares acl set \
  --share public \
  --user admin \
  --permissions read,write,delete

# 3. 验证权限
sudo nasd services shares acl get --share public
```

---

## 🐳 Docker 容器迁移

### 方法一：导出/导入镜像

#### 在源系统导出

```bash
# 列出容器
docker ps -a

# 导出镜像
docker save -o myapp.tar myapp:latest

# 导出容器配置
docker inspect myapp > myapp-config.json
```

#### 传输到 NAS-OS

```bash
# 使用 scp 传输
scp myapp.tar myapp-config.json user@nas-os:/tmp/
```

#### 在 NAS-OS 导入

```bash
# 导入镜像
docker load -i /tmp/myapp.tar

# 查看配置参考
cat /tmp/myapp-config.json

# 重新创建容器
docker run -d \
  --name myapp \
  --restart unless-stopped \
  -p 8080:80 \
  -v /data/myapp:/data \
  myapp:latest
```

### 方法二：使用 Docker Compose

#### 在源系统导出

```bash
# 导出 docker-compose.yml
docker-compose config > docker-compose.yml

# 导出 .env 文件（如有）
cp .env docker-compose.env
```

#### 在 NAS-OS 导入

```bash
# 创建应用目录
mkdir -p /data/apps/myapp
cd /data/apps/myapp

# 上传配置文件
# docker-compose.yml
# docker-compose.env

# 启动容器
docker-compose up -d

# 查看状态
docker-compose ps
```

---

## 📊 特定系统迁移指南

### 群晖 DSM → NAS-OS

#### 1. 导出配置

```bash
# 通过 SSH 连接群晖
ssh admin@synology-ip

# 导出用户
cat /etc/passwd | grep -E '/home|/bin/bash' > /tmp/users.txt

# 导出共享列表
cat /etc/samba/smb.conf | grep -A 5 '^\[' > /tmp/shares.txt

# 导出权限
getfacl /volume1/* > /tmp/acl.txt
```

#### 2. 迁移数据

```bash
# 在 NAS-OS 上
sudo rsync -avzH admin@synology-ip:/volume1/data/ /data/migrated/
```

#### 3. 重建配置

- 参考导出的配置文件
- 在 NAS-OS Web UI 手动重建用户、共享、权限

### TrueNAS → NAS-OS

#### 1. 导出 ZFS 配置

```bash
# 在 TrueNAS 上
zpool export -a  # 仅导出配置，不导出数据
zpool get all > zpool-config.txt
zfs get all > zfs-config.txt
```

#### 2. 迁移数据

```bash
# 方案一：网络传输
sudo rsync -avzH root@truenas-ip:/mnt/pool/data/ /data/migrated/

# 方案二：物理磁盘迁移（高级）
# 1. 关闭 TrueNAS
# 2. 拆卸磁盘
# 3. 安装到 NAS-OS 服务器
# 4. 在 NAS-OS 上创建新 btrfs 卷
# 5. 复制数据
```

#### 3. 转换文件系统

```bash
# ZFS → btrfs（数据结构不同，需复制文件）
# 不支持直接转换，必须通过文件级复制
```

### Windows 文件服务器 → NAS-OS

#### 1. 启用 SMB 共享

```powershell
# 在 Windows 上
# 确保共享已启用
# 记录共享路径和权限
```

#### 2. 挂载并复制

```bash
# 在 NAS-OS 上
sudo mkdir -p /mnt/windows
sudo mount -t cifs //windows-ip/share /mnt/windows \
  -o username=Administrator,password=YourPass,iocharset=utf8

# 复制数据
sudo rsync -avH /mnt/windows/ /data/migrated/

# 卸载
sudo umount /mnt/windows
```

#### 3. 导出用户（可选）

```powershell
# 在 Windows 上导出用户
net user > users.txt
net localgroup > groups.txt
```

---

## ✅ 迁移后验证

### 1. 数据完整性检查

```bash
# 检查文件数量
find /data -type f | wc -l

# 检查总大小
du -sh /data

# 抽样检查
md5sum /data/somefile
# 与源系统对比

# 检查文件权限
ls -la /data
```

### 2. 用户验证

```bash
# 列出所有用户
nasd auth users list

# 测试登录
# 使用 Web UI 测试几个用户登录

# 检查用户组
nasd auth groups list
```

### 3. 共享验证

```bash
# 列出所有共享
nasd services shares list

# 测试 SMB 访问
# 从 Windows: \\nas-os-ip\share
# 从 macOS: smb://nas-os-ip/share
# 从 Linux: mount -t cifs //nas-os-ip/share /mnt

# 测试 NFS 访问
showmount -e nas-os-ip
```

### 4. 权限验证

```bash
# 检查共享 ACL
nasd services shares acl get --share public

# 测试访问控制
# 使用不同用户尝试访问共享
```

### 5. 功能测试清单

- [ ] 用户登录成功
- [ ] 共享访问正常
- [ ] 文件读写正常
- [ ] 权限控制生效
- [ ] 监控图表显示
- [ ] 告警通知正常
- [ ] 备份任务执行
- [ ] Docker 容器运行（如使用）

---

## 🔙 回滚方案

### 如果迁移失败

#### 1. 停止迁移

```bash
# 停止正在进行的 rsync
pkill rsync

# 停止迁移工具
sudo nas-migrate-tool stop
```

#### 2. 清理部分迁移的数据

```bash
# 删除迁移的数据
sudo rm -rf /data/migrated

# 删除导入的用户
sudo nasd auth users delete --all
```

#### 3. 恢复备份

```bash
# 恢复迁移前备份
nasd backup restore pre-migration
```

#### 4. 重新启动迁移

- 分析问题原因
- 修复问题
- 重新开始迁移流程

---

## 🐛 常见问题

### Q1: rsync 传输速度慢

**解决方案**:
```bash
# 使用压缩（增加 CPU 使用，减少网络传输）
rsync -avzH ...

# 限制带宽（避免影响其他服务）
rsync -avH --bwlimit=50000 ...  # 50MB/s

# 使用万兆网络
# 确保交换机和网卡支持
```

### Q2: 文件权限丢失

**解决方案**:
```bash
# 使用 rsync 保留权限
rsync -avH --chmod=Du=rwx,Dgo=rx,Fu=rw,Fgo=r ...

# 或迁移后重新设置
find /data -type f -exec chmod 644 {} \;
find /data -type d -exec chmod 755 {} \;
```

### Q3: 中文文件名乱码

**解决方案**:
```bash
# rsync 指定字符集
rsync -avH --iconv=utf-8,utf-8 ...

# 挂载 SMB 指定字符集
mount -t cifs ... -o iocharset=utf8
```

### Q4: 大文件传输失败

**解决方案**:
```bash
# 使用 --partial 保留部分传输的文件
rsync -avH --partial ...

# 使用 --inplace 就地更新
rsync -avH --inplace ...

# 分批次传输
# 按目录分批 rsync
```

### Q5: 用户密码无法迁移

**说明**: 由于加密算法差异，密码哈希通常无法直接迁移。

**解决方案**:
1. 迁移用户账户（不含密码）
2. 设置临时密码
3. 通知用户首次登录时重置密码
4. 启用密码重置邮件通知

---

## 📞 获取帮助

### 自助资源
- [官方文档](https://docs.nas-os.com)
- [迁移工具 GitHub](https://github.com/nas-os/nas-migrate-tool)
- [社区论坛](https://community.nas-os.com)

### 联系支持
- **GitHub Issues**: https://github.com/nas-os/nasd/issues
- **Discord**: https://discord.gg/nas-os
- **邮件支持**: support@nas-os.com

### 报告问题
```markdown
**源系统**: 群晖 DSM 7.0
**目标系统**: NAS-OS v1.0.0
**迁移方法**: rsync
**错误信息**: [粘贴错误日志]
**数据量**: 2TB
**复现步骤**: 
1. ...
2. ...
```

---

## 📊 迁移检查清单

### 迁移前
- [ ] 源系统备份完成
- [ ] 目标系统安装完成
- [ ] 网络连通性测试通过
- [ ] 磁盘空间充足
- [ ] 迁移工具准备就绪

### 迁移中
- [ ] 用户和组导出
- [ ] 数据开始传输
- [ ] 监控传输进度
- [ ] 处理传输错误

### 迁移后
- [ ] 数据完整性验证
- [ ] 用户导入验证
- [ ] 共享配置验证
- [ ] 权限设置验证
- [ ] 功能测试通过
- [ ] 通知用户迁移完成

---

## 🎉 迁移完成

恭喜！您已成功迁移到 NAS-OS v1.0.0。

**下一步建议**:
1. 配置自动备份
2. 启用双因素认证
3. 设置监控告警
4. 探索应用商店
5. 部署常用应用

**分享您的经验**:
- 在社区分享迁移经验
- 帮助其他迁移用户
- 向团队反馈改进建议

---

*文档版本：1.0.0*  
*最后更新：2026-07-31*  
*维护团队：NAS-OS 工部*
