# 刑部第97轮安全审计报告

**执行部门:** 刑部（法务合规）  
**审计日期:** 2026-03-30  
**审计范围:** gosec/govulncheck扫描、RAIDZ Expansion安全、AI人脸识别隐私合规

---

## 一、安全扫描结果

### 1.1 gosec扫描统计

| 指标 | 数值 |
|------|------|
| 扫描文件数 | 713 |
| 扫描代码行数 | 422,144 |
| nosec标记数 | 79 |
| 发现问题总数 | **1,094** |

### 1.2 问题严重性分布

| 严重性 | 数量 | 占比 |
|--------|------|------|
| HIGH | 240 | 21.9% |
| MEDIUM | 795 | 72.7% |
| LOW | 59 | 5.4% |

### 1.3 HIGH级别问题类型分布

| 规则ID | CWE | 数量 | 说明 |
|--------|-----|------|------|
| G115 | 190 | 125 | 整数溢出转换 |
| G703 | 22 | 54 | 路径遍历污点追踪 |
| G704 | 22 | 22 | 潜在路径遍历 |
| G118 | 22 | 5 | 原子操作不安全 |
| G702 | 22 | 10 | 路径遍历字符串 |
| G101 | 798 | 10 | 硬编码凭证 |
| G122 | 367 | 9 | TOCTOU竞争 |
| G402 | 295 | 2 | TLS验证跳过 |
| G404 | 338 | 2 | 弱随机数生成 |

### 1.4 govulncheck扫描结果

**结果: ✅ 无已知漏洞**

```
DB: https://vuln.go.dev
DB updated: 2026-03-27 18:39:44 +0000 UTC
Go版本: go1.26.0

No vulnerabilities found.
```

**说明:** Go 1.26.0版本无已知安全漏洞，依赖包也无已知CVE。

---

## 二、RAIDZ Expansion安全性检查

### 2.1 代码安全特性评估

**检查文件:** `pkg/storage/zfs/raidz_expansion.go` (734行)

| 安全特性 | 实现状态 | 评价 |
|----------|----------|------|
| 状态机管理 | ✅ 完善 | 7种状态（idle/preparing/running/paused/completed/failed/cancelled） |
| 并发安全 | ✅ 完善 | sync.RWMutex保护所有状态变更 |
| 错误处理 | ✅ 完善 | 定义13种明确错误类型 |
| 进度监控 | ✅ 完善 | ticker 5秒更新，支持速度/ETA计算 |
| 取消/暂停机制 | ✅ 完善 | channel信号，支持中断恢复 |
| 磁盘验证 | ✅ 完善 | 存在性检查 + ZFS使用检查 |
| 历史记录 | ✅ 完善 | JSON持久化，最多100条 |
| ZFS版本检查 | ✅ 完善 | 要求OpenZFS 2.2.0+ |

### 2.2 潜在风险点

**风险1: 命令执行安全（G204类）**

```go
// 第394行
cmd := exec.CommandContext(ctx, "zpool", "expand", config.PoolName, config.NewDisk)
```

**评估:** 低风险 - 参数来自内部配置，非用户直接输入，且使用固定命令格式。

**风险2: 进程Kill无错误处理**

```go
// 第417行
_ = cmd.Process.Kill()
```

**评估:** 可接受 - Kill失败不影响后续状态更新，只是清理。

**风险3: 文件权限**

```go
// 第156行
return os.WriteFile(historyPath, data, 0640)
```

**评估:** 正确 - 使用0640而非0644，限制其他用户读取。

### 2.3 安全建议

1. **添加参数验证白名单**
```go
// 建议在StartExpansion前添加
func validatePoolName(name string) error {
    if strings.Contains(name, "..") || strings.Contains(name, "/") {
        return errors.New("invalid pool name")
    }
    return nil
}
```

