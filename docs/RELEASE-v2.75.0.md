# NAS-OS v2.75.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable  

---

## 版本亮点

v2.75.0 是一个质量提升版本，专注于代码质量和测试体系的持续改进。

- **代码质量提升** - 静态分析问题修复，代码格式规范化
- **测试增强** - 单元测试和集成测试覆盖率提升

---

## 改进内容

### 代码质量提升 (兵部)

- 代码静态分析问题修复
- 代码格式规范化 (gofmt)
- 代码结构优化

### 测试增强 (兵部)

- 单元测试覆盖率提升
- 集成测试场景完善
- 测试用例优化

### 文档同步 (礼部)

- 版本号统一更新至 v2.75.0
- Docker 镜像标签更新

---

## 升级说明

### 从 v2.74.0 升级

```bash
# 下载新版本
wget https://github.com/crazyqin/nas-os/releases/download/v2.75.0/nasd-linux-amd64
chmod +x nasd-linux-amd64
sudo mv nasd-linux-amd64 /usr/local/bin/nasd

# 重启服务
sudo systemctl restart nas-os
```

### Docker 升级

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.75.0
docker stop nasd
docker rm nasd
docker run -d \
  --name nasd \
  --restart unless-stopped \
  -p 8080:8080 \
  -v /data:/data \
  -v /etc/nas-os:/config \
  ghcr.io/crazyqin/nas-os:v2.75.0
```

---

## 下载链接

| 平台 | 架构 | 下载链接 |
|------|------|----------|
| Linux | AMD64 | [nasd-linux-amd64](https://github.com/crazyqin/nas-os/releases/download/v2.75.0/nasd-linux-amd64) |
| Linux | ARM64 | [nasd-linux-arm64](https://github.com/crazyqin/nas-os/releases/download/v2.75.0/nasd-linux-arm64) |
| Linux | ARMv7 | [nasd-linux-armv7](https://github.com/crazyqin/nas-os/releases/download/v2.75.0/nasd-linux-armv7) |
| Docker | Multi-arch | `ghcr.io/crazyqin/nas-os:v2.75.0` |

---

**六部协同** - 兵部负责代码质量和测试，礼部负责文档更新