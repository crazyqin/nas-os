# NAS-OS 法务合规文档

**文档版本:** 1.0  
**最后更新:** 2026-03-26  
**审查部门:** 刑部（法务合规）

---

## 一、项目许可证

### 1.1 项目主许可证

NAS-OS 项目采用 **MIT License** 发布。

```
MIT License

Copyright (c) 2025 nas-os

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
```

### 1.2 许可证特性

MIT License 是宽松型开源许可证，允许：
- ✅ 商业使用
- ✅ 修改代码
- ✅ 分发
- ✅ 私有使用
- ✅ 闭源衍生作品

**唯一要求：** 保留版权声明和许可证文本。

---

## 二、AI服务模块合规性

### 2.1 支持的AI提供商

NAS-OS AI服务模块 (`internal/ai/`) 支持以下AI提供商：

| 提供商 | 类型 | 数据流向 | 合规状态 |
|--------|------|----------|----------|
| OpenAI | 云端API | 数据发送至OpenAI服务器 | ⚠️ 需用户授权 |
| DeepSeek | 云端API | 数据发送至DeepSeek服务器 | ⚠️ 需用户授权 |
| 智谱AI (GLM) | 云端API | 数据发送至智谱服务器 | ⚠️ 需用户授权 |
| 通义千问 (Qwen) | 云端API | 数据发送至阿里云服务器 | ⚠️ 需用户授权 |
| Moonshot (Kimi) | 云端API | 数据发送至Moonshot服务器 | ⚠️ 需用户授权 |
| **本地LLM (Ollama)** | 本地部署 | **数据不离开设备** | ✅ 完全合规 |
| **本地LLM (LocalAI)** | 本地部署 | **数据不离开设备** | ✅ 完全合规 |

### 2.2 本地LLM许可证调研

#### Ollama

- **许可证:** MIT License
- **版权归属:** Ollama
- **许可证特点:**
  - 允许商业使用
  - 允许修改和分发
  - 允许闭源衍生作品
  - 无GPL传染风险
- **仓库地址:** https://github.com/ollama/ollama
- **合规状态:** ✅ 与本项目MIT许可证完全兼容

```
MIT License

Copyright (c) Ollama

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
```

#### LocalAI

- **许可证:** MIT License
- **版权归属:** Ettore Di Giacinto (mudler@localai.io)
- **许可证特点:**
  - 允许商业使用
  - 允许修改和分发
  - 允许闭源衍生作品
  - 无GPL传染风险
- **仓库地址:** https://github.com/mudler/LocalAI
- **合规状态:** ✅ 与本项目MIT许可证完全兼容

```
MIT License

Copyright (c) 2023-2025 Ettore Di Giacinto (mudler@localai.io)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.
```

### 2.3 第三方AI模型许可证注意事项

当用户使用NAS-OS连接本地LLM服务时，模型的许可证取决于用户下载的具体模型：

| 模型系列 | 典型许可证 | 商业使用 |
|----------|-----------|----------|
| Llama 2/3 | Llama Community License | 有条件允许 |
| Mistral | Apache 2.0 | ✅ 允许 |
| Qwen | Apache 2.0 / Qwen License | 有条件允许 |
| DeepSeek | MIT License | ✅ 允许 |
| Gemma | Gemma Terms of Use | 有条件允许 |
| Phi系列 | MIT License | ✅ 允许 |

**建议：** 用户应自行确认所使用模型的许可证条款。

---

## 三、隐私合规要求

### 3.1 数据处理原则

NAS-OS AI服务遵循以下数据保护原则：

1. **数据最小化**: 仅收集必要的AI请求数据
2. **本地优先**: 优先支持本地LLM，确保数据不离开设备
3. **用户知情**: 明确告知数据将发送至第三方AI服务
4. **数据脱敏**: 提供PII（个人身份信息）自动脱敏功能

### 3.2 PII脱敏功能

AI服务模块实现了完善的PII脱敏机制 (`internal/ai/service.go`)：

```go
// 默认PII保护规则
func DefaultDeIDRules() []DeIDRule {
    return []DeIDRule{
        {Name: "idcard", Pattern: `\d{17}[\dXx]`, Replacement: "[ID]", Enabled: true},  // 身份证
        {Name: "credit_card", Pattern: `\d{16}`, Replacement: "[CARD]", Enabled: true}, // 信用卡
        {Name: "phone", Pattern: `\d{11}`, Replacement: "[PHONE]", Enabled: true},      // 手机号
        {Name: "email", Pattern: `[\w.-]+@[\w.-]+\.\w+`, Replacement: "[EMAIL]", Enabled: true},
        {Name: "ip_address", Pattern: `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`, Replacement: "[IP]", Enabled: true},
    }
}
```

**脱敏数据类型：**
- 身份证号 (18位)
- 信用卡号 (16位)
- 手机号码 (11位)
- 电子邮箱
- IP地址

### 3.3 数据处理流程

```
用户输入 → [PII脱敏] → AI请求 → [云端/本地] → AI响应 → [PII还原] → 返回用户
```

### 3.4 隐私合规建议

