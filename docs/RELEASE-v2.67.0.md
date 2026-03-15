# NAS-OS v2.67.0 发布说明

**发布日期**: 2026-03-15  
**版本类型**: Stable  

---

## 📝 版本概述

v2.67.0 是一个维护版本，主要包含代码质量优化和文档体系完善。

---

## ✨ 变更亮点

### 代码质量优化 (兵部)
- cost_analysis 模块代码格式化
- 静态分析问题修复
- 代码规范统一

### 文档体系完善 (礼部)
- README.md 版本更新至 v2.67.0
- docs/ 目录文档版本同步
- API 文档版本更新
- 用户指南版本同步

### 安全审计完善 (刑部)
- 安全审计报告更新
- 依赖安全检查

---

## 📦 安装方式

### 二进制文件

```bash
# AMD64
wget https://github.com/crazyqin/nas-os/releases/download/v2.67.0/nasd-linux-amd64

# ARM64
wget https://github.com/crazyqin/nas-os/releases/download/v2.67.0/nasd-linux-arm64

# ARMv7
wget https://github.com/crazyqin/nas-os/releases/download/v2.67.0/nasd-linux-armv7
```

### Docker

```bash
docker pull ghcr.io/crazyqin/nas-os:v2.67.0
```

---

## 🔄 升级指南

从 v2.66.0 升级到 v2.67.0：

1. **备份数据**（推荐）
   ```bash
   # 备份配置文件
   cp -r /etc/nas-os /etc/nas-os.backup
   ```

2. **停止服务**
   ```bash
   sudo systemctl stop nasd
   ```

3. **更新二进制**
   ```bash
   # 下载新版本
   wget https://github.com/crazyqin/nas-os/releases/download/v2.67.0/nasd-linux-amd64
   chmod +x nasd-linux-amd64
   sudo mv nasd-linux-amd64 /usr/local/bin/nasd
   ```

4. **启动服务**
   ```bash
   sudo systemctl start nasd
   ```

### Docker 升级

```bash
docker stop nasd
docker rm nasd
docker pull ghcr.io/crazyqin/nas-os:v2.67.0
docker run -d --name nasd ... ghcr.io/crazyqin/nas-os:v2.67.0
```

---

## 📋 完整变更日志

详见 [CHANGELOG.md](../CHANGELOG.md)

---

## 🙏 贡献者

感谢所有参与此版本开发的贡献者！

---

**NAS-OS 团队**