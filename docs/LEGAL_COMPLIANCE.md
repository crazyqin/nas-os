# NAS-OS 法务合规报告

**生成时间:** 2026-03-10  
**审查范围:** 开源许可证、第三方依赖、数据隐私、知识产权  
**项目位置:** ~/clawd/nas-os

---

## 1. 开源许可证审查

### 1.1 项目许可证状态

**⚠️ 问题:** 项目根目录 **缺少 LICENSE 文件**

- README.md 中声明使用 **MIT License**
- 但实际未包含 LICENSE 文件

**✅ 建议:**
```bash
cd ~/clawd/nas-os
# 创建 MIT LICENSE 文件
cat > LICENSE << 'EOF'
MIT License

Copyright (c) 2026 NAS-OS Project

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
EOF
```

### 1.2 许可证兼容性

MIT License 是**宽松型许可证**，允许：
- ✅ 商业使用
- ✅ 修改代码
- ✅ 分发
- ✅ 私有使用

**唯一要求:** 保留版权声明和许可证文本

---

## 2. 第三方依赖许可证检查

### 2.1 核心依赖分析

| 依赖包 | 许可证 | 风险等级 | 说明 |
|--------|--------|----------|------|
| `github.com/gin-gonic/gin` | MIT | ✅ 低风险 | Web 框架 |
| `golang.org/x/crypto` | BSD-3-Clause | ✅ 低风险 | 加密库 |
| `github.com/bytedance/sonic` | Apache-2.0 | ✅ 低风险 | JSON 序列化 |
| `github.com/go-playground/validator/v10` | MIT | ✅ 低风险 | 参数验证 |
| `google.golang.org/protobuf` | BSD-3-Clause | ✅ 低风险 | Protocol Buffers |
| `github.com/quic-go/quic-go` | MIT | ✅ 低风险 | QUIC 协议 |
| `go.mongodb.org/mongo-driver/v2` | Apache-2.0 | ✅ 低风险 | MongoDB 驱动 |

### 2.2 许可证兼容性总结

所有依赖均使用**宽松型许可证** (MIT/BSD/Apache-2.0)，与项目 MIT 许可证**完全兼容**。

**✅ 无需担心许可证冲突**

### 2.3 建议行动

1. **自动化许可证检查** (可选):
```bash
# 安装 go-licenses 工具
go install github.com/google/go-licenses@latest

# 检查依赖许可证
go-licenses check ./...

# 生成许可证报告
go-licenses report ./... > THIRD_PARTY_LICENSES.txt
```

2. **维护第三方许可证清单**:
   - 在 `docs/THIRD_PARTY_LICENSES.md` 记录所有依赖的许可证
   - 在发布前更新

---

## 3. 数据隐私合规建议

### 3.1 当前数据处理情况

**已识别的数据类型:**

| 数据类型 | 存储位置 | 加密状态 | 风险等级 |
|----------|----------|----------|----------|
| 用户名 | 内存 (users.Manager) | ❌ 未加密 | 🟡 中 |
| 密码哈希 | 内存 (bcrypt) | ✅ 已哈希 | 🟢 低 |
| 用户邮箱 | 内存 | ❌ 未加密 | 🟡 中 |
| 会话令牌 | 内存 | ❌ 未加密 | 🟡 中 |
| 系统日志 | 标准输出 | ❌ 明文 | 🟡 中 |

### 3.2 隐私合规风险

#### 🔴 高风险问题

1. **无数据持久化加密**
   - 用户数据仅存储在内存中（当前实现）
   - 未来如需持久化，必须加密存储

2. **无传输加密配置**
   - 当前 HTTP 明文传输
   - 生产环境必须启用 HTTPS

3. **默认密码问题**
   - 默认管理员密码: `admin123`
   - **必须**在首次启动时强制修改

#### 🟡 中风险问题

1. **会话管理**
   - 令牌存储在内存中，重启后失效
   - 无令牌刷新机制
   - 令牌有效期固定 24 小时，无法配置

2. **日志脱敏**
   - 请求日志可能包含敏感信息
   - 建议脱敏处理

3. **无审计日志**
   - 用户操作无审计追踪
   - 无法追溯安全事件

