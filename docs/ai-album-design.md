# AI相册"以文搜图"功能设计

## 1. 概述

### 1.1 背景
参考飞牛fnOS的AI相册功能，设计"以文搜图"(Text-to-Image Search)能力。用户可以通过自然语言描述搜索照片，如：
- "海边的日落"
- "穿红衣服的人"
- "去年春节的照片"

### 1.2 核心价值
- **自然语言搜索**：无需记住文件名或时间，用描述即可找到照片
- **语义理解**：支持同义词、场景描述、情感表达
- **跨模态检索**：文本与图像在同一向量空间匹配

### 1.3 技术选型
- **CLIP模型**：OpenAI多模态预训练模型，支持图文跨模态检索
- **向量数据库**：Qdrant/Milvus 用于高效向量检索
- **推理框架**：ONNX Runtime / TensorRT 用于模型推理

---

## 2. 架构设计

### 2.1 整体架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        API Layer                                 │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────────┐ │
│  │ POST /search│  │POST /index  │  │ GET /suggestions        │ │
│  │  text query │  │  photo path │  │  partial text           │ │
│  └──────┬──────┘  └──────┬──────┘  └────────────┬────────────┘ │
└─────────┼────────────────┼──────────────────────┼───────────────┘
          │                │                      │
┌─────────▼────────────────▼──────────────────────▼───────────────┐
│                     Service Layer                                │
│  ┌───────────────────────────────────────────────────────────┐  │
│  │                   PhotoSearchService                       │  │
│  │  - TextToImageSearch()    - IndexPhoto()                  │  │
│  │  - BatchIndex()           - GetSuggestions()              │  │
│  └───────────────────────────────────────────────────────────┘  │
│                              │                                   │
│  ┌───────────────┬───────────┴───────────┬───────────────────┐  │
│  │               │                       │                   │  │
│  ▼               ▼                       ▼                   ▼  │
│ ┌─────────┐ ┌──────────┐ ┌──────────────┐ ┌────────────────┐  │
│ │Embedding│ │  Vector  │ │   Metadata   │ │   Similarity   │  │
│ │ Service │ │  Store   │ │    Store     │ │    Scorer      │  │
│ └────┬────┘ └────┬─────┘ └──────┬───────┘ └────────┬───────┘  │
└──────┼───────────┼──────────────┼─────────────────┼───────────┘
       │           │              │                 │
┌──────▼───────────▼──────────────▼─────────────────▼───────────┐
│                     Infrastructure Layer                        │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────────────┐  │
│  │  CLIP    │ │  Qdrant  │ │ Postgres │ │    Redis         │  │
│  │  Model   │ │  Vector  │ │  Metadata│ │    Cache         │  │
│  │(ONNX/TF) │ │   DB     │ │    DB    │ │                  │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────────────┘  │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 模块职责

| 模块 | 职责 |
|------|------|
| PhotoSearchService | 核心服务，协调各组件完成搜索和索引 |
| EmbeddingService | 调用CLIP模型生成文本/图像向量 |
| VectorStore | 向量存储与相似度检索 |
| MetadataStore | 照片元数据存储 (EXIF、标签等) |
| SimilarityScorer | 多维度相似度融合计算 |

### 2.3 数据流

**索引流程**：
```
新照片 → 图片预处理 → CLIP图像编码 → 向量存储
    ↓
EXIF提取 → 元数据解析 → 元数据存储
    ↓
人脸检测 → 人脸聚类 → 人物标签
    ↓
场景分类 → 场景标签 → 标签存储
```

**搜索流程**：
```
用户文本 → 文本预处理 → CLIP文本编码 → 向量检索
    ↓
元数据过滤 ← 时间/地点/人物过滤条件
    ↓
结果融合 ← 向量相似度 + 元数据匹配
    ↓
排序返回 → 返回Top-K结果
```

---

## 3. 核心接口设计

### 3.1 数据结构

