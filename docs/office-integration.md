# OnlyOffice 文档编辑集成部署指南

## 目录

1. [系统要求](#系统要求)
2. [部署架构](#部署架构)
3. [快速开始](#快速开始)
4. [生产环境部署](#生产环境部署)
5. [配置 NAS-OS 集成](#配置-nas-os-集成)
6. [前端集成](#前端集成)
7. [安全配置](#安全配置)
8. [性能优化](#性能优化)
9. [故障排除](#故障排除)

## 系统要求

### OnlyOffice Document Server

| 要求 | 最低配置 | 推荐配置 |
|------|----------|----------|
| CPU | 2 核 | 4 核+ |
| 内存 | 4 GB | 8 GB+ |
| 存储 | 20 GB | 50 GB+ |
| Docker | 20.10+ | 最新版 |

### 支持的文档格式

- **文档**: doc, docx, docm, dot, dotx, dotm, odt, fodt, ott, rtf, txt, html, pdf
- **表格**: xls, xlsx, xlsm, xlt, xltx, xltm, ods, fods, ots, csv
- **演示**: ppt, pptx, pptm, pot, potx, potm, odp, fodp, otp, ppsx

## 部署架构

```
┌─────────────────────────────────────────────────────────────────┐
│                         用户浏览器                               │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │  OnlyOffice Editor JS (从 Document Server 加载)           │  │
│  └───────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
                    │                           │
                    │ 编辑器 API                │ 文档数据
                    ▼                           ▼
┌───────────────────────────────┐  ┌───────────────────────────────┐
│   OnlyOffice Document Server   │  │         NAS-OS Server         │
│   ┌─────────────────────────┐  │  │  ┌─────────────────────────┐  │
│   │   docservice           │  │  │  │  Office Module          │  │
│   │   (文档转换服务)        │◄─┼──┼──┤  (会话管理)             │  │
│   └─────────────────────────┘  │  │  └─────────────────────────┘  │
│   ┌─────────────────────────┐  │  │  ┌─────────────────────────┐  │
│   │   converter            │  │  │  │ File Accessor           │  │
│   │   (格式转换)           │  │  │  │ (文件访问接口)          │  │
│   └─────────────────────────┘  │  │  └─────────────────────────┘  │
└───────────────────────────────┘  └───────────────────────────────┘
                    │                           │
                    │ 回调通知                   │ 文件读写
                    └───────────────────────────┘
```

## 快速开始

### 1. 创建 Docker 网络

```bash
# 创建 NAS-OS 使用的网络
docker network create nas-network
```

### 2. 启动 OnlyOffice Document Server

```bash
# 使用提供的 docker-compose 配置
cd /path/to/nas-os
docker-compose -f docker-compose.onlyoffice.yml up -d

# 等待服务启动（约 1-2 分钟）
docker logs -f nas-onlyoffice
```

### 3. 验证服务

```bash
# 访问健康检查端点
curl http://localhost:8081/healthcheck

# 访问 Welcome 页面
open http://localhost:8081/welcome/
```

## 生产环境部署

### 1. 外部数据库配置

创建 `docker-compose.onlyoffice.prod.yml`:

```yaml
version: '3.8'

services:
  onlyoffice:
    image: onlyoffice/documentserver:latest
    container_name: nas-onlyoffice
    restart: unless-stopped
    environment:
      - JWT_ENABLED=true
      - JWT_SECRET=${JWT_SECRET}
      - DB_TYPE=postgres
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_NAME=onlyoffice
      - DB_USER=onlyoffice
      - DB_PWD=${DB_PASSWORD}
      - REDIS_SERVER_HOST=redis
      - REDIS_SERVER_PORT=6379
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy

  postgres:
    image: postgres:15-alpine
    environment:
      - POSTGRES_DB=onlyoffice
      - POSTGRES_USER=onlyoffice
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - postgres-data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U onlyoffice"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 5s
      retries: 5

volumes:
  postgres-data:
  redis-data:
```

### 2. HTTPS 配置

使用反向代理（如 Nginx）:

```nginx
server {
    listen 443 ssl http2;
    server_name office.your-domain.com;

    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;

    # 安全配置
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;

    client_max_body_size 100M;

    location / {
        proxy_pass http://127.0.0.1:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket 支持
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }
}
```

## 配置 NAS-OS 集成

### 1. 更新 OnlyOffice 配置

```bash
# 设置 OnlyOffice 服务器地址和 JWT 密钥
curl -X PUT http://localhost:8080/api/v1/office/config \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "server_url": "https://office.your-domain.com",
    "secret_key": "your-jwt-secret-key",
    "enabled": true,
    "callback_auth": true
  }'
```

### 2. 配置文件访问器

在 NAS-OS 中实现 `FileAccessor` 接口：

```go
// 文件访问器实现示例
type NASFileAccessor struct {
    storageMgr *storage.Manager
}

func (a *NASFileAccessor) GetFileInfo(fileID string) (*FileInfo, error) {
    // 从存储系统获取文件信息
    file, err := a.storageMgr.GetFile(fileID)
    if err != nil {
        return nil, err
    }
    return &FileInfo{
        ID:       file.ID,
        Name:     file.Name,
        Path:     file.Path,
        Size:     file.Size,
        MimeType: file.MimeType,
        OwnerID:  file.OwnerID,
    }, nil
}

func (a *NASFileAccessor) GetFileURL(fileID string) (string, error) {
    // 生成文件访问 URL
    return fmt.Sprintf("https://nas.your-domain.com/api/v1/files/%s/download", fileID), nil
}

func (a *NASFileAccessor) SaveFile(fileID string, reader io.Reader) error {
    // 保存编辑后的文件
    return a.storageMgr.WriteFile(fileID, reader)
}

func (a *NASFileAccessor) GetFilePath(fileID string) (string, error) {
    // 获取文件物理路径
    return a.storageMgr.GetFilePath(fileID)
}
```

### 3. 初始化 Office Manager

```go
// 在 server.go 中初始化
fileAccessor := &NASFileAccessor{storageMgr: storMgr}
officeMgr, err := office.NewManager(
    "/etc/nas-os/office.json",
    fileAccessor,
)
if err != nil {
    log.Printf("⚠️ Office 模块初始化警告: %v", err)
}

// 注册路由
officeHandlers := office.NewHandlers(officeMgr)
officeHandlers.RegisterRoutes(api.Group("/api/v1"))
```

## 前端集成

### React 示例

```tsx
import { useEffect, useRef } from 'react';

interface OfficeEditorProps {
  fileId: string;
  mode?: 'edit' | 'view';
}

export function OfficeEditor({ fileId, mode = 'edit' }: OfficeEditorProps) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let editor: any;

    async function initEditor() {
      // 获取编辑器配置
      const response = await fetch(`/api/v1/office/edit/${fileId}`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ mode }),
      });

      const { data } = await response.json();

      // 加载 OnlyOffice API
      const script = document.createElement('script');
      script.src = data.editor_url;
      script.onload = () => {
        // 初始化编辑器
        editor = new (window as any).DocsAPI.DocEditor(containerRef.current, {
          document: data.editor_config.document,
          documentType: data.editor_config.document_type,
          editorConfig: data.editor_config.editor,
          height: '100%',
          width: '100%',
        });
      };
      document.head.appendChild(script);
    }

    initEditor();

    return () => {
      if (editor) {
        editor.destroyEditor();
      }
    };
  }, [fileId, mode]);

  return (
    <div
      ref={containerRef}
      style={{ height: '100vh', width: '100%' }}
    />
  );
}
```

### Vue 示例

```vue
<template>
  <div ref="editorContainer" class="office-editor"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';

const props = defineProps<{
  fileId: string;
  mode?: 'edit' | 'view';
}>();

const editorContainer = ref<HTMLDivElement>();
let editor: any = null;

onMounted(async () => {
  const response = await fetch(`/api/v1/office/edit/${props.fileId}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ mode: props.mode || 'edit' }),
  });

  const { data } = await response.json();

  // 加载 OnlyOffice API
  const script = document.createElement('script');
  script.src = data.editor_url;
  script.onload = () => {
    editor = new (window as any).DocsAPI.DocEditor(editorContainer.value, {
      document: data.editor_config.document,
      documentType: data.editor_config.document_type,
      editorConfig: data.editor_config.editor,
      height: '100%',
      width: '100%',
    });
  };
  document.head.appendChild(script);
});

onUnmounted(() => {
  if (editor) {
    editor.destroyEditor();
  }
});
</script>

<style scoped>
.office-editor {
  height: 100vh;
  width: 100%;
}
</style>
```

## 安全配置

### 1. JWT 密钥

```bash
# 生成强密钥
openssl rand -base64 32

# 配置到 OnlyOffice
JWT_SECRET=$(openssl rand -base64 32)
echo "JWT_SECRET=$JWT_SECRET" > .env
```

### 2. 网络隔离

```yaml
# 只允许 NAS-OS 访问 OnlyOffice
services:
  onlyoffice:
    networks:
      - internal
    # 不暴露到外部
    # ports:
    #   - "8081:80"

networks:
  internal:
    internal: true  # 内部网络，无法访问外网
```

### 3. 访问控制

```go
// 在中间件中检查权限
func OfficeAuthMiddleware(officeMgr *office.Manager) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 检查用户是否有权限编辑文件
        fileID := c.Param("fileId")
        userID := c.GetString("user_id")

        if !checkFilePermission(userID, fileID, "edit") {
            c.JSON(403, gin.H{"error": "没有权限"})
            c.Abort()
            return
        }

        c.Next()
    }
}
```

## 性能优化

### 1. 资源限制

```yaml
services:
  onlyoffice:
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 8G
```

### 2. 并发编辑限制

```go
// 在 manager.go 中添加
const MaxConcurrentEditors = 50

func (m *Manager) CreateSession(fileID, userID, userName, mode string) (*EditingSession, *EditorInitConfig, error) {
    m.mu.RLock()
    if len(m.sessions) >= MaxConcurrentEditors {
        m.mu.RUnlock()
        return nil, nil, errors.New("达到最大并发编辑数")
    }
    // ...
}
```

### 3. 缓存优化

```yaml
services:
  redis:
    image: redis:7-alpine
    command: redis-server --maxmemory 1gb --maxmemory-policy allkeys-lru
```

## 故障排除

### 常见问题

#### 1. 编辑器无法加载

```bash
# 检查 OnlyOffice 服务状态
docker logs nas-onlyoffice --tail 100

# 检查网络连通性
docker exec nas-onlyoffice curl -I http://localhost/healthcheck
```

#### 2. 文档无法保存

1. 检查回调 URL 是否可达
2. 验证 JWT Token 配置
3. 检查文件权限

```bash
# 测试回调
curl -X POST http://localhost:8080/api/v1/office/callback/test \
  -H "Content-Type: application/json" \
  -d '{"key": "test", "status": 2, "url": "http://example.com/doc.docx"}'
```

#### 3. 内存不足

```bash
# 增加内存限制
docker update --memory 8g nas-onlyoffice

# 或在 docker-compose 中配置
deploy:
  resources:
    limits:
      memory: 8G
```

### 日志分析

```bash
# 查看 OnlyOffice 日志
docker exec nas-onlyoffice cat /var/log/onlyoffice/documentserver/converter/err.log
docker exec nas-onlyoffice cat /var/log/onlyoffice/documentserver/docservice/err.log

# 查看 NAS-OS 日志
journalctl -u nas-os -f
```