#### 使用云端AI服务时

⚠️ **重要提示：** 使用云端AI服务（OpenAI、DeepSeek等）时：

1. 用户数据将发送至第三方服务器
2. 需在用户界面明确告知用户
3. 建议在设置页面添加数据传输声明
4. 建议启用PII脱敏功能

#### 使用本地LLM时

✅ **完全合规：** 使用Ollama/LocalAI等本地LLM时：

1. 数据完全在用户设备处理
2. 不涉及数据传输
3. 符合最严格的隐私法规要求
4. 适合处理敏感数据场景

---

## 四、第三方依赖许可证

### 4.1 核心依赖许可证清单

详见 `LICENSE_CHECK.md`，主要依赖均为宽松许可证：

| 依赖包 | 许可证 | 风险等级 |
|--------|--------|----------|
| github.com/gin-gonic/gin | MIT | ✅ 低 |
| golang.org/x/crypto | BSD-3-Clause | ✅ 低 |
| github.com/bytedance/sonic | Apache-2.0 | ✅ 低 |
| google.golang.org/grpc | Apache-2.0 | ✅ 低 |
| github.com/prometheus/client_golang | Apache-2.0 | ✅ 低 |
| go.opentelemetry.io/otel | Apache-2.0 | ✅ 低 |

### 4.2 许可证兼容性

所有依赖许可证（MIT、Apache-2.0、BSD系列）与本项目MIT许可证**完全兼容**。

**无GPL/AGPL依赖，无传染风险。**

---

## 五、合规检查清单

### 5.1 许可证合规

- [x] 项目包含LICENSE文件
- [x] 使用宽松型开源许可证（MIT）
- [x] 第三方依赖无GPL/AGPL许可证
- [x] Ollama许可证兼容（MIT）
- [x] LocalAI许可证兼容（MIT）

### 5.2 隐私合规

- [x] 实现PII自动脱敏功能
- [x] 支持本地LLM（数据不离开设备）
- [x] AI请求审计日志
- [x] 用户可选择的AI提供商

### 5.3 待改进项

- [ ] 在WebUI中添加AI服务隐私声明
- [ ] 添加AI功能使用协议确认
- [ ] 提供更详细的AI数据处理说明文档

---

## 六、附录

### 6.1 相关文档

- `LICENSE` - 项目主许可证
- `LICENSE_CHECK.md` - 依赖许可证检查报告
- `docs/LEGAL_COMPLIANCE.md` - 通用法务合规报告
- `docs/compliance/` - 合规相关文档

### 6.2 外部资源

- [MIT License全文](https://opensource.org/licenses/MIT)
- [Ollama许可证](https://github.com/ollama/ollama/blob/main/LICENSE)
- [LocalAI许可证](https://github.com/mudler/LocalAI/blob/master/LICENSE)
- [GDPR合规指南](https://gdpr.eu/)
- [个人信息保护法](http://www.npc.gov.cn/npc/c30834/202108/a8c4e3676c74491a80b53a172bb753fe.shtml)

---

## 七、审计日志合规

### 7.1 数据处理声明

NAS-OS 审计日志用于：
- 安全事件追溯和取证
- 合规审计要求
- 系统性能分析

审计日志记录内容包括：
- 用户登录/登出事件
- 文件访问操作（SMB/NFS）
- 系统配置变更
- 权限管理操作

### 7.2 用户权利

根据GDPR和中国《个人信息保护法》，用户有权：

| 权利 | 说明 | API |
|------|------|-----|
| 访问权 | 查看自己的审计日志记录 | `GET /api/v1/audit/export` |
| 删除权 | 请求删除审计日志 | `DELETE /api/v1/audit/user/{user_id}` |
| 可携带权 | 导出审计日志数据（JSON/CSV格式） | `GET /api/v1/audit/export?format=json` |

### 7.3 数据保留策略

| 日志类型 | 保留期限 | 说明 |
|----------|----------|------|
| 常规文件操作 | 90天 | 读/写/打开/关闭 |
| 敏感操作 | 1年 | 删除/权限变更 |
| 管理操作 | 1年 | 系统配置/用户管理 |
| 安全事件 | 1年 | 登录失败/异常访问 |

### 7.4 数据安全措施

- **传输加密**: 审计日志传输使用TLS加密
- **存储加密**: 敏感日志字段加密存储
- **访问控制**: 仅授权管理员可查看完整日志
- **完整性保护**: 日志链式哈希校验
- **自动清理**: 过期日志自动删除

### 7.5 第三方数据传输

使用云端AI服务时：
- 审计日志**不包含**AI请求内容
- AI请求内容由AI服务模块单独处理
- 启用PII脱敏时，敏感数据在发送前已脱敏
- 详见 [AI服务隐私合规](#三隐私合规要求)

### 7.6 合规认证目标

- [ ] ISO 27001 信息安全管理体系
- [ ] 等保2.0 三级认证
- [ ] GDPR 合规评估
- [ ] 《个人信息保护法》合规评估

---

*本文档由刑部（法务合规）维护 | 下次审查日期：2026-06-26*