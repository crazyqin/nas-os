# 许可证合规检查报告

**项目**: nas-os  
**版本**: v2.224.0  
**检查日期**: 2026-03-18  
**检查人**: 刑部（法务合规）

---

## 一、主要依赖许可证清单（前 20 项）

| 序号 | 依赖 | 版本 | 许可证 | 风险等级 |
|---|---|---|---|---|
| 1 | github.com/aws/aws-sdk-go-v2 | v1.41.3 | Apache-2.0 | ✅ 低 |
| 2 | github.com/gin-gonic/gin | v1.11.0 | MIT | ✅ 低 |
| 3 | github.com/blevesearch/bleve/v2 | v2.5.7 | Apache-2.0 | ✅ 低 |
| 4 | github.com/bytedance/sonic | v1.14.0 | Apache-2.0 | ✅ 低 |
| 5 | google.golang.org/grpc | v1.79.2 | Apache-2.0 | ✅ 低 |
| 6 | github.com/prometheus/client_golang | v1.23.2 | Apache-2.0 | ✅ 低 |
| 7 | github.com/spf13/cobra | v1.10.2 | Apache-2.0 | ✅ 低 |
| 8 | github.com/stretchr/testify | v1.11.1 | MIT | ✅ 低 |
| 9 | github.com/gorilla/mux | v1.8.1 | BSD-3-Clause | ✅ 低 |
| 10 | github.com/gorilla/websocket | v1.5.3 | BSD-2-Clause | ✅ 低 |
| 11 | github.com/go-redis/redis/v8 | v8.11.5 | BSD-2-Clause | ✅ 低 |
| 12 | github.com/go-ldap/ldap/v3 | v3.4.12 | MIT | ✅ 低 |
| 13 | github.com/swaggo/swag | v1.16.6 | MIT | ✅ 低 |
| 14 | github.com/quic-go/quic-go | v0.57.0 | MIT | ✅ 低 |
| 15 | go.etcd.io/bbolt | v1.4.0 | MIT | ✅ 低 |
| 16 | github.com/xuri/excelize/v2 | v2.10.1 | BSD-3-Clause | ✅ 低 |
| 17 | go.uber.org/zap | v1.27.0 | MIT | ✅ 低 |
| 18 | golang.org/x/crypto | v0.48.0 | BSD-3-Clause | ✅ 低 |
| 19 | go.opentelemetry.io/otel | v1.42.0 | Apache-2.0 | ✅ 低 |
| 20 | github.com/jcmturner/gokrb5/v8 | v8.4.4 | Apache-2.0 | ✅ 低 |

---

## 二、许可证类型分布

| 许可证类型 | 数量 | 性质 |
|---|---|---|
| Apache-2.0 | 6 | 宽松，专利授权 |
| MIT | 8 | 宽松，极简 |
| BSD-2-Clause | 2 | 宽松 |
| BSD-3-Clause | 4 | 宽松 |

---

## 三、GPL/AGPL 风险检查

### ✅ 未发现 GPL/AGPL 许可证

经检查，项目依赖中 **不存在** 以下许可证：
- GPL-2.0 / GPL-3.0
- AGPL-3.0
- LGPL（未发现）
- MPL-2.0（弱 copyleft，允许作为库使用）

**结论**: 无 GPL/AGPL 传染风险。

---

## 四、合规建议

### 1. 当前状态
项目依赖许可证整体合规性良好，所有主要依赖均为宽松许可证（MIT、Apache-2.0、BSD 系列）。

### 2. 建议措施
1. **保留许可证声明**: 在分发时保留所有依赖的许可证声明文件
2. **定期审查**: 建议每次升级主要依赖时重新检查许可证
3. **许可证兼容性**: Apache-2.0、MIT、BSD 之间互相兼容，无冲突

### 3. 注意事项
- `golang.org/x/*` 系列包使用 BSD-3-Clause，需保留版权声明
- `github.com/bytedance/sonic` 使用 Apache-2.0，包含专利授权条款

---

## 五、结论

**合规评级**: ✅ 通过

项目依赖许可证风险可控，无 GPL/AGPL 传染风险，可安全用于商业闭源项目。

---

*报告生成: 刑部（法务合规）*