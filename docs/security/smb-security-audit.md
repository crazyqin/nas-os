# SMB 服务安全审计报告

**审计日期**: 2026-04-04  
**审计范围**: SMB/CIFS服务配置与安全  
**参考**: TrueNAS SMB Spotlight安全设计

---

## 审计项目

### 1. SMB配置安全

| 检查项 | 状态 | 说明 |
|--------|------|------|
| SMB协议版本 | ✅ | SMB3优先，SMB1禁用 |
| 加密传输 | ✅ | SMB3加密支持 |
| 认证机制 | ✅ | Kerberos/NTLMv2 |
| 签名验证 | ✅ | 强制签名配置 |

### 2. Spotlight搜索安全评估

参考 TrueNAS 26 SMB Spotlight 特性，评估nas-os实现需求：

| 安全要点 | TrueNAS实现 | nas-os建议 |
|----------|-------------|------------|
| 访问控制 | ACL过滤 | 实现搜索ACL |
| 索引范围 | 共享级别限制 | 共享级索引配置 |
| 加密排除 | 加密数据集不索引 | 继承现有策略 |
| 性能隔离 | 独立索引进程 | 独立服务设计 |

### 3. SMB Spotlight技术预研

```go
// macOS Spotlight集成架构设计
type SMBSpotlightService struct {
    Indexer      *SpotlightIndexer   // macOS兼容索引
    ACLFilter    *SearchACLFilter    // 权限过滤
    QueryHandler *SpotlightQuery     // macOS查询协议
}
```

**技术要点**:
- 支持macOS Spotlight查询协议 (mdbulkquery)
- 索引数据格式兼容macOS
- ACL实时过滤机制

### 4. 安全建议

| 优先级 | 建议 | 说明 |
|--------|------|------|
| **P0** | SMB Spotlight ACL | 搜索权限过滤 |
| **P1** | SMB Stateful Failover | 企业HA需求 |
| **P2** | SMB审计日志增强 | 操作追踪 |

---

## 审计结论

**整体安全评级**: ✅ 通过

SMB服务配置符合安全标准。Spotlight功能需后续开发，建议纳入M113里程碑。

---

*刑部 2026-04-04*