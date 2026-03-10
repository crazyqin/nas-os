# GitHub Release v0.2.0 Alpha 准备清单

**创建时间**: 2026-03-10  
**状态**: ✅ 准备完成

---

## ✅ 已完成

### 1. 二进制文件构建
- [x] `nasd-linux-amd64` (22MB) - SHA256: `623ff6fe40bc1182d05808682079277dc64bef60692486ad991d9617000ec4f5`
- [x] `nasd-linux-arm64` (20MB) - SHA256: `1f6f0e78085de25294657dd8b17c4c1cdf57fc8b159e9389a468abb50eb0d59d`
- [x] `nasd-linux-armv7` (21MB) - SHA256: `84e9e9688d6f0589fe122a5d4bf10ed8fe061b18404437041a77ab326cd227b0`

**位置**: `/home/mrafter/clawd/nas-os/`

### 2. 发布说明文档
- [x] `docs/RELEASE-v0.2.0.md` - 完整发布说明
- [x] `docs/CHANGELOG.md` - 更新变更日志

**内容包含**:
- 新功能列表
- 技术规格
- 已知问题
- 下载链接
- 升级指南
- 验证步骤

### 3. README 更新
- [x] 更新版本号 (v0.2.0 Alpha)
- [x] 添加二进制下载说明
- [x] 添加 Docker 部署说明
- [x] 更新 API 接口文档
- [x] 更新开发计划状态
- [x] 添加配置示例
- [x] 添加快速使用指南

### 4. 发布脚本
- [x] `scripts/create-release-v0.2.0.sh` - GitHub Release 创建脚本

---

## ⏳ 待完成

### 创建 GitHub Release

**方式一：使用脚本 (推荐)**
```bash
cd /home/mrafter/clawd/nas-os

# 1. 登录 GitHub
gh auth login

# 2. 运行发布脚本
./scripts/create-release-v0.2.0.sh

# 3. 在 GitHub 上检查并发布草稿
# https://github.com/nas-os/nasd/releases
```

**方式二：手动创建**
1. 访问 https://github.com/nas-os/nasd/releases/new
2. Tag version: `v0.2.0`
3. Release title: `NAS-OS v0.2.0 Alpha`
4. 上传文件:
   - `nasd-linux-amd64`
   - `nasd-linux-arm64`
   - `nasd-linux-armv7`
5. 发布说明参考 `docs/RELEASE-v0.2.0.md`
6. 勾选 "Set as the latest release"
7. 点击 "Publish release"

**方式三：使用 Git 标签**
```bash
cd /home/mrafter/clawd/nas-os

# 创建并推送标签
git tag -a v0.2.0 -m "Release NAS-OS v0.2.0 Alpha"
git push origin v0.2.0

# 然后在 GitHub UI 上创建 Release
```

---

## 📋 发布说明摘要

### 核心功能
- ✅ SMB/CIFS 文件共享
- ✅ NFS 文件共享
- ✅ 配置持久化 (YAML)
- ✅ btrfs 完整功能

### 下载链接
- https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-amd64
- https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-arm64
- https://github.com/nas-os/nasd/releases/download/v0.2.0/nasd-linux-armv7

### Docker 镜像
```bash
docker pull nas-os/nasd:v0.2.0
```

---

## 📊 文件清单

```
nas-os/
├── nasd-linux-amd64          ✅ 22MB
├── nasd-linux-arm64          ✅ 20MB
├── nasd-linux-armv7          ✅ 21MB
├── README.md                 ✅ 已更新
├── scripts/
│   └── create-release-v0.2.0.sh  ✅ 已创建
└── docs/
    ├── RELEASE-v0.2.0.md     ✅ 已创建
    └── CHANGELOG.md          ✅ 已更新
```

---

## ⚠️ 注意事项

1. **GitHub 认证**: 需要配置 `gh auth login` 或使用 GitHub UI 手动创建
2. **仓库配置**: 需要配置 git remote (目前未配置)
3. **发布前检查**: 建议在发布前测试二进制文件功能

---

## 🔗 相关链接

- GitHub Releases: https://github.com/nas-os/nasd/releases
- 发布说明：`docs/RELEASE-v0.2.0.md`
- 安装指南：`docs/INSTALL.md`
- 变更日志：`docs/CHANGELOG.md`

---

*准备完成时间：2026-03-10 23:26*  
*准备部门：吏部*
