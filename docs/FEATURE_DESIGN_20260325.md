# 功能设计文档 - 网盘挂载与人脸识别

## 一、概述

本文档定义 nas-os 网盘挂载功能和人脸识别模块的技术设计方案，参考 fnOS 网盘挂载实现和 Synology Photos 人脸识别功能。

## 二、P0: 网盘挂载功能

### 2.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Web API Layer                          │
│                  (cloudfuse/handlers.go)                    │
├─────────────────────────────────────────────────────────────┤
│                    Mount Manager                            │
│                  (cloudfuse/manager.go)                     │
│  - 挂载生命周期管理                                            │
│  - 配置持久化                                                 │
│  - 自动挂载                                                   │
├─────────────────────────────────────────────────────────────┤
│                    FUSE File System                         │
│                   (cloudfuse/fuse_fs.go)                    │
│  - 文件操作映射                                                │
│  - 目录缓存                                                   │
│  - 读写缓冲                                                   │
├─────────────────────────────────────────────────────────────┤
│                    Cloud Providers                          │
│                  (cloudsync/providers.go)                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐       │
│  │  115网盘  │ │ 夸克网盘  │ │ 阿里云盘  │ │ 百度网盘  │       │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘       │
├─────────────────────────────────────────────────────────────┤
│                    Cache Layer                              │
│                  (cloudfuse/cache.go)                      │
│  - 本地缓存                                                   │
│  - LRU 淘汰                                                   │
│  - 预读优化                                                   │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心模块

#### 2.2.1 FUSE 文件系统 (`fuse_fs.go`)

```go
// CloudFileSystem 实现完整的 FUSE 文件系统
type CloudFileSystem struct {
    config     *MountConfig
    provider   cloudsync.Provider
    cache      *FileCache
    dirCache   *DirectoryCache    // 目录列表缓存
    writeBuf   *WriteBuffer       // 写缓冲
    handles    map[uint64]*FileHandle
    nextHandle uint64
    mu         sync.RWMutex
}

// 实现接口:
// - Lookup: 查找文件/目录
// - Getattr: 获取属性
// - Open/Read/Write/Flush/Release: 文件操作
// - Opendir/Readdir/Releasedir: 目录操作
// - Create/Mkdir/Unlink/Rmdir: 创建删除
```

#### 2.2.2 目录缓存

```go
// DirectoryCache 缓存目录列表，减少 API 调用
type DirectoryCache struct {
    entries  map[string]*DirCacheEntry  // path -> entry
    ttl      time.Duration
    maxSize  int
}

type DirCacheEntry struct {
    Files     []cloudsync.FileInfo
    ExpiresAt time.Time
}
```

#### 2.2.3 文件缓存

```go
// FileCache 本地文件缓存，支持断点续传
type FileCache struct {
    cacheDir  string
    maxSize   int64
    usedSize  int64
    lruList   *list.List
    entries   map[string]*CacheEntry
}

// 策略:
// - LRU 淘汰
// - 大文件分块缓存
// - 后台预读
```

### 2.3 数据流

#### 读取流程
```
1. FUSE Read 请求
2. 检查本地缓存
   - 命中: 直接返回
   - 未命中: 从云端下载
3. 更新缓存
4. 返回数据
```

#### 写入流程
```
1. FUSE Write 请求
2. 写入写缓冲区
3. Flush 时上传到云端
4. 清理缓存（如果配置了不缓存写入）
```

### 2.4 关键实现

#### FUSE Read 实现
```go
func (fs *CloudFileSystem) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
    path := req.Header.Node.String()
    
    // 1. 检查缓存
    if data, ok := fs.cache.Get(path, req.Offset, req.Size); ok {
        resp.Data = data
        return nil
    }
    
    // 2. 从云端获取文件
    localPath := fs.cache.GetTempPath(path)
    if err := fs.provider.Download(ctx, path, localPath); err != nil {
        return err
    }
    
    // 3. 加入缓存
    fs.cache.Put(path, localPath)
    
    // 4. 读取数据
    data, err := fs.cache.Read(path, req.Offset, req.Size)
    if err != nil {
        return err
    }
    resp.Data = data
    return nil
}
```

### 2.5 支持的网盘

| 网盘 | 读取 | 写入 | 秒传 | 离线下载 | 流媒体 |
|------|------|------|------|----------|--------|
| 115网盘 | ✅ | ✅ | ✅ | ✅ | ✅ |
| 夸克网盘 | ✅ | ✅ | ✅ | - | ✅ |
| 阿里云盘 | ✅ | ✅ | ✅ | - | ✅ |
| 百度网盘 | ✅ | ✅ | ✅ | ✅ | ✅ |