### 3.3 GDPR/隐私法规合规建议

#### 必须实施 (生产环境)

1. **数据传输加密**
```go
// 启用 HTTPS
// internal/web/server.go
func (s *Server) StartTLS(addr, certFile, keyFile string) error {
    s.httpSrv = &http.Server{
        Addr:    addr,
        Handler: s.engine,
    }
    return s.httpSrv.ListenAndServeTLS(certFile, keyFile)
}
```

2. **密码策略强化**
```go
// 密码复杂度要求
// - 最少 8 字符
// - 包含大小写字母、数字、特殊字符
// - 禁止常见弱密码
```

3. **用户同意机制**
   - 首次使用需同意隐私政策
   - 记录同意时间戳

4. **数据导出/删除功能**
   - 支持用户导出个人数据
   - 支持账户注销和数据删除

#### 建议实施

1. **隐私政策文档**
   - 创建 `PRIVACY_POLICY.md`
   - 说明数据收集、使用、存储方式

2. **数据最小化**
   - 仅收集必要信息
   - 邮箱字段当前为可选，保持

3. **访问控制**
   - 实现基于角色的 API 访问控制
   - 敏感操作需要二次认证

4. **审计日志**
```go
// 记录关键操作
type AuditLog struct {
    UserID    string    `json:"user_id"`
    Action    string    `json:"action"`
    Resource  string    `json:"resource"`
    Timestamp time.Time `json:"timestamp"`
    IP        string    `json:"ip"`
}
```

### 3.4 合规检查清单

- [ ] 添加 LICENSE 文件
- [ ] 启用 HTTPS (生产环境)
- [ ] 强制首次登录修改密码
- [ ] 实现密码复杂度策略
- [ ] 添加隐私政策文档
- [ ] 实现审计日志
- [ ] 支持数据导出/删除
- [ ] 日志脱敏处理
- [ ] 会话令牌可配置过期时间
- [ ] 添加速率限制防止暴力破解

---

## 4. 知识产权风险评估

### 4.1 代码原创性

**✅ 低风险** - 项目代码为原创开发

- 未发现抄袭或未经授权复制的代码
- 依赖均通过正规渠道 (go mod) 引入

### 4.2 商标风险

**⚠️ 注意:**

1. **项目名称 "NAS-OS"**
   - 建议进行商标检索
   - 避免与现有产品重名

2. **第三方商标使用**
   - btrfs、Samba、Docker 等均为注册商标
   - 文档中应使用™或®标注
   - 声明与相关公司无关联

### 4.3 专利风险

**🟡 中等关注:**

1. **文件系统技术**
   - btrfs 涉及多项专利
   - 通过 Linux 内核使用，已有专利保护

2. **数据同步/RAID 技术**
   - 部分 RAID 算法可能有专利
   - 建议使用标准实现

### 4.4 开源贡献合规

如需接受外部贡献:

1. **添加贡献者协议**
   - DCO (Developer Certificate of Origin)
   - 或 CLA (Contributor License Agreement)

2. **代码审查流程**
   - 确保贡献代码无版权问题
   - 检查许可证兼容性

---

## 5. 总结与优先级

### 🔴 高优先级 (立即处理)

1. **添加 LICENSE 文件** - 5 分钟
2. **修改默认密码机制** - 30 分钟
3. **启用 HTTPS 支持** - 1 小时

### 🟡 中优先级 (发布前完成)

1. **实现密码复杂度策略** - 2 小时
2. **添加审计日志** - 4 小时
3. **编写隐私政策** - 2 小时
4. **会话管理优化** - 2 小时

### 🟢 低优先级 (长期改进)

1. **自动化许可证检查** - 1 小时
2. **商标检索** - 外部服务
3. **数据导出/删除功能** - 4 小时
4. **贡献者协议** - 1 小时

---

## 附录：参考资源

- [MIT License 全文](https://opensource.org/licenses/MIT)
- [GDPR 合规指南](https://gdpr.eu/)
- [OWASP 认证会话管理](https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html)
- [Go 安全最佳实践](https://go.dev/doc/security)

---

*本报告由刑部 - 法务合规自动生成 | 下次审查：2026-04-10*
