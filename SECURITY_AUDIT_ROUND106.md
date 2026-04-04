# 安全审计报告 Round 106

**审计日期**: 2026-04-04
**版本**: v2.389.0 → v2.390.0
**审计工具**: golangci-lint + go vet + 代码审查
**审计范围**: User-linked API Keys、ZFS Deduplication、路径注入漏洞跟踪

---

## 一、User-linked API Keys 安全设计评估

### 1.1 审计范围

| 模块 | 文件 | 功能 |
|------|------|------|
| AI API Key管理 | `internal/ai/apikey/` | AI服务API密钥管理 |
| 认证API Key | `internal/auth/apikey/` | 用户认证API密钥管理 |
| RBAC集成 | `internal/ai/apikey/rbac_integration.go` | 权限控制集成 |

### 1.2 安全设计评估结果

#### ✅ 加密存储 (AES-256-GCM)

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 加密算法 | ✅ 安全 | AES-256-GCM，业界标准 |
| 密钥管理 | ✅ 安全 | 系统自动生成32字节密钥 |
| 密钥存储 | ✅ 安全 | 文件权限0600，受限访问 |
| 密钥轮换 | ✅ 支持 | 默认90天轮换周期 |
|Nonce生成 | ✅ 安全 | 使用crypto/rand |

**关键代码** (`encryption.go`):
```go
// AES-256-GCM 加密
block, err := aes.NewCipher(km.masterKey)  // 32字节密钥
gcm, err := cipher.NewGCM(block)
nonce := make([]byte, gcm.NonceSize())
rand.Read(nonce)  // crypto/rand
ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
```

#### ✅ 密钥验证与格式检查

| 检查项 | 状态 | 说明 |
|--------|------|------|
| Key Prefix验证 | ✅ 支持 | sk-、sk-ant-等前缀检查 |
| Key Pattern正则 | ✅ 支持 | `^sk-[a-zA-Z0-9]{20,}$` |
| Key Hash存储 | ✅ 安全 | SHA256哈希，不存原文 |
| Key Preview | ✅ 安全 | 仅显示末尾4字符 |

#### ✅ RBAC权限控制

| 权限 | 说明 | 状态 |
|------|------|------|
| `ai:key:create` | 创建密钥 | ✅ 已定义 |
| `ai:key:read` | 读取密钥 | ✅ 已定义 |
| `ai:key:update` | 更新密钥 | ✅ 已定义 |
| `ai:key:delete` | 删除密钥 | ✅ 已定义 |
| `ai:key:use` | 使用密钥 | ✅ 已定义 |
| `ai:key:rotate` | 轮换密钥 | ✅ 已定义 |
| `ai:key:view_secret` | 查看解密密钥 | ✅ 仅管理员/所有者 |

**权限检查逻辑** (`rbac_integration.go`):
```go
// 所有者有完全访问权限
if r.config.OwnerFullAccess && key.OwnerID == userID {
    return true, nil
}
// 管理员有完全访问权限
if r.config.AdminFullAccess && isAdmin {
    return true, nil
}
// 检查用户列表
if containsUser(key.AllowedUsers, userID) {
    return true, nil
}
// 检查组列表
for _, allowedGroup := range key.AllowedGroups {
    // ...
}
```

#### ✅ 使用限制与审计

| 功能 | 状态 | 说明 |
|------|------|------|
| 使用次数限制 | ✅ 支持 | UsageLimit/CostLimit |
| 过期时间 | ✅ 支持 | ExpiresAt字段 |
| 自动禁用过期 | ✅ 支持 | AutoDisableExpired |
| 审计日志 | ✅ 完整 | 所有操作记录 |
| 使用统计 | ✅ 支持 | UsageStats详细记录 |

### 1.3 安全建议

| 优先级 | 建议 | 说明 |
|--------|------|------|
| P1 | 添加密钥泄露检测 | 监控异常使用模式 |
| P1 | 添加IP白名单 | 已有SourceIPs字段，需激活使用 |
| P2 | 添加密钥使用告警 | 超限额/异常时间使用告警 |
| P2 | 实现密钥版本回退 | 轮换失败时可回退旧版本 |

### 1.4 User-linked API Keys 安全评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 加密存储 | 9/10 | AES-256-GCM，密钥管理完善 |
| 权限控制 | 9/10 | RBAC完整， Owner/Admin/User三层 |
| 审计日志 | 9/10 | 所有操作记录完整 |
| 使用限制 | 8/10 | 额度限制完善，IP限制待激活 |
| 代码质量 | 10/10 | golangci-lint 0 issues |

**综合评分**: **9.0/10** ✅ 安全设计优秀

---

## 二、ZFS Deduplication 安全评估

### 2.1 审计范围