```go
// internal/ai/photos/text_search.go

// TextSearchQuery 文本搜索请求
type TextSearchQuery struct {
    // 核心参数
    Text      string `json:"text"`                // 搜索文本
    Limit     int    `json:"limit,omitempty"`     // 返回数量，默认20
    Offset    int    `json:"offset,omitempty"`    // 分页偏移

    // 过滤条件
    DateFrom  *time.Time `json:"date_from,omitempty"`
    DateTo    *time.Time `json:"date_to,omitempty"`
    Location  *Location  `json:"location,omitempty"`
    PersonIDs []string   `json:"person_ids,omitempty"`

    // 搜索模式
    Mode      SearchMode `json:"mode,omitempty"` // semantic, hybrid, exact
    MinScore  float64    `json:"min_score,omitempty"` // 最低相似度阈值
}

// SearchMode 搜索模式
type SearchMode string

const (
    SearchModeSemantic SearchMode = "semantic" // 纯语义搜索
    SearchModeHybrid   SearchMode = "hybrid"   // 混合搜索(语义+关键词)
    SearchModeExact    SearchMode = "exact"    // 精确匹配
)

// TextSearchResult 文本搜索结果
type TextSearchResult struct {
    Photos     []PhotoMatch `json:"photos"`
    Total      int          `json:"total"`
    QueryTime  int64        `json:"query_time_ms"`
    Expanded   []string     `json:"expanded_terms,omitempty"` // 扩展词
    VectorTime int64        `json:"vector_time_ms"`
    MatchTime  int64        `json:"match_time_ms"`
}

// PhotoMatch 匹配的照片
type PhotoMatch struct {
    Photo
    Score       float64          `json:"score"`        // 综合相似度
    VectorScore float64          `json:"vector_score"` // 向量相似度
    MatchReason MatchReason      `json:"match_reason"` // 匹配原因
    Highlights  []HighlightSpan  `json:"highlights,omitempty"`
}

// MatchReason 匹配原因
type MatchReason struct {
    Type       string   `json:"type"` // semantic, tag, scene, person, location
    MatchedOn  []string `json:"matched_on"`
    Confidence float64  `json:"confidence"`
}

// PhotoEmbedding 照片向量
type PhotoEmbedding struct {
    PhotoID    string    `json:"photo_id"`
    Vector     []float32 `json:"vector"`     // 512维向量
    Model      string    `json:"model"`      // clip-vit-base-patch32
    CreatedAt  time.Time `json:"created_at"`
    Dimensions int       `json:"dimensions"` // 512
}

// TextEmbedding 文本向量
type TextEmbedding struct {
    Text       string    `json:"text"`
    Vector     []float32 `json:"vector"`
    Model      string    `json:"model"`
    CreatedAt  time.Time `json:"created_at"`
}
```

### 3.2 服务接口

```go
// TextSearchService 文本搜索服务接口
type TextSearchService interface {
    // SearchByText 以文搜图
    SearchByText(ctx context.Context, query *TextSearchQuery) (*TextSearchResult, error)

    // IndexPhoto 索引单张照片
    IndexPhoto(ctx context.Context, photoPath string) error

    // IndexBatch 批量索引
    IndexBatch(ctx context.Context, photoPaths []string) (*IndexResult, error)

    // DeleteIndex 删除索引
    DeleteIndex(ctx context.Context, photoID string) error

    // RebuildIndex 重建索引
    RebuildIndex(ctx context.Context) error

    // GetSuggestions 获取搜索建议
    GetSuggestions(ctx context.Context, prefix string) ([]SearchSuggestion, error)
}

// EmbeddingService 向量化服务接口
type EmbeddingService interface {
    // EncodeImage 图像向量化
    EncodeImage(ctx context.Context, img image.Image) (*PhotoEmbedding, error)

    // EncodeImageBatch 批量图像向量化
    EncodeImageBatch(ctx context.Context, images []image.Image) ([]*PhotoEmbedding, error)

    // EncodeText 文本向量化
    EncodeText(ctx context.Context, text string) (*TextEmbedding, error)

    // EncodeTextBatch 批量文本向量化
    EncodeTextBatch(ctx context.Context, texts []string) ([]*TextEmbedding, error)

    // GetModelInfo 获取模型信息
    GetModelInfo() *ModelInfo
}

// VectorStore 向量存储接口
type VectorStore interface {
    // Insert 插入向量
    Insert(ctx context.Context, embedding *PhotoEmbedding) error

    // InsertBatch 批量插入
    InsertBatch(ctx context.Context, embeddings []*PhotoEmbedding) error

    // Search 相似度搜索
    Search(ctx context.Context, vector []float32, limit int, filter *VectorFilter) ([]VectorMatch, error)

    // Delete 删除向量
    Delete(ctx context.Context, photoID string) error

    // Count 统计数量
    Count(ctx context.Context) (int64, error)
}

// VectorFilter 向量搜索过滤条件
type VectorFilter struct {
    DateFrom  *time.Time `json:"date_from,omitempty"`
    DateTo    *time.Time `json:"date_to,omitempty"`
    Locations []string   `json:"locations,omitempty"`
    Persons   []string   `json:"persons,omitempty"`
    Scenes    []string   `json:"scenes,omitempty"`
}

// VectorMatch 向量匹配结果
type VectorMatch struct {
    PhotoID string  `json:"photo_id"`
    Score   float64 `json:"score"`
}
```

