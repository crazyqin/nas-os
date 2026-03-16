# NAS-OS 故障排查指南

> v2.147.0 - 工部运维手册

本文档提供 NAS-OS 常见问题的诊断和解决方案。

## 目录

1. [快速诊断](#快速诊断)
2. [服务问题](#服务问题)
3. [存储问题](#存储问题)
4. [网络问题](#网络问题)
5. [备份与快照](#备份与快照)
6. [性能问题](#性能问题)
7. [监控告警](#监控告警)
8. [日志分析](#日志分析)

---

## 快速诊断

### 一键健康检查

```bash
# 检查服务状态
systemctl status nas-os

# 检查端口
ss -tulpn | grep -E '8080|445|2049|111'

# 检查磁盘
df -h
btrfs filesystem show

# 检查内存
free -h

# 检查日志最近错误
journalctl -u nas-os -p err -n 20
```

### 健康检查 API

```bash
# 基础健康检查
curl http://localhost:8080/api/v1/health

# 详细健康检查
curl http://localhost:8080/api/v1/health/detailed

# 就绪检查（Kubernetes）
curl http://localhost:8080/ready
```

### 常用诊断命令

```bash
# 查看进程状态
ps aux | grep nasd

# 查看系统资源
top -p $(pgrep nasd)

# 查看打开的文件
lsof -p $(pgrep nasd) | wc -l

# 查看网络连接
netstat -anp | grep nasd

# 查看系统日志
journalctl -u nas-os -f
```

---

## 服务问题

### 服务无法启动

**症状：** `systemctl start nas-os` 失败

**诊断步骤：**

```bash
# 1. 查看详细错误
journalctl -u nas-os -n 50

# 2. 检查配置文件语法
nasd --config /etc/nas-os/config.yaml --test

# 3. 检查端口占用
ss -tulpn | grep 8080
lsof -i :8080

# 4. 检查权限
ls -la /etc/nas-os/
ls -la /var/lib/nas-os/

# 5. 检查依赖服务
systemctl status dbus
systemctl status systemd-resolved
```

**常见原因及解决：**

| 原因 | 解决方案 |
|------|---------|
| 端口被占用 | `kill $(lsof -t -i:8080)` 或修改配置端口 |
| 配置错误 | 检查 YAML 语法，运行 `--test` 验证 |
| 权限不足 | `sudo chown -R nas-os:nas-os /var/lib/nas-os` |
| 依赖缺失 | `apt install -y smartmontools btrfs-progs` |

### 服务频繁重启

**诊断：**

```bash
# 查看重启历史
systemctl status nas-os | grep -A 5 "Active:"

# 查看崩溃日志
journalctl -u nas-os -g "panic\|fatal\|error" --since "1 hour ago"

# 检查 OOM
dmesg | grep -i "out of memory"
journalctl -k | grep -i oom
```

**解决方案：**

```bash
# 增加 OOM 评分保护
echo -500 > /proc/$(pidof nasd)/oom_score_adj

# 或在 service 文件中配置
# /etc/systemd/system/nas-os.service
[Service]
OOMScoreAdjust=-500
MemoryMax=2G
```

### 服务响应缓慢

**诊断：**

```bash
# 检查 API 响应时间
curl -w "Time: %{time_total}s\n" http://localhost:8080/api/v1/health

# 检查 goroutine 数量
curl http://localhost:8080/debug/pprof/goroutine?debug=1 | head -20

# 检查 CPU 使用
top -p $(pgrep nasd) -H

# 检查内存使用
pmap -x $(pgrep nasd) | tail -5
```

---

## 存储问题

### 磁盘空间不足

**诊断：**

```bash
# 查看磁盘使用
df -h
du -sh /* | sort -hr | head -10

# 查找大文件
find /mnt -type f -size +100M -exec ls -lh {} \; 2>/dev/null | head -20

# Btrfs 特有检查
btrfs filesystem df /mnt/data
btrfs device usage /mnt/data
```

**清理建议：**

```bash
# 清理日志
journalctl --vacuum-time=7d
rm -rf /var/log/*.gz /var/log/*.1

# 清理包缓存
apt clean
apt autoremove

# 清理 Docker
docker system prune -a --volumes

# 清理旧快照（谨慎）
btrfs subvolume list /mnt/data | sort -r | tail -n +10 | xargs -I {} btrfs subvolume delete {}
```

### Btrfs 阵列降级

**症状：** 存储池显示 degraded 状态

**诊断：**

```bash
# 查看阵列状态
btrfs filesystem show
btrfs device stats /mnt/data

# 查看详细错误
dmesg | grep -i btrfs | tail -20

# 检查设备健康
smartctl -a /dev/sda
smartctl -a /dev/sdb
```

**修复步骤：**

```bash
# 1. 挂载为降级模式（数据可能丢失）
mount -o degraded /dev/sda1 /mnt/recovery

# 2. 添加新设备替换
btrfs device add /dev/sdc1 /mnt/data
btrfs device replace /dev/sdb1 /dev/sdc1 /mnt/data

# 3. 重建 RAID
btrfs balance start -dconvert=raid1 -mconvert=raid1 /mnt/data

# 4. 验证
btrfs filesystem show
btrfs scrub start /mnt/data
```

### 磁盘 I/O 性能问题

**诊断：**

```bash
# 查看磁盘 I/O 统计
iostat -x 1 10

# 查看进程 I/O
iotop -o

# 测试磁盘性能
fio --name=randwrite --ioengine=libaio --iodepth=16 --rw=randwrite --bs=4k --direct=1 --size=1G --numjobs=4 --runtime=60 --group_reporting

# 检查 I/O 调度器
cat /sys/block/sda/queue/scheduler
```

**优化建议：**

```bash
# 更改 I/O 调度器（SSD 推荐 none/mq-deadline）
echo none > /sys/block/sda/queue/scheduler

# 增加队列深度
echo 128 > /sys/block/sda/queue/nr_requests
```

---

## 网络问题

### SMB 共享无法访问

**诊断：**

```bash
# 检查 Samba 服务
systemctl status smbd
smbstatus

# 检查端口
ss -tulpn | grep -E '445|139'

# 测试本地连接
smbclient -L localhost -U%

# 检查防火墙
ufw status
iptables -L -n | grep 445

# 查看连接日志
journalctl -u smbd -f
```

**常见问题：**

| 问题 | 解决方案 |
|------|---------|
| 认证失败 | `smbpasswd -a username` 添加用户 |
| 权限问题 | 检查 SELinux/AppArmor，`chmod/chown` 调整 |
| 无法浏览 | 检查 `workgroup` 设置，启用 NetBIOS |
| 连接慢 | 禁用 `dns proxy`，检查 `socket options` |

### NFS 共享无法挂载

**诊断：**

```bash
# 检查 NFS 服务
systemctl status nfs-server
exportfs -v

# 检查端口
ss -tulpn | grep -E '2049|111|20048'

# 测试本地挂载
showmount -e localhost
mount -t nfs localhost:/export/test /mnt/test

# 检查防火墙
firewall-cmd --list-services
```

**解决方案：**

```bash
# 重启 NFS 服务
systemctl restart nfs-server
systemctl restart rpcbind

# 重新导出
exportfs -ra

# 允许防火墙
firewall-cmd --add-service=nfs --permanent
firewall-cmd --add-service=rpc-bind --permanent
firewall-cmd --add-service=mountd --permanent
firewall-cmd --reload
```

### Web UI 无法访问

**诊断：**

```bash
# 检查服务
curl -v http://localhost:8080

# 检查监听
ss -tlnp | grep 8080

# 检查反向代理（如有）
nginx -t
caddy validate --config /etc/caddy/Caddyfile

# 检查 SSL 证书
openssl s_client -connect localhost:443 -servername nas.local
```

---

## 备份与快照

### 备份任务失败

**诊断：**

```bash
# 查看备份日志
journalctl -u nas-os -t backup --since "1 day ago"

# 检查备份存储空间
df -h /backup

# 检查备份任务配置
cat /etc/nas-os/backup.yaml

# 手动运行测试
/usr/libexec/nas-os/backup-runner.sh --dry-run
```

**常见错误：**

| 错误 | 原因 | 解决方案 |
|------|------|---------|
| `ENOSPC` | 空间不足 | 清理旧备份或扩容 |
| `EACCES` | 权限问题 | 检查 SSH 密钥/密码 |
| `ETIMEDOUT` | 网络超时 | 增加 timeout 设置 |
| `rsync error 23` | 部分文件无法传输 | 检查文件权限/打开状态 |

### 快照创建失败

**诊断：**

```bash
# 检查 Btrfs 快照
btrfs subvolume list /mnt/data

# 检查快照空间
btrfs filesystem df /mnt/data

# 检查快照配额
btrfs qgroup show /mnt/data
```

**解决方案：**

```bash
# 启用配额
btrfs quota enable /mnt/data

# 清理旧快照
btrfs subvolume delete /mnt/data/.snapshots/old-snapshot

# 重建快照配额
btrfs qgroup create 1/100 /mnt/data
```

---

## 性能问题

### CPU 使用率高

**诊断：**

```bash
# 实时监控
top -H -p $(pgrep nasd)

# 生成 CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof -http=:8081 cpu.prof

# 查看热点函数
go tool pprof -top cpu.prof
```

### 内存泄漏

**诊断：**

```bash
# 监控内存
watch -n 1 'ps -p $(pgrep nasd) -o rss,vsz,pmem'

# 生成内存 profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof -http=:8081 heap.prof

# 查看 goroutine 泄漏
curl http://localhost:8080/debug/pprof/goroutine?debug=1 | grep -c "goroutine"
```

**解决方案：**

```bash
# 限制内存使用
systemctl edit nas-os
# 添加:
[Service]
MemoryMax=2G
MemoryHigh=1.5G
```

---

## 监控告警

### Prometheus 指标异常

**诊断：**

```bash
# 检查 metrics 端点
curl http://localhost:8080/metrics | grep -E "nas_os_"

# 检查 Prometheus 连接
curl http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | select(.labels.job=="nas-os")'

# 验证告警规则
promtool check rules /etc/prometheus/alerts.yml
```

### Alertmanager 通知失败

**诊断：**

```bash
# 检查 Alertmanager 状态
curl http://localhost:9093/api/v2/status

# 查看告警
curl http://localhost:9093/api/v2/alerts

# 测试邮件发送
amtool alert add --alertmanager.url=http://localhost:9093 \
  alertname=TestAlert severity=critical service=test

# 检查配置
amtool config show --alertmanager.url=http://localhost:9093
```

---

## 日志分析

### 日志位置

| 日志 | 路径 |
|------|------|
| NAS-OS 服务 | `journalctl -u nas-os` |
| Samba | `/var/log/samba/` |
| NFS | `/var/log/nfs.log` |
| 系统 | `/var/log/syslog`, `/var/log/messages` |
| 审计 | `/var/log/audit/audit.log` |

### 常用日志命令

```bash
# 实时查看错误
journalctl -u nas-os -f -p err

# 查看最近 1 小时
journalctl -u nas-os --since "1 hour ago"

# 搜索关键词
journalctl -u nas-os -g "error|failed|panic"

# 导出日志
journalctl -u nas-os --since "2024-01-01" > nas-os.log
```

### 日志级别调整

```bash
# 临时调整
nasd --log-level debug

# 永久调整（配置文件）
# /etc/nas-os/config.yaml
logging:
  level: debug
  format: json
  output: /var/log/nas-os/debug.log
```

---

## 紧急恢复

### 重置管理员密码

```bash
# 进入恢复模式
nasd --reset-admin

# 或手动修改
sqlite3 /var/lib/nas-os/nas-os.db "UPDATE users SET password='$(echo -n 'newpassword' | sha256sum | cut -d' ' -f1)' WHERE username='admin';"
```

### 恢复出厂设置

```bash
# 备份配置
cp -r /etc/nas-os /etc/nas-os.backup

# 重置配置
nasd --factory-reset

# 注意：此操作会清除所有配置，数据不受影响
```

### 数据紧急恢复

```bash
# 从 Btrfs 快照恢复
btrfs subvolume snapshot /mnt/data/.snapshots/daily-2024-01-01 /mnt/data/recovered

# 从备份恢复
rsync -avz --progress /backup/latest/ /mnt/data/
```

---

## 获取帮助

1. **文档**: `docs/` 目录
2. **日志**: `journalctl -u nas-os -n 100`
3. **社区**: GitHub Discussions
4. **Issue**: GitHub Issues (附带日志和系统信息)

### 系统信息收集

```bash
# 收集诊断信息
nasctl diag --output diag-$(date +%Y%m%d).tar.gz

# 或手动收集
{
  echo "=== System Info ==="
  uname -a
  cat /etc/os-release
  
  echo -e "\n=== NAS-OS Version ==="
  nasd --version
  
  echo -e "\n=== Service Status ==="
  systemctl status nas-os
  
  echo -e "\n=== Recent Logs ==="
  journalctl -u nas-os -n 50
  
  echo -e "\n=== Disk Info ==="
  df -h
  btrfs filesystem show
  
  echo -e "\n=== Memory ==="
  free -h
  
  echo -e "\n=== Network ==="
  ip addr
  ss -tulpn
} > nas-os-diag.txt
```

---

**最后更新**: 2026-03-16
**版本**: v2.147.0
**维护**: 工部 (DevOps Team)