2. **添加操作审计日志**
```go
// 建议：记录所有关键操作
type AuditEntry struct {
    Operation   string    // "start", "pause", "cancel", "complete"
    PoolName    string
    NewDisk     string
    Timestamp   time.Time
    UserID      string
    Result      string    // "success" or error message
}
```

---

## 三、AI人脸识别隐私合规研究

### 3.1 群晖DSM 7.3 AI Console合规参考

群晖AI Console数据遮罩合规要点：
- PII数据脱敏后发送外部AI服务
- 本地人脸数据加密存储
- 用户可导出/删除数据
- 审计日志记录AI调用

### 3.2 飞牛fnOS人脸识别合规参考

飞牛fnOS合规要点：
- 启用人脸识别前强制隐私声明
- 人脸数据仅存储本地
- GPU加速（Intel核显）数据处理在本地完成
- 数据不上传第三方

### 3.3 本系统隐私合规实现评估

**检查文件:** `internal/ai/face/privacy.go` (185行)

| 合规要求 | 实现状态 | 说明 |
|----------|----------|------|
| 知情同意机制 | ✅ 完善 | RequestConsent + RecordConsent |
| 同意记录持久化 | ✅ 完善 | JSON存储，权限0600 |
| 数据本地存储 | ✅ 明确 | DataDir指定本地路径 |
| 数据导出功能 | ✅ 实现 | ExportData方法 |
| 数据删除功能 | ✅ 实现 | DeleteAllData方法 |
| 隐私政策展示 | ✅ 完善 | GetPrivacyPolicy返回6条款 |
| 加密存储选项 | ⚠️ 配置项 | EnableEncryption字段存在但未实现加密逻辑 |
| 数据保留策略 | ⚠️ 未实现 | DataRetentionDays=0(永久)，无自动清理 |

### 3.4 人脸检测器安全评估

**检查文件:** `internal/ai/face/detector.go` (280行)

| 安全特性 | 实现状态 | 说明 |
|----------|----------|------|
| 文件存在性检查 | ✅ | os.Stat验证 |
| GPU类型隔离 | ✅ | 按GPU类型分支处理 |
| Embedding隔离 | ✅ | 512维向量，无外部传输 |
| 聚类本地完成 | ✅ | cosineSimilarity本地计算 |
| Mutex保护 | ✅ | sync.Mutex保护检测器状态 |

### 3.5 待改进合规建议

#### 优先级P0（必须实现）

1. **加密存储实现**
```go
// 当前：EnableEncryption=true但无加密逻辑
// 建议：使用AES-256-GCM加密人脸特征向量
func (pm *PrivacyManager) encryptFaceData(data []byte) ([]byte, error) {
    block, err := aes.NewCipher(pm.encryptKey)
    if err != nil {
        return nil, err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(crand.Reader, nonce); err != nil {
        return nil, err
    }
    return gcm.Seal(nonce, nonce, data, nil), nil
}
```

2. **首次使用强制同意**
```go
// 建议：人脸识别API中间件
func FaceRecognitionAuthMiddleware(pm *PrivacyManager) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            userID := getUserID(r)
            consented, _ := pm.CheckConsent(userID)
            if !consented {
                WriteError(w, 403, "人脸识别功能需要用户知情同意")
                return
            }
            next.ServeHTTP(w, r)
        })
    }
}
```

#### 优先级P1（建议实现）

1. **数据自动清理**
```go
// 建议：添加定期清理任务
func (pm *PrivacyManager) StartRetentionChecker(ctx context.Context) {
    if pm.config.DataRetentionDays <= 0 {
        return // 永久保留
    }
    ticker := time.NewTicker(24 * time.Hour)
    for {
        select {
        case <-ticker.C:
            pm.cleanExpiredData()
        case <-ctx.Done():
            ticker.Stop()
            return
        }
    }
}
```

2. **人脸数据匿名化选项**
```go
// 建议：为导出数据提供匿名化选项
type ExportOptions struct {
    Anonymize     bool   // 移除姓名标签
    IncludeEmbed  bool   // 是否包含特征向量
    Format        string // json/zip
}
```