### 3.3 API设计

**搜索接口**：
```yaml
POST /api/v1/photos/search
Content-Type: application/json

Request:
{
  "text": "海边日落的照片",
  "limit": 20,
  "offset": 0,
  "mode": "hybrid",
  "filters": {
    "date_from": "2024-01-01",
    "date_to": "2024-12-31",
    "location": {
      "city": "三亚"
    }
  }
}

Response:
{
  "photos": [
    {
      "id": "photo_001",
      "path": "/photos/2024/08/beach_sunset.jpg",
      "filename": "beach_sunset.jpg",
      "score": 0.92,
      "vector_score": 0.89,
      "match_reason": {
        "type": "semantic",
        "matched_on": ["beach", "sunset", "海边", "日落"],
        "confidence": 0.92
      },
      "taken_at": "2024-08-15T18:30:00Z",
      "location": {
        "city": "三亚",
        "poi": "亚龙湾"
      },
      "scene": {
        "primary_category": "beach",
        "categories": [
          {"category": "beach", "score": 0.95},
          {"category": "sunset", "score": 0.88}
        ]
      }
    }
  ],
  "total": 15,
  "query_time_ms": 45,
  "expanded_terms": ["海边", "日落", "beach", "sunset", "黄昏"]
}
```

**索引接口**：
```yaml
POST /api/v1/photos/index
Content-Type: application/json

Request:
{
  "paths": ["/photos/2024/"],
  "recursive": true,
  "force": false
}

Response:
{
  "job_id": "idx_20240325_001",
  "status": "running",
  "total_files": 5000,
  "indexed": 1200,
  "failed": 3
}
```

**搜索建议接口**：
```yaml
GET /api/v1/photos/suggestions?q=海

Response:
{
  "suggestions": [
    {"text": "海边", "type": "scene", "count": 150},
    {"text": "海鲜", "type": "food", "count": 45},
    {"text": "海岸线", "type": "scene", "count": 32}
  ]
}
```

---

## 4. CLIP模型集成方案

### 4.1 模型选型

| 模型 | 维度 | 速度 | 精度 | 推荐场景 |
|------|------|------|------|----------|
| CLIP ViT-B/32 | 512 | 快 | 中 | 资源受限环境 |
| CLIP ViT-B/16 | 512 | 中 | 高 | 平衡选择 |
| CLIP ViT-L/14 | 768 | 慢 | 最高 | 高精度需求 |
| Chinese-CLIP | 512 | 中 | 高(中文) | 中文场景 |

**推荐方案**：Chinese-CLIP ViT-B/16
- 支持中英文
- 精度与速度平衡
- 512维向量，存储开销适中

### 4.2 模型部署架构

```
┌─────────────────────────────────────────────────────────┐
│                    Inference Service                     │
│  ┌─────────────────────────────────────────────────────┐│
│  │              Model Manager                           ││
│  │  - 模型加载/卸载                                     ││
│  │  - GPU内存管理                                       ││
│  │  - 模型热切换                                        ││
│  └─────────────────────────────────────────────────────┘│
│                                                          │
│  ┌──────────────────┐    ┌──────────────────────────┐   │
│  │  Image Encoder   │    │     Text Encoder         │   │
│  │  ┌────────────┐  │    │  ┌────────────────────┐  │   │
│  │  │ Vision     │  │    │  │ Tokenizer          │  │   │
│  │  │ Transformer│  │    │  │ (BPE/SentencePiece)│  │   │
│  │  └─────┬──────┘  │    │  └─────────┬──────────┘  │   │
│  │        ▼         │    │            ▼              │   │
│  │  ┌────────────┐  │    │  ┌────────────────────┐  │   │
│  │  │ Projection │  │    │  │ Text Transformer   │  │   │
│  │  │ Layer      │  │    │  └─────────┬──────────┘  │   │
│  │  └─────┬──────┘  │    │            ▼              │   │
│  │        ▼         │    │  ┌────────────────────┐  │   │
│  │  512-d Vector    │    │  │ Projection Layer   │  │   │
│  │                  │    │  └─────────┬──────────┘  │   │
│  └──────────────────┘    │            ▼              │   │
│                          │  512-d Vector            │   │
│                          └──────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────────────────────────────┐│
│  │              ONNX Runtime / TensorRT                ││
│  │  - GPU加速 (CUDA/TensorRT)                          ││
│  │  - CPU优化 (OpenVINO/ONNX)                          ││
│  │  - 批处理支持                                       ││
│  └─────────────────────────────────────────────────────┘│
└─────────────────────────────────────────────────────────┘
```

