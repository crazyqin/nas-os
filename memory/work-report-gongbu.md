# 工部工作报告 - AI 服务 Docker 镜像优化

**日期**: 2026-03-26
**任务**: 验证并优化 AI 服务 Docker 镜像
**状态**: ✅ 已完成

---

## 发现的问题

### 1. ❌ 致命问题：`cmd/nas-ai` 目录不存在

原 `Dockerfile.ai` 尝试编译 `./cmd/nas-ai`，但该目录不存在：
- 实际只有 `cmd/nasd`、`cmd/nasctl`、`cmd/backup`
- AI 功能集成在 `nasd` 主服务中（`internal/ai/`）
- 构建会失败

### 2. ⚠️ 镜像过大问题

原 GPU 镜像基于 `nvidia/cuda:12.2.0-runtime-ubuntu22.04`：
- 基础镜像约 4GB
- PyTorch GPU 约 2GB
- 总镜像约 6-8GB

### 3. ⚠️ 资源限制缺失

原 `docker-compose.ai.yml` 只有 GPU 预留，缺少：
- CPU/内存限制
- 资源预留配置
- 日志轮转配置

### 4. ⚠️ 健康检查超时过短

原 `start_period: 60s` 对于需要加载模型的服务过短

---

## 实施的修复

### 1. 重写 `Dockerfile.ai`

**变更内容**：
- 移除不存在的 `cmd/nas-ai` 编译步骤
- 创建独立的 Python AI 服务入口
- 保留 GPU 和 CPU 两个构建目标
- 优化分层：分离 PyTorch 和其他依赖便于缓存

**新结构**：
```
Dockerfile.ai
├── gpu-base (nvidia/cuda:12.2.0-runtime-ubuntu22.04)
│   └── Python venv + PyTorch GPU + CLIP + InsightFace
└── ai-cpu (python:3.11-slim)
    └── PyTorch CPU + CLIP + InsightFace
```

### 2. 创建 AI 服务入口脚本

**新文件**: `scripts/ai-entrypoint.py`

**功能**：
- FastAPI 服务框架
- 健康检查端点 (`/health`, `/ready`)
- CLIP API 占位 (`/api/v1/clip/*`)
- 人脸识别 API 占位 (`/api/v1/face/*`)
- PII 脱敏 API（已实现基础功能）
- OpenAI 兼容 API 占位 (`/v1/chat/completions`)

### 3. 优化 `docker-compose.ai.yml`

**新增配置**：
```yaml
deploy:
  resources:
    limits:
      cpus: '8.0'
      memory: 16G
    reservations:
      cpus: '2.0'
      memory: 4G
      devices: [nvidia gpu]

logging:
  driver: "json-file"
  options:
    max-size: "10m"
    max-file: "3"
```

**改进**：
- 添加 CPU/内存限制和预留
- 添加日志轮转配置
- 增加健康检查 `start_period` 到 120s
- 添加模型缓存环境变量

### 4. 修复 `docker-compose.ai.cpu.yml`

**改进**：
- 移除 GPU 设备预留
- 设置 CPU 合理资源限制
- 添加 `CUDA_VISIBLE_DEVICES=""` 禁用 GPU

---

## NVIDIA GPU Runtime 集成

### 验证方法

在支持 NVIDIA GPU 的主机上执行：

```bash
# 1. 验证 NVIDIA runtime
docker info | grep -i nvidia

# 2. 测试 GPU 镜像
docker run --gpus all nvidia/cuda:12.2.0-runtime-ubuntu22.04 nvidia-smi

# 3. 构建 AI 镜像
docker build -f Dockerfile.ai --target gpu-base -t nas-ai:test .

# 4. 运行并测试
docker run --gpus all -p 8081:8081 nas-ai:test
curl http://localhost:8081/health
```

### 集成要点

1. **基础镜像**: `nvidia/cuda:12.2.0-runtime-ubuntu22.04` 包含 CUDA runtime
2. **环境变量**:
   - `NVIDIA_VISIBLE_DEVICES=all`
   - `NVIDIA_DRIVER_CAPABILITIES=compute,utility`
3. **PyTorch GPU**: 使用 `--index-url https://download.pytorch.org/whl/cu121`
4. **Docker Compose**: `deploy.resources.reservations.devices` 配置

---

## 镜像大小分析

| 组件 | GPU 版本 | CPU 版本 |
|------|----------|----------|
| 基础镜像 | ~4GB (cuda) | ~150MB (slim) |
| PyTorch | ~2GB (cu121) | ~200MB (cpu) |
| 其他依赖 | ~500MB | ~300MB |
| **预计总大小** | **~6-7GB** | **~600-800MB** |

### 进一步优化建议

1. **使用 cuDNN runtime 镜像**: `nvidia/cuda:12.2.0-cudnn8-runtime` 更小
2. **分离模型存储**: 模型文件用 volume 存储，不打包进镜像
3. **多阶段构建优化**: 只复制虚拟环境目录

---

## 文件变更清单

| 文件 | 状态 | 说明 |
|------|------|------|
| `Dockerfile.ai` | ✏️ 重写 | 修复构建错误，优化结构 |
| `docker-compose.ai.yml` | ✏️ 更新 | 添加资源限制、日志配置 |
| `docker-compose.ai.cpu.yml` | ✏️ 更新 | 修复配置，移除无效字段 |
| `scripts/ai-entrypoint.py` | ✨ 新增 | AI 服务入口脚本 |

---

## 后续建议

1. **实现 AI 功能**：当前 API 为占位实现，需要集成 CLIP/InsightFace
2. **添加监控指标**：暴露 Prometheus 指标
3. **CI/CD 集成**：添加镜像构建和推送 workflow
4. **安全扫描**：集成 Trivy 扫描镜像漏洞

---

**工部**
*2026-03-26*