| 模块 | 文件 | 功能 |
|------|------|------|
| Dedup核心 | `internal/dedup/dedup.go` | 文件去重核心逻辑 |
| Fast Dedup | `pkg/storage/dedup/fast_dedup.go` | ZFS DDT架构快速去重 |
| Dedup管理 | `internal/dedup/manager.go` | 去重任务管理 |
| Dedup配置 | `internal/dedup/config.go` | 配置管理 |

### 2.2 安全风险分析

#### ✅ 哈希算法安全

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 哈希算法 | ✅ 安全 | SHA-256，抗碰撞 |
| 哈希长度 | ✅ 安全 | 32字节(256位) |
| 哈希冲突检测 | ✅ 支持 | DDTStats.Collisions记录 |

**关键代码** (`dedup.go`):
```go
hash := sha256.Sum256(data)
hashStr := hex.EncodeToString(hash[:])
```

#### ⚠️ 路径遍历风险 (需关注)

| 检查项 | 状态 | 说明 |
|--------|------|------|
| 路径清理 | ⚠️ 部分 | filepath.Clean使用，但未强制基目录 |
| 用户路径提取 | ⚠️ 风险 | extractUserFromPath可能泄露信息 |
| 路径遍历检查 | ⚠️ 部分 | 存在".."检查，但不完整 |

**风险代码** (`dedup.go:310`):
```go
// extractUserFromPath 从路径提取用户
func extractUserFromPath(path string) string {
    patterns := []string{
        "/home/",
        "/Users/",
        "/data/users/",
    }
    // 可能泄露用户目录结构信息
}
```

**建议修复**:
```go
// 应使用 pathutil.SafePath 验证路径
safePath, err := pathutil.SafePath(baseDir, userPath)
if err != nil {
    return nil, err
}
```

#### ✅ 引用计数安全

| 检查项 | 状态 | 说明 |
|--------|------|------|
| RefCount溢出 | ✅ 安全 | uint32，上限4B |
| RefCount原子操作 | ✅ 安全 | atomic操作 |
| RefCount验证 | ✅ 支持 | Dedup时检查引用 |

#### ⚠️ 内存安全风险

| 检查项 | 状态 | 说明 |
|--------|------|------|
| DDT内存限制 | ⚠️ 需验证 | MaxMemoryMB配置存在，需激活 |
| 大文件处理 | ⚠️ 风险 | 无文件大小上限检查 |
| Chunk大小验证 | ✅ 安全 | 有ChunkSize限制 |

**风险代码** (`fast_dedup.go`):
```go
// DedupConfig 有 MaxMemoryMB，但需确认实际使用
MaxMemoryMB uint32 `json:"maxMemoryMB"`
```

#### ✅ 去重操作安全

| 操作 | 安全性 | 说明 |
|------|--------|------|
| 软链接 | ✅ 安全 | 保留属性，有权限检查 |
| 硬链接 | ✅ 安全 | 同软链接 |
| 删除 | ⚠️ 需谨慎 | ActionRemove直接删除，需确认 |
| 报告 | ✅ 安全 | 仅报告不操作 |

### 2.3 安全建议

| 优先级 | 建议 | 说明 |
|--------|------|------|
| P0 | 强制基目录验证 | 所有路径操作使用pathutil.SafePath |
| P0 | 添加文件大小上限 | 防止内存耗尽攻击 |
| P1 | 添加用户隔离 | 跨用户去重需权限确认 |
| P1 | 添加去重操作审计 | 记录所有去重操作 |
| P2 | 添加哈希验证 | Dedup后验证目标文件完整性 |

### 2.4 ZFS Deduplication 安全评分

| 维度 | 评分 | 说明 |
|------|------|------|
| 哈希安全 | 9/10 | SHA-256，抗碰撞 |
| 路径安全 | 6/10 | 需强制基目录验证 |
| 内存安全 | 7/10 | 有配置但需激活 |
| 操作安全 | 8/10 | 链接安全，删除需审计 |
| 代码质量 | 10/10 | golangci-lint 0 issues |

**综合评分**: **7.5/10** ⚠️ 需要改进路径安全

---

## 三、路径注入漏洞跟踪 (Round 105 #736, #737)

### 3.1 高危Alert状态

| Alert | 规则 | 严重性 | 状态 |
|-------|------|--------|------|
| #736 | go/path-injection | 🔴 High | Open |
| #737 | go/path-injection | 🔴 High | Open |

### 3.2 现有防护措施

#### ✅ 已有安全工具

| 工具 | 文件 | 功能 |
|------|------|------|
| SafePath | `internal/security/pathutil/pathutil.go` | 基目录验证+路径遍历检查 |
| SafePath | `pkg/security/sanitize.go` | 路径清理+基目录验证 |
| ValidatePath | `internal/security/pathutil/pathutil.go` | 简单路径验证 |
| SafeJoin | `internal/security/pathutil/pathutil.go` | 安全路径连接 |