### 4.3 部署配置

```yaml
# configs/clip-service.yaml

model:
  name: "chinese-clip-vit-b-16"
  version: "1.0"
  path: "/models/chinese-clip-vit-b-16.onnx"
  dimensions: 512
  
inference:
  device: "cuda"  # cuda, cpu, tensorrt
  batch_size: 32
  max_batch_wait_ms: 10
  
  # GPU配置
  gpu:
    device_id: 0
    memory_limit_mb: 4096
    
  # CPU配置
  cpu:
    num_threads: 8
    optimization_level: 3
    
performance:
  # 预热
  warmup:
    enabled: true
    batch_size: 8
    iterations: 10
    
  # 批处理
  batching:
    enabled: true
    max_batch_size: 64
    timeout_ms: 50
    
  # 缓存
  cache:
    enabled: true
    max_entries: 10000
    ttl_seconds: 3600
```

### 4.4 推理性能优化

**批处理优化**：
```go
// internal/ai/photos/embedding_batch.go

type BatchEmbeddingProcessor struct {
    queue       chan *EmbeddingRequest
    batchSize   int
    timeout     time.Duration
    encoder     EmbeddingService
}

type EmbeddingRequest struct {
    Type     string      // "image" or "text"
    Input    interface{} // image.Image or string
    Result   chan *EmbeddingResult
}

type EmbeddingResult struct {
    Embedding *PhotoEmbedding
    Error     error
}

func (p *BatchEmbeddingProcessor) Start() {
    go func() {
        batch := make([]*EmbeddingRequest, 0, p.batchSize)
        timer := time.NewTimer(p.timeout)
        
        for {
            select {
            case req := <-p.queue:
                batch = append(batch, req)
                if len(batch) >= p.batchSize {
                    p.processBatch(batch)
                    batch = make([]*EmbeddingRequest, 0, p.batchSize)
                    timer.Reset(p.timeout)
                }
                
            case <-timer.C:
                if len(batch) > 0 {
                    p.processBatch(batch)
                    batch = make([]*EmbeddingRequest, 0, p.batchSize)
                }
                timer.Reset(p.timeout)
            }
        }
    }()
}
```

---

## 5. 向量存储设计

### 5.1 存储选型

| 方案 | 优点 | 缺点 | 推荐场景 |
|------|------|------|----------|
| Qdrant | 高性能、Rust实现、过滤能力强 | 相对新 | 生产环境首选 |
| Milvus | 成熟、分布式 | 部署复杂 | 大规模场景 |
| pgvector | 集成简单 | 性能一般 | 小规模/原型 |
| FAISS | 高性能 | 仅库，需自己实现服务 | 自建服务 |

**推荐方案**：Qdrant
- 单二进制部署，运维简单
- 支持复杂过滤条件
- 原生支持批量操作

### 5.2 Collection设计

```json
{
  "collection_name": "photos",
  "vectors": {
    "size": 512,
    "distance": "Cosine"
  },
  "payload_schema": {
    "photo_id": "keyword",
    "path": "keyword",
    "filename": "text",
    "taken_at": "datetime",
    "created_at": "datetime",
    "modified_at": "datetime",
    "width": "integer",
    "height": "integer",
    "size": "integer",
    "format": "keyword",
    
    "location_country": "keyword",
    "location_province": "keyword", 
    "location_city": "keyword",
    "location_poi": "text",
    
    "person_ids": "keyword[]",
    "scene_categories": "keyword[]",
    "tags": "keyword[]",
    "objects": "keyword[]",
    
    "camera_make": "keyword",
    "camera_model": "keyword",
    
    "quality_score": "float",
    "aesthetic_score": "float"
  }
}
```

### 5.3 索引策略

