# NAS-OS v2.42.0 Release Notes

**发布日期**: 2026-03-15  
**版本类型**: Stable

## 修复

### 🔧 编译错误修复 (司礼监)
- ResourceAlert 结构体添加 Status 字段
- 修复 CI/CD 构建失败问题
- internal/reports/resource_report.go 类型定义完善

## 技术细节

### ResourceAlert 结构体更新
```go
type ResourceAlert struct {
    ID          string    `json:"id"`
    Type        string    `json:"type"`
    Severity    string    `json:"severity"`
    Message     string    `json:"message"`
    Timestamp   time.Time `json:"timestamp"`
    Status      string    `json:"status"` // 新增字段
}
```

## 升级指南

### Docker 升级
```bash
docker pull ghcr.io/crazyqin/nas-os:v2.42.0
docker stop nasd
docker rm nasd
# 使用新镜像启动
```

### 二进制升级
```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.42.0/nasd-linux-$(uname -m)
chmod +x nasd-linux-$(uname -m)
sudo mv nasd-linux-$(uname -m) /usr/local/bin/nasd
```

## 贡献者

- 司礼监：编译错误修复

---

**完整变更日志**: [CHANGELOG.md](../CHANGELOG.md)