**SafePath实现** (`pathutil.go`):
```go
func SafePath(baseDir, userPath string) (string, error) {
    // 清理基目录
    baseDir = filepath.Clean(baseDir)
    // 获取绝对路径
    absBase, err := filepath.Abs(baseDir)
    // 清理用户路径，移除路径分隔符
    cleanPath = strings.TrimPrefix(cleanPath, string(os.PathSeparator))
    // 检查路径遍历
    if strings.Contains(cleanPath, "..") {
        return "", ErrPathTraversal
    }
    // 验证最终路径在基目录内
    relPath, err := filepath.Rel(absBase, absFull)
    if strings.HasPrefix(relPath, "..") {
        return "", ErrPathTraversal
    }
    // 检查符号链接目标
    // ...
}
```

### 3.3 需要修复的模块

| 模块 | 风险等级 | 说明 |
|------|----------|------|
| `internal/dedup/dedup.go` | 🔴 High | FindDuplicates未使用SafePath |
| `internal/files/manager.go` | ⚠️ Medium | 部分路径操作需验证 |
| `internal/webshare/` | ⚠️ Medium | HTTP路径输入需验证 |

### 3.4 修复建议

**优先级P0**: dedup模块路径验证

```go
// dedup.go FindDuplicates 修复
func FindDuplicates(paths []string, config *Config, callback ProgressCallback) (*FindDuplicatesResult, error) {
    // 验证所有路径
    for _, rootPath := range paths {
        safePath, err := pathutil.SafePath(config.BaseDir, rootPath)
        if err != nil {
            return nil, fmt.Errorf("路径验证失败: %w", err)
        }
        paths[i] = safePath
    }
    // ...
}
```

---

## 四、golangci-lint 扫描结果

### 4.1 API Key模块

```bash
$ golangci-lint run --timeout 5m ./internal/ai/apikey/...
0 issues. ✅
```

### 4.2 Dedup模块

```bash
$ golangci-lint run --timeout 5m ./internal/dedup/... ./pkg/storage/dedup/...
0 issues. ✅
```

### 4.3 go vet 结果

```bash
$ go vet ./internal/ai/apikey/... ./internal/dedup/... ./pkg/storage/dedup/...
(no output) ✅
```

---

## 五、编译验证

```bash
$ go build ./...
# 成功，无错误 ✅
```

---

## 六、漏洞修复进度汇总

| 漏洞类型 | 历史数量 | 本轮变化 | 当前状态 |
|----------|----------|----------|----------|
| 命令注入 (G702) | 10 | 无变化 | ⚠️ 已添加 #nosec |
| 路径遍历 (G703) | 49+2 | 跟踪中 | ⚠️ #736, #737 Open |
| 整数溢出 (G115) | 108 | 无变化 | ⚠️ 已排除 |
| User API Keys安全 | - | ✅ 新评估 | **9.0/10** |
| Dedup安全 | - | ✅ 新评估 | **7.5/10** |

---

## 七、审计结论

### 7.1 安全状态评分

| 检查项 | 状态 | 说明 |
|--------|------|------|
| User-linked API Keys | ✅ 优秀 | 加密+RBAC+审计完整 |
| ZFS Deduplication | ⚠️ 需改进 | 路径安全需强化 |
| 路径注入漏洞 | ⚠️ 跟踪中 | #736, #737 待修复 |
| 编译检查 | ✅ 通过 | 无错误 |
| golangci-lint | ✅ 通过 | 0 issues |

### 7.2 下轮任务建议

**P0 优先级**:
1. 修复路径注入 #736, #737 (dedup模块)
2. 强制dedup模块使用pathutil.SafePath

**P1 优先级**:
1. 添加dedup文件大小上限
2. 添加dedup跨用户权限检查
3. 添加API Key IP白名单激活

**持续监控**:
1. 跟踪已排除规则的验证逻辑实现
2. 定期审查 #nosec 注释的有效性

---

## 八、六部协同状态

| 部门 | 安全任务 | 状态 |
|------|----------|------|
| 刑部 | 安全审计报告 Round 106 | ✅ 完成 |
| 刑部 | User API Keys安全评估 | ✅ 完成 |
| 刑部 | ZFS Dedup安全评估 | ✅ 完成 |
| 刑部 | 路径注入漏洞跟踪 | ✅ 已标记 |
| 兵部 | 路径注入修复 | 📋 待分配 |
| 工部 | dedup模块加固 | 📋 待分配 |

---

**审计人**: 刑部安全审计系统
**报告生成时间**: 2026-04-04 15:30 UTC+8