```go
// internal/ai/photos/vector_index.go

// IndexPhoto 索引照片向量
func (s *VectorStore) IndexPhoto(ctx context.Context, photo *Photo, embedding *PhotoEmbedding) error {
    point := &qdrant.PointStruct{
        ID:      qdrant.NewIDString(photo.ID),
        Vector:  embedding.Vector,
        Payload: s.buildPayload(photo),
    }
    
    _, err := s.client.Upsert(ctx, &qdrant.UpsertPoints{
        CollectionName: s.collection,
        Points:         []*qdrant.PointStruct{point},
    })
    return err
}

func (s *VectorStore) buildPayload(photo *Photo) map[string]interface{} {
    payload := map[string]interface{}{
        "photo_id":     photo.ID,
        "path":         photo.Path,
        "filename":     photo.Filename,
        "taken_at":     photo.TakenAt,
        "created_at":   photo.CreatedAt,
        "modified_at":  photo.ModifiedAt,
        "width":        photo.Width,
        "height":       photo.Height,
        "size":         photo.Size,
        "format":       photo.Format,
        "quality_score": photo.Scene.Quality.Score,
    }
    
    // 位置信息
    if photo.GPS != nil && photo.GPS.Location != nil {
        payload["location_country"] = photo.GPS.Location.Country
        payload["location_province"] = photo.GPS.Location.Province
        payload["location_city"] = photo.GPS.Location.City
        payload["location_poi"] = photo.GPS.Location.POI
    }
    
    // 人物标签
    if len(photo.Faces) > 0 {
        personIDs := make([]string, len(photo.Faces))
        for i, face := range photo.Faces {
            personIDs[i] = face.PersonID
        }
        payload["person_ids"] = personIDs
    }
    
    // 场景分类
    if photo.Scene != nil {
        categories := make([]string, len(photo.Scene.Categories))
        for i, cat := range photo.Scene.Categories {
            categories[i] = string(cat.Category)
        }
        payload["scene_categories"] = categories
        payload["aesthetic_score"] = photo.Scene.AestheticScore
    }
    
    return payload
}
```

### 5.4 搜索优化

```go
// SearchWithFilter 带过滤条件的向量搜索
func (s *VectorStore) SearchWithFilter(ctx context.Context, query *TextSearchQuery, vector []float32) ([]VectorMatch, error) {
    // 构建过滤条件
    filter := s.buildFilter(query)
    
    // 执行搜索
    result, err := s.client.Search(ctx, &qdrant.SearchPoints{
        CollectionName: s.collection,
        Vector:         vector,
        Limit:          uint64(query.Limit * 2), // 多取一些用于后处理
        Filter:         filter,
        WithPayload:    qdrant.NewWithPayload(true),
    })
    
    if err != nil {
        return nil, err
    }
    
    // 转换结果
    matches := make([]VectorMatch, 0, len(result))
    for _, point := range result {
        matches = append(matches, VectorMatch{
            PhotoID: point.Id.GetStringValue(),
            Score:   float64(point.Score),
        })
    }
    
    return matches, nil
}

func (s *VectorStore) buildFilter(query *TextSearchQuery) *qdrant.Filter {
    var conditions []*qdrant.Condition
    
    // 时间过滤
    if query.DateFrom != nil || query.DateTo != nil {
        dateCond := &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Field{
                Field: &qdrant.FieldCondition{
                    Key: "taken_at",
                    Match: &qdrant.FieldCondition_Range{
                        Range: &qdrant.Range{
                            Gte: query.DateFrom,
                            Lte: query.DateTo,
                        },
                    },
                },
            },
        }
        conditions = append(conditions, dateCond)
    }
    
    // 人物过滤
    if len(query.PersonIDs) > 0 {
        personCond := &qdrant.Condition{
            ConditionOneOf: &qdrant.Condition_Field{
                Field: &qdrant.FieldCondition{
                    Key: "person_ids",
                    Match: &qdrant.FieldCondition_MatchAny{
                        MatchAny: &qdrant.MatchAny{
                            Any: query.PersonIDs,
                        },
                    },
                },
            },
        }
        conditions = append(conditions, personCond)
    }
    
    if len(conditions) == 0 {
        return nil
    }
    
    return &qdrant.Filter{
        Must: conditions,
    }
}
```

---

## 6. 搜索策略

### 6.1 混合搜索架构

