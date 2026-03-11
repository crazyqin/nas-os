# {{version}}

{{date}}

## 🎯 版本亮点

<!-- 简要描述本版本的主要特性 -->

## ✨ 新特性

<!-- 列出新增功能 -->

## 🐛 Bug 修复

<!-- 列出修复的问题 -->

## ⚠️ 重要说明

<!-- 升级注意事项、破坏性变更等 -->

## 📦 下载

### 二进制文件

| 平台 | 文件 | SHA256 |
|------|------|--------|
| Linux x86_64 | `nasd-linux-amd64` | {{checksum_amd64}} |
| Linux ARM64 | `nasd-linux-arm64` | {{checksum_arm64}} |
| Linux ARMv7 | `nasd-linux-armv7` | {{checksum_armv7}} |

### Docker

```bash
docker pull ghcr.io/{{repository}}:{{version}}
```

## 📋 变更日志

{{changelog}}

## 🔍 校验

下载后请验证文件完整性：

```bash
sha256sum -c checksums.txt
```

## 🙏 致谢

感谢所有贡献者！