---

## 三、P1: 人脸识别模块

### 3.1 架构设计

```
┌─────────────────────────────────────────────────────────────┐
│                      Face Service                           │
│                  (ai/photos/service.go)                     │
├─────────────────────────────────────────────────────────────┤
│                    Face Recognizer                          │
│                  (ai/photos/recognizer.go)                  │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐        │
│  │  Detector    │ │  Embedder    │ │  Clusterer   │        │
│  │ (RetinaFace) │ │  (ArcFace)   │ │  (DBSCAN)    │        │
│  └──────────────┘ └──────────────┘ └──────────────┘        │
├─────────────────────────────────────────────────────────────┤
│                    Model Backend                            │
│  ┌──────────────┐ ┌──────────────┐                         │
│  │   go-face    │ │  ONNX Runtime │  (可选)                 │
│  │   (dlib)     │ │   (GPU)       │                         │
│  └──────────────┘ └──────────────┘                         │
├─────────────────────────────────────────────────────────────┤
│                    Face Index                               │
│                  (ai/photos/index.go)                       │
│  - 人脸嵌入向量索引                                            │
│  - 相似度搜索                                                  │
│  - 持久化存储                                                  │
└─────────────────────────────────────────────────────────────┘
```

### 3.2 核心模块

#### 3.2.1 人脸检测器 (`detector.go`)

```go
// FaceDetector 人脸检测器
type FaceDetector interface {
    // DetectFaces 检测图像中的人脸
    DetectFaces(img image.Image) ([]FaceDetection, error)
    
    // DetectFacesBatch 批量检测
    DetectFacesBatch(images []image.Image) ([][]FaceDetection, error)
}

// FaceDetection 人脸检测结果
type FaceDetection struct {
    BoundingBox Rectangle   // 人脸边界框
    Landmarks   []Point     // 关键点 (眼睛、鼻子、嘴巴)
    Confidence  float64     // 检测置信度
    Quality     float64     // 人脸质量评分
}

// 实现:
// - GoFaceDetector: 基于 go-face (dlib) 实现
// - ONNXDetector: 基于 ONNX Runtime 实现 (可选)
```

#### 3.2.2 人脸嵌入器 (`embedder.go`)

```go
// FaceEmbedder 人脸嵌入向量提取器
type FaceEmbedder interface {
    // GetEmbedding 提取人脸嵌入向量
    GetEmbedding(img image.Image, detection FaceDetection) ([]float32, error)
}

// ArcFaceEmbedder ArcFace 模型实现
type ArcFaceEmbedder struct {
    modelPath  string
    inputSize  int        // 112x112
    embedding  int        // 512 维
}

// 流程:
// 1. 对齐人脸 (使用关键点)
// 2. 缩放到 112x112
// 3. 归一化
// 4. 前向推理
// 5. L2 归一化输出
```

#### 3.2.3 人脸聚类器 (`clusterer.go`)

```go
// FaceClusterer 人脸聚类器
type FaceClusterer interface {
    // Cluster 将人脸嵌入向量聚类为人物
    Cluster(embeddings [][]float32) ([]Cluster, error)
}

// DBSCANClusterer 基于 DBSCAN 的聚类实现
type DBSCANClusterer struct {
    epsilon     float64   // 相似度阈值 (0.6)
    minSamples  int       // 最小样本数 (2)
}

// Cluster 聚类结果
type Cluster struct {
    PersonID     string
    FaceIDs      []string
    Representative string    // 代表性人脸ID
}
```

#### 3.2.4 人脸索引 (`index.go`)

```go
// FaceIndex 人脸索引，支持快速相似度搜索
type FaceIndex struct {
    embeddings map[string][]float32
    persons    map[string]string    // face_id -> person_id
    metadata   map[string]*FaceMeta
}

// Search 搜索相似人脸
func (idx *FaceIndex) Search(embedding []float32, threshold float64, topK int) []SearchResult

// Add 添加人脸
func (idx *FaceIndex) Add(faceID string, embedding []float32, personID string)

// Persistence 持久化
func (idx *FaceIndex) Save(path string) error
func (idx *FaceIndex) Load(path string) error
```

### 3.3 工作流程

#### 人脸识别流程
```
1. 用户上传照片
2. 触发 AI 分析任务
3. 人脸检测: 检测照片中的人脸
4. 人脸对齐: 使用关键点对齐人脸
5. 特征提取: 提取 512 维嵌入向量
6. 相似度匹配: 在人脸索引中搜索相似人脸
7. 结果更新: 更新照片的人脸信息和人物相册
```