```
用户查询: "去年春节海边全家福"
         │
         ▼
┌────────────────────────────────────────┐
│           Query Analyzer               │
│  - 分词: [去年, 春节, 海边, 全家福]     │
│  - 实体识别: 时间=去年春节, 地点=海边   │
│  - 意图分析: 全家福 → 多人合影          │
└────────────────┬───────────────────────┘
                 │
         ┌───────┴───────┐
         ▼               ▼
┌─────────────┐  ┌─────────────┐
│ Text Encoder│  │ Rule Parser │
│ (CLIP)      │  │ (结构化条件)│
└──────┬──────┘  └──────┬──────┘
       │                │
       ▼                ▼
┌─────────────┐  ┌─────────────┐
│ Vector      │  │ Metadata    │
│ Search      │  │ Filter      │
└──────┬──────┘  └──────┬──────┘
       │                │
       └───────┬────────┘
               ▼
┌────────────────────────────────────────┐
│         Result Fusion                  │
│  - 向量相似度 (60%)                     │
│  - 元数据匹配 (30%)                     │
│  - 时间衰减 (10%)                       │
└────────────────┬───────────────────────┘
                 │
                 ▼
┌────────────────────────────────────────┐
│         Re-ranking & Response          │
│  - 按综合得分排序                        │
│  - 生成匹配解释                          │
└────────────────────────────────────────┘
```

### 6.2 查询理解

```go
// internal/ai/photos/query_analyzer.go

type QueryAnalyzer struct {
    ner       *NERModel      // 命名实体识别
    segmenter *Segmenter     // 中文分词
    intent    *IntentModel   // 意图识别
}

type ParsedQuery struct {
    OriginalText  string
    Tokens        []string
    Entities      []Entity
    Intent        string
    
    // 提取的结构化条件
    TimeRange     *TimeRange
    Location      *Location
    PersonCount   *int        // 全家福 → 多人
    SceneType     []string
    Keywords      []string
}

type Entity struct {
    Text  string
    Type  string  // time, location, person, scene, object
    Value interface{}
}

func (a *QueryAnalyzer) Analyze(query string) (*ParsedQuery, error) {
    result := &ParsedQuery{OriginalText: query}
    
    // 1. 分词
    result.Tokens = a.segmenter.Cut(query)
    
    // 2. 命名实体识别
    entities := a.ner.Extract(query)
    
    for _, entity := range entities {
        switch entity.Type {
        case "time":
            result.TimeRange = a.parseTimeEntity(entity.Text)
        case "location":
            result.Location = a.parseLocationEntity(entity.Text)
        case "scene":
            result.SceneType = append(result.SceneType, entity.Text)
        }
        result.Entities = append(result.Entities, entity)
    }
    
    // 3. 意图识别
    result.Intent = a.intent.Classify(query)
    
    // 4. 关键词提取 (去除实体后的词)
    result.Keywords = a.extractKeywords(result)
    
    return result, nil
}

// parseTimeEntity 解析时间表达式
func (a *QueryAnalyzer) parseTimeEntity(text string) *TimeRange {
    now := time.Now()
    
    patterns := map[string]func() *TimeRange{
        "去年": func() *TimeRange {
            year := now.Year() - 1
            return &TimeRange{
                Start: time.Date(year, 1, 1, 0, 0, 0, 0, time.Local),
                End:   time.Date(year, 12, 31, 23, 59, 59, 0, time.Local),
            }
        },
        "春节": func() *TimeRange {
            // 根据农历计算春节日期
            springFestival := getSpringFestival(now.Year())
            return &TimeRange{
                Start: springFestival.AddDate(0, 0, -7),
                End:   springFestival.AddDate(0, 0, 7),
            }
        },
        "上个月": func() *TimeRange {
            firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.Local)
            start := firstOfMonth.AddDate(0, -1, 0)
            end := firstOfMonth.AddDate(0, 0, -1)
            return &TimeRange{Start: start, End: end}
        },
    }
    
    for pattern, fn := range patterns {
        if strings.Contains(text, pattern) {
            return fn()
        }
    }
    
    return nil
}
```

### 6.3 结果融合

