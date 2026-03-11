# NAS-OS API 文档

## 概述

NAS-OS 提供完整的 RESTful API，用于管理存储卷、用户、共享、网络配置等。

## 访问 API 文档

### Swagger UI (交互式文档)

启动服务后访问：

```
http://localhost:8080/swagger/index.html
```

Swagger UI 提供：
- 完整的 API 端点列表
- 交互式测试功能
- 请求/响应示例
- 参数说明

### OpenAPI 规范

- JSON 格式: `http://localhost:8080/openapi.json`
- YAML 格式: `http://localhost:8080/openapi.yaml`

## 生成文档

### 使用 Makefile

```bash
# 生成 Swagger 文档
make swagger

# 生成静态 HTML 文档 (使用 Redoc)
make swagger-html

# 导出多种格式 (JSON/YAML/HTML/Markdown)
make docs-export

# 启动本地文档服务器
make docs-serve
```

### 手动生成

```bash
# 安装 swag CLI
go install github.com/swaggo/swag/cmd/swag@latest

# 生成文档
swag init -g cmd/nasd/main.go -o docs/swagger --parseDependency --parseInternal
```

## API 模块

| 模块 | 路径前缀 | 说明 |
|------|---------|------|
| 卷管理 | `/api/v1/volumes` | Btrfs 存储卷创建、管理 |
| 子卷管理 | `/api/v1/volumes/:name/subvolumes` | 子卷 CRUD |
| 快照管理 | `/api/v1/volumes/:name/snapshots` | 快照创建、恢复、删除 |
| 用户管理 | `/api/v1/users` | 用户和用户组管理 |
| 认证 | `/api/v1/login`, `/api/v1/logout` | 登录认证 |
| 共享管理 | `/api/v1/shares` | SMB 和 NFS 共享 |
| 网络管理 | `/api/v1/network` | 网络接口、DDNS、防火墙 |
| Docker | `/api/v1/docker` | 容器、镜像管理 |
| 插件 | `/api/v1/plugins` | 插件安装、配置 |
| 配额 | `/api/v1/quota` | 存储配额管理 |
| 性能监控 | `/api/v1/perf` | 系统性能指标 |
| 系统 | `/api/v1/system` | 系统信息、健康检查 |

## 认证

API 支持 JWT 认证：

```bash
# 登录获取令牌
curl -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"password"}'

# 使用令牌访问 API
curl http://localhost:8080/api/v1/volumes \
  -H "Authorization: Bearer <token>"
```

## 文件结构

```
docs/
├── swagger/           # Swagger 生成的文档
│   ├── docs.go       # Go 文档代码
│   ├── swagger.json  # OpenAPI JSON 规范
│   └── swagger.yaml  # OpenAPI YAML 规范
├── html/             # 静态 HTML 文档
│   └── api-docs.html # Redoc 生成的 HTML
├── export/           # 导出的多格式文档
│   ├── openapi.json
│   ├── openapi.yaml
│   ├── API.md
│   └── api-docs.html
└── API_README.md     # 本文档
```

## 更新文档

添加新的 API 注释后，运行 `make swagger` 重新生成文档。

注释示例：

```go
// listVolumes 列出所有卷
// @Summary 列出所有卷
// @Description 获取系统中所有 Btrfs 卷的列表
// @Tags volumes
// @Accept json
// @Produce json
// @Success 200 {object} GenericResponse "成功"
// @Router /volumes [get]
func (s *Server) listVolumes(c *gin.Context) {
    // ...
}
```

## 第三方工具集成

### Postman

导入 OpenAPI 规范：

1. 打开 Postman
2. Import → Upload Files
3. 选择 `docs/swagger/swagger.json`

### Insomnia

1. 创建新 Collection
2. Import → From File
3. 选择 `docs/swagger/swagger.yaml`