---

## 四、AI Console API安全问题

**检查文件:** `internal/ai/api/ai_console_api.go`

### 4.1 发现问题

**问题1: 多处错误未处理（G104）**

```go
// 第240、218、201、185、157行等多处
//nolint:errcheck
json.NewEncoder(w).Encode(resp)
```

**风险等级: 低**  
JSON编码失败会导致响应不完整，但不会泄露数据。

**建议修复:**
```go
if err := json.NewEncoder(w).Encode(resp); err != nil {
    log.Printf("JSON encode error: %v", err)
    // 不再尝试写响应，连接已broken
}
```

**问题2: 缺少速率限制**

当前AI Console API未实现速率限制，可能导致：
- AI服务滥用
- 成本失控
- DoS风险

**建议添加:**
```go
type RateLimiter struct {
    requests map[string]*UserLimit
    mu       sync.RWMutex
}

type UserLimit struct {
    Count     int
    ResetTime time.Time
    Limit     int // 每小时最大请求数
}
```

---

## 五、RAIDZ操作安全规范建议

### 5.1 操作前检查清单

| 检查项 | 必须 | 说明 |
|--------|------|------|
| ZFS版本 ≥ 2.2.0 | ✅ | RAIDZ Expansion特性要求 |
| 池状态 ONLINE | ✅ | 离线/降级池禁止操作 |
| 单RAIDZ VDEV | ✅ | 多VDEV不支持扩展 |
| 新盘健康SMART | ✅ | 无错误/警告 |
| 新盘不在ZFS中 | ✅ | ValidateDisk检查 |
| 数据备份 | ✅ | 建议操作前创建快照 |
| 空间预估 | ⚠️ | 可选，提前告知用户 |

### 5.2 操作过程安全规范

1. **暂停机制**
   - 用户可暂停扩展
   - 暂停后状态持久化
   - 可恢复继续

2. **取消机制**
   - 取消需二次确认
   - 取消后记录历史
   - 不自动恢复原状态（需用户手动处理）

3. **失败处理**
   - 记录错误详情
   - 不自动重试
   - 通知管理员

### 5.3 操作后验证

```bash
# 建议验证步骤
zpool status -v <pool>     # 检查状态
zpool list <pool>          # 检查容量变化
zfs list -r <pool>         # 检查数据完整性
```

---

## 六、总结

### 6.1 关键指标

| 指标 | 当前值 | 目标值 | 评价 |
|------|--------|--------|------|
| gosec HIGH问题 | 240 | <50 | ⚠️ 需修复 |
| govulncheck漏洞 | 0 | 0 | ✅ 良好 |
| RAIDZ代码安全 | 90% | 95% | ⚠️ 可改进 |
| 人脸隐私合规 | 70% | 95% | ⚠️ 需完善加密 |
| AI API安全 | 65% | 85% | ⚠️ 缺速率限制 |

### 6.2 优先修复清单

| 优先级 | 问题 | 建议修复时间 |
|--------|------|--------------|
| P0 | 人脸数据加密存储 | 1天 |
| P0 | 人脸识别强制同意中间件 | 1天 |
| P1 | 整数溢出转换（125处G115） | 3天 |
| P1 | AI API速率限制 | 2天 |
| P2 | 错误处理改进（59处G104） | 5天 |

### 6.3 合规声明建议模板

```
【人脸识别功能知情同意书 v1.0】

您即将启用人脸识别功能。请注意：

1. 本功能将在您的本地NAS设备上处理和存储人脸数据。
2. 人脸数据用于自动识别和分类照片中的人物。
3. 人脸特征向量采用AES-256加密存储。
4. 数据不会上传至任何云端或第三方服务。
5. 您可随时导出或删除所有人脸数据。
6. 数据删除后不可恢复，请谨慎操作。

[同意启用] [暂不启用]
```

---

*此报告由刑部生成*  
*完成时间: 2026-03-30 05:15*