```go
// internal/ai/photos/result_fusion.go

type ResultFusion struct {
    weights FusionWeights
}

type FusionWeights struct {
    Vector    float64 // 向量相似度权重
    Metadata  float64 // 元数据匹配权重
    TimeDecay float64 // 时间衰减权重
    Quality   float64 // 质量分数权重
}

func DefaultFusionWeights() FusionWeights {
    return FusionWeights{
        Vector:    0.6,
        Metadata:  0.2,
        TimeDecay: 0.1,
        Quality:   0.1,
    }
}

func (f *ResultFusion) Fuse(vectorResults []VectorMatch, photos []*Photo, query *ParsedQuery) []PhotoMatch {
    matches := make([]PhotoMatch, 0, len(vectorResults))
    
    // 构建photo id到向量分数的映射
    vectorScores := make(map[string]float64)
    for _, vm := range vectorResults {
        vectorScores[vm.PhotoID] = vm.Score
    }
    
    for _, photo := range photos {
        match := PhotoMatch{
            Photo:      *photo,
            VectorScore: vectorScores[photo.ID],
        }
        
        // 计算各维度分数
        vectorScore := match.VectorScore
        metadataScore := f.calcMetadataScore(photo, query)
        timeScore := f.calcTimeDecayScore(photo, query)
        qualityScore := f.calcQualityScore(photo)
        
        // 加权融合
        match.Score = f.weights.Vector*vectorScore +
            f.weights.Metadata*metadataScore +
            f.weights.TimeDecay*timeScore +
            f.weights.Quality*qualityScore
        
        // 生成匹配原因
        match.MatchReason = f.generateMatchReason(photo, query, vectorScore, metadataScore)
        
        matches = append(matches, match)
    }
    
    // 按综合分数排序
    sort.Slice(matches, func(i, j int) bool {
        return matches[i].Score > matches[j].Score
    })
    
    return matches
}

func (f *ResultFusion) calcMetadataScore(photo *Photo, query *ParsedQuery) float64 {
    var score float64
    var factors int
    
    // 场景匹配
    if len(query.SceneType) > 0 && photo.Scene != nil {
        for _, scene := range query.SceneType {
            for _, cat := range photo.Scene.Categories {
                if strings.Contains(string(cat.Category), scene) ||
                    strings.Contains(scene, string(cat.Category)) {
                    score += cat.Score
                    factors++
                }
            }
        }
    }
    
    // 地点匹配
    if query.Location != nil && photo.GPS != nil && photo.GPS.Location != nil {
        if query.Location.City != "" && photo.GPS.Location.City == query.Location.City {
            score += 1.0
            factors++
        }
    }
    
    if factors > 0 {
        return score / float64(factors)
    }
    return 0.5 // 默认中等分数
}

func (f *ResultFusion) calcTimeDecayScore(photo *Photo, query *ParsedQuery) float64 {
    if query.TimeRange == nil || photo.TakenAt == nil {
        return 0.5
    }
    
    // 在时间范围内的照片，越接近中间分数越高
    duration := query.TimeRange.End.Sub(query.TimeRange.Start)
    if duration == 0 {
        return 1.0
    }
    
    midTime := query.TimeRange.Start.Add(duration / 2)
    offset := photo.TakenAt.Sub(midTime)
    
    if offset < 0 {
        offset = -offset
    }
    
    // 使用正态分布计算衰减
    sigma := duration.Seconds() / 4 // 标准差为时间范围的1/4
    decay := math.Exp(-(offset.Seconds() * offset.Seconds()) / (2 * sigma * sigma))
    
    return decay
}

func (f *ResultFusion) calcQualityScore(photo *Photo) float64 {
    if photo.Scene == nil {
        return 0.5
    }
    return photo.Scene.Quality.Score
}
```

---

## 7. 性能与资源评估

### 7.1 模型资源需求

| 模型 | GPU显存 | CPU内存 | 推理时间(GPU) | 推理时间(CPU) |
|------|---------|---------|---------------|---------------|
| CLIP ViT-B/32 | 1.5GB | 2GB | 15ms | 150ms |
| CLIP ViT-B/16 | 2GB | 3GB | 25ms | 250ms |
| CLIP ViT-L/14 | 4GB | 6GB | 50ms | 500ms |
| Chinese-CLIP | 2GB | 3GB | 30ms | 300ms |

### 7.2 存储估算

**向量存储**：
- 单张照片向量: 512 × 4 bytes = 2KB
- 10万张照片: 200MB
- 100万张照片: 2GB

**元数据存储** (PostgreSQL):
- 单张照片元数据: ~500 bytes
- 10万张照片: 50MB
- 100万张照片: 500MB

**总存储需求**:
- 10万张照片: ~500MB (向量 + 元数据 + 索引)
- 100万张照片: ~5GB

### 7.3 性能指标

**索引性能**:
- 单张照片索引: ~100ms (图像预处理 + 编码 + 存储)
- 批量索引 (batch=32): ~50张/秒 (GPU)
- 批量索引 (batch=32): ~10张/秒 (CPU)

**搜索性能**:
- 文本编码: 30ms (GPU)
- 向量检索 (10万向量): <10ms
- 总响应时间: <100ms (P99)