#### 新人脸聚类流程
```
1. 收集未分配的人脸嵌入向量
2. 执行 DBSCAN 聚类
3. 为每个聚类创建新人物
4. 更新人脸索引
5. 生成人物相册
```

### 3.4 集成 go-face

```go
// GoFaceDetector 使用 go-face 库实现人脸检测
type GoFaceDetector struct {
    recognizer *face.Recognizer
    samples    []face.Descriptor
    cats       []int32
}

func NewGoFaceDetector(modelDir string) (*GoFaceDetector, error) {
    // 加载预训练模型
    rec, err := face.NewRecognizer(modelDir)
    if err != nil {
        return nil, err
    }
    return &GoFaceDetector{recognizer: rec}, nil
}

func (d *GoFaceDetector) DetectFaces(img image.Image) ([]FaceDetection, error) {
    // 转换为 go-face 格式
    faces, err := d.recognizer.Recognize(img)
    if err != nil {
        return nil, err
    }
    
    detections := make([]FaceDetection, len(faces))
    for i, f := range faces {
        detections[i] = FaceDetection{
            BoundingBox: Rectangle{
                X: f.Rectangle.Min.X,
                Y: f.Rectangle.Min.Y,
                Width: f.Rectangle.Max.X - f.Rectangle.Min.X,
                Height: f.Rectangle.Max.Y - f.Rectangle.Min.Y,
            },
            Landmarks: []Point{
                {f.Landmarks[0].X, f.Landmarks[0].Y}, // 左眼
                {f.Landmarks[1].X, f.Landmarks[1].Y}, // 右眼
                // ... 其他关键点
            },
            Confidence: 1.0,
            Quality:    d.calculateQuality(f),
        }
    }
    return detections, nil
}
```

### 3.5 性能优化

| 优化项 | 描述 |
|--------|------|
| 批量处理 | 多张照片并行检测 |
| 缩略图检测 | 先在小图检测，再用原图精确定位 |
| 模型量化 | INT8 量化减少内存占用 |
| GPU 加速 | 可选 CUDA 支持 |
| 缓存 | 缓存已分析的嵌入向量 |

---

## 四、API 设计

### 4.1 网盘挂载 API

```
POST   /api/v1/cloudfuse/mounts          # 创建挂载
GET    /api/v1/cloudfuse/mounts          # 列出挂载
GET    /api/v1/cloudfuse/mounts/:id      # 获取挂载详情
DELETE /api/v1/cloudfuse/mounts/:id      # 卸载挂载
POST   /api/v1/cloudfuse/mounts/:id/test # 测试连接
GET    /api/v1/cloudfuse/providers       # 获取支持的网盘列表
GET    /api/v1/cloudfuse/mounts/:id/stats # 获取统计信息
```

### 4.2 人脸识别 API

```
POST   /api/v1/photos/:id/analyze        # 分析单张照片
POST   /api/v1/photos/batch-analyze      # 批量分析
GET    /api/v1/photos/:id/faces          # 获取照片人脸信息
GET    /api/v1/persons                   # 列出人物
PUT    /api/v1/persons/:id               # 更新人物信息
POST   /api/v1/persons/:id/merge         # 合并人物
DELETE /api/v1/persons/:id               # 删除人物
GET    /api/v1/persons/:id/photos        # 获取人物照片
POST   /api/v1/persons/:id/train         # 重新训练人物识别
```

---

## 五、依赖

### 5.1 网盘挂载
- `bazil.org/fuse` - FUSE 库 (已有)
- `github.com/aws/aws-sdk-go-v2` - S3 客户端 (已有)

### 5.2 人脸识别
- `github.com/Kagami/go-face` - dlib 封装，人脸检测和识别
- 预训练模型:
  - `shape_predictor_5_face_landmarks.dat` - 关键点检测
  - `dlib_face_recognition_resnet_model_v1.dat` - 人脸识别
  - `mmod_human_face_detector.dat` - CNN 人脸检测器 (可选)

---

## 六、实施计划

### Phase 1: 网盘挂载核心 (P0)
1. ✅ FUSE 文件系统框架
2. 🔲 实现 Read/Write 操作
3. 🔲 目录缓存优化
4. 🔲 文件缓存实现
5. 🔲 单元测试

### Phase 2: 人脸识别核心 (P1)
1. ✅ 人脸识别框架
2. 🔲 集成 go-face 库
3. 🔲 人脸索引和搜索
4. 🔲 人物聚类
5. 🔲 智能相册生成

---

*兵部 软件工程*
*2026-03-25*