### 7.4 部署建议

**最小配置**:
- CPU: 4核
- 内存: 8GB
- 存储: 50GB SSD
- 适用: 1万张照片，CPU推理

**推荐配置**:
- CPU: 8核
- 内存: 16GB
- GPU: RTX 3060 (12GB)
- 存储: 200GB NVMe SSD
- 适用: 10-50万张照片，GPU推理

**高性能配置**:
- CPU: 16核
- 内存: 32GB
- GPU: RTX 4090 (24GB)
- 存储: 1TB NVMe SSD
- 适用: 100万+张照片，高并发

---

## 8. 实现计划

### Phase 1: 基础框架 (2周)
- [ ] 向量化服务接口定义
- [ ] CLIP模型集成 (ONNX)
- [ ] Qdrant向量库集成
- [ ] 基础API实现

### Phase 2: 核心功能 (2周)
- [ ] 图像向量化流水线
- [ ] 文本向量化与搜索
- [ ] 元数据索引同步
- [ ] 批量索引任务

### Phase 3: 搜索优化 (2周)
- [ ] 查询理解与解析
- [ ] 混合搜索融合
- [ ] 过滤条件支持
- [ ] 搜索建议功能

### Phase 4: 生产化 (2周)
- [ ] 性能优化
- [ ] 错误处理
- [ ] 监控指标
- [ ] 文档完善

---

## 9. 监控与运维

### 9.1 关键指标

```yaml
# 监控指标
metrics:
  # 索引指标
  - index_total_photos
  - index_photos_per_second
  - index_queue_length
  - index_error_rate
  
  # 搜索指标
  - search_latency_p50
  - search_latency_p99
  - search_requests_per_second
  - search_result_count_avg
  
  # 模型指标
  - model_inference_time_image
  - model_inference_time_text
  - model_batch_size_avg
  - model_queue_length
  
  # 向量库指标
  - vector_db_total_vectors
  - vector_db_search_latency
  - vector_db_insert_latency
```

### 9.2 健康检查

```go
// internal/ai/photos/health.go

type HealthChecker struct {
    vectorStore VectorStore
    embedder    EmbeddingService
    metadata    Storage
}

func (h *HealthChecker) Check(ctx context.Context) (*HealthStatus, error) {
    status := &HealthStatus{
        Components: make(map[string]ComponentHealth),
    }
    
    // 检查向量库
    count, err := h.vectorStore.Count(ctx)
    if err != nil {
        status.Components["vector_store"] = ComponentHealth{
            Status: "unhealthy",
            Error:  err.Error(),
        }
    } else {
        status.Components["vector_store"] = ComponentHealth{
            Status: "healthy",
            Details: map[string]interface{}{
                "total_vectors": count,
            },
        }
    }
    
    // 检查模型服务
    modelInfo := h.embedder.GetModelInfo()
    testEmbedding, err := h.embedder.EncodeText(ctx, "test")
    if err != nil {
        status.Components["embedding_service"] = ComponentHealth{
            Status: "unhealthy",
            Error:  err.Error(),
        }
    } else {
        status.Components["embedding_service"] = ComponentHealth{
            Status: "healthy",
            Details: map[string]interface{}{
                "model":      modelInfo.Name,
                "dimensions": len(testEmbedding.Vector),
            },
        }
    }
    
    // 检查元数据存储
    // ...
    
    return status, nil
}
```

---

## 10. 风险与缓解

| 风险 | 影响 | 缓解措施 |
|------|------|----------|
| 模型推理慢 | 搜索延迟高 | GPU加速、批处理、模型量化 |
| 向量库故障 | 搜索不可用 | 主从复制、定期备份 |
| 索引延迟 | 新照片搜不到 | 增量索引、优先队列 |
| 存储不足 | 无法添加新照片 | 定期清理、存储扩容 |
| 中文理解不准 | 搜索结果差 | 使用Chinese-CLIP、查询改写 |

---

## 11. 附录

### A. 参考实现
- [OpenAI CLIP](https://github.com/openai/CLIP)
- [Chinese-CLIP](https://github.com/OFA-Sys/Chinese-CLIP)
- [Qdrant Documentation](https://qdrant.tech/documentation/)

### B. 相关文档
- [PHOTOS_API.md](./PHOTOS_API.md) - 照片API文档
- [PHOTOS_IMPLEMENTATION.md](./PHOTOS_IMPLEMENTATION.md) - 照片模块实现

### C. 变更记录
- 2026-03-25: 初始设计