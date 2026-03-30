// Package face - AI人脸聚类算法
// 优化DBSCAN聚类性能，支持大规模人脸数据的高效聚类
package face

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== 聚类配置 ====================

// ClusterConfig 聚类算法配置
type ClusterConfig struct {
	// DBSCAN参数
	Epsilon       float64 `json:"epsilon"`        // 相似度阈值（0-1），越大聚类越宽松
	MinPoints     int     `json:"min_points"`     // 最小点数，形成聚类需要的最小邻居数

	// 性能优化参数
	BatchSize     int     `json:"batch_size"`     // 批处理大小
	MaxWorkers    int     `json:"max_workers"`    // 最大并行worker数
	UseIndex      bool    `json:"use_index"`      // 使用索引加速
	IndexType     string  `json:"index_type"`     // 索引类型：hnsw, faiss, brute

	// 高级参数
	SimilarityMetric string `json:"similarity_metric"` // cosine, euclidean, dot
	PreFilter        bool   `json:"pre_filter"`        // 预过滤低质量人脸
	MinQuality       float64 `json:"min_quality"`       // 最小人脸质量阈值
	MaxClusterSize   int     `json:"max_cluster_size"`  // 最大聚类大小（防止噪声聚合）

	// 迭代优化
	Iterative      bool    `json:"iterative"`         // 是否迭代优化聚类
	MaxIterations  int     `json:"max_iterations"`    // 最大迭代次数
	Convergence    float64 `json:"convergence"`       // 收敛阈值
}

// DefaultClusterConfig 默认配置
func DefaultClusterConfig() *ClusterConfig {
	return &ClusterConfig{
		Epsilon:          0.6,           // 人脸相似度阈值
		MinPoints:        2,             // 最小邻居数
		BatchSize:        1000,          // 批处理大小
		MaxWorkers:       runtime.NumCPU(), // 默认使用CPU核心数
		UseIndex:         true,          // 启用索引加速
		IndexType:        "hnsw",        // HNSW索引（速度快）
		SimilarityMetric: "cosine",      // 余弦相似度
		PreFilter:        true,          // 预过滤低质量人脸
		MinQuality:       0.5,           // 最低质量要求
		MaxClusterSize:   500,           // 防止过大聚类
		Iterative:        true,          // 启用迭代优化
		MaxIterations:    10,            // 最大迭代次数
		Convergence:      0.01,          // 收敛阈值
	}
}

// ==================== 人脸数据结构 ====================

// FaceEmbedding 人脸embedding数据
type FaceEmbedding struct {
	ID         string    `json:"id"`
	PhotoID    string    `json:"photo_id"`
	Embedding  []float32 `json:"embedding"`  // 人脸特征向量（512维）
	Quality    float64   `json:"quality"`    // 检测质量/置信度
	ClusterID  int       `json:"cluster_id"` // 聚类ID，-1表示噪声/未分配
	PersonID   string    `json:"person_id"`  // 人物ID（用户指定）
	CreatedAt  time.Time `json:"created_at"`
}

// PersonCluster 人物聚类结果
type PersonCluster struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`           // 用户指定名称
	FaceCount        int             `json:"face_count"`     // 人脸数量
	Representative   *FaceEmbedding  `json:"representative"` // 代表人脸
	CenterEmbedding  []float32       `json:"center_embedding"` // 聚类中心embedding
	Faces            []*FaceEmbedding `json:"faces"`          // 所有属于此聚类的人脸
	CoverPhotoID     string          `json:"cover_photo_id"` // 封面照片ID
	AvgQuality       float64         `json:"avg_quality"`    // 平均质量
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// ClusterResult 聚类结果
type ClusterResult struct {
	Persons      []*PersonCluster `json:"persons"`       // 聚类的人物列表
	Unassigned   []*FaceEmbedding `json:"unassigned"`    // 未分配的人脸（噪声）
	TotalFaces   int              `json:"total_faces"`   // 总人脸数
	ClusterCount int              `json:"cluster_count"` // 聚类数量
	ProcessTime  time.Duration    `json:"process_time"`  // 处理时间
}

// ==================== 相似度计算 ====================

// SimilarityCalculator 相似度计算器
type SimilarityCalculator struct {
	metric string
}

// NewSimilarityCalculator 创建相似度计算器
func NewSimilarityCalculator(metric string) *SimilarityCalculator {
	return &SimilarityCalculator{metric: metric}
}

// Calculate 计算两个embedding的相似度
func (sc *SimilarityCalculator) Calculate(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	switch sc.metric {
	case "cosine":
		return sc.cosineSimilarity(a, b)
	case "euclidean":
		return sc.euclideanSimilarity(a, b)
	case "dot":
		return sc.dotProduct(a, b)
	default:
		return sc.cosineSimilarity(a, b)
	}
}

// cosineSimilarity 余弦相似度（推荐用于人脸embedding）
func (sc *SimilarityCalculator) cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

// euclideanSimilarity 欧式距离转相似度
func (sc *SimilarityCalculator) euclideanSimilarity(a, b []float32) float64 {
	var sum float64
	for i := range a {
		diff := float64(a[i]) - float64(b[i])
		sum += diff * diff
	}

	distance := math.Sqrt(sum)
	// 将距离转换为相似度（距离越小相似度越高）
	// 使用指数衰减：similarity = exp(-distance)
	return math.Exp(-distance)
}

// dotProduct 点积相似度
func (sc *SimilarityCalculator) dotProduct(a, b []float32) float64 {
	var sum float64
	for i := range a {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}

// ==================== 聚类索引 ====================

// ClusterIndex 聚类索引接口
// 用于加速大规模数据的相似度搜索
type ClusterIndex interface {
	// Add 添加embedding到索引
	Add(id string, embedding []float32) error
	// Search 搜索相似embedding
	Search(embedding []float32, k int, threshold float64) ([]SearchResult, error)
	// SearchByID 根据ID搜索相似项
	SearchByID(id string, k int, threshold float64) ([]SearchResult, error)
	// Get 获取指定embedding
	Get(id string) ([]float32, error)
	// Delete 删除指定embedding
	Delete(id string) error
	// Size 返回索引大小
	Size() int
	// Close 关闭索引
	Close() error
}

// SearchResult 搜索结果
type SearchResult struct {
	ID       string  `json:"id"`
	Score    float64 `json:"score"` // 相似度分数
	Distance float64 `json:"distance"` // 距离（可选）
}

// BruteForceIndex 暴力搜索索引（小数据量）
type BruteForceIndex struct {
	embeddings map[string][]float32
	calculator *SimilarityCalculator
	mu         sync.RWMutex
}

// NewBruteForceIndex 创建暴力索引
func NewBruteForceIndex(calculator *SimilarityCalculator) *BruteForceIndex {
	return &BruteForceIndex{
		embeddings: make(map[string][]float32),
		calculator: calculator,
	}
}

func (idx *BruteForceIndex) Add(id string, embedding []float32) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.embeddings[id] = embedding
	return nil
}

func (idx *BruteForceIndex) Search(embedding []float32, k int, threshold float64) ([]SearchResult, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	results := make([]SearchResult, 0)
	for id, emb := range idx.embeddings {
		score := idx.calculator.Calculate(embedding, emb)
		if score >= threshold {
			results = append(results, SearchResult{ID: id, Score: score})
		}
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > k {
		results = results[:k]
	}

	return results, nil
}

func (idx *BruteForceIndex) SearchByID(id string, k int, threshold float64) ([]SearchResult, error) {
	idx.mu.RLock()
	embedding, ok := idx.embeddings[id]
	idx.mu.RUnlock()

	if !ok {
		return nil, nil
	}

	return idx.Search(embedding, k, threshold)
}

func (idx *BruteForceIndex) Get(id string) ([]float32, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	emb, ok := idx.embeddings[id]
	if !ok {
		return nil, nil
	}
	return emb, nil
}

func (idx *BruteForceIndex) Delete(id string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	delete(idx.embeddings, id)
	return nil
}

func (idx *BruteForceIndex) Size() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.embeddings)
}

func (idx *BruteForceIndex) Close() error {
	return nil
}

// ==================== DBSCAN聚类优化 ====================

// FaceClusterer 人脸聚类器
// 使用优化的DBSCAN算法进行人脸聚类
type FaceClusterer struct {
	config     *ClusterConfig
	calculator *SimilarityCalculator
	index      ClusterIndex
	logger     *zap.Logger
}

// NewFaceClusterer 创建人脸聚类器
func NewFaceClusterer(config *ClusterConfig, logger *zap.Logger) *FaceClusterer {
	if config == nil {
		config = DefaultClusterConfig()
	}

	calculator := NewSimilarityCalculator(config.SimilarityMetric)

	// 根据配置选择索引类型
	var index ClusterIndex
	if config.UseIndex {
		switch config.IndexType {
		case "hnsw":
			// HNSW索引需要外部库支持，这里使用暴力索引作为fallback
			index = NewBruteForceIndex(calculator)
		case "faiss":
			// FAISS索引需要外部库支持
			index = NewBruteForceIndex(calculator)
		default:
			index = NewBruteForceIndex(calculator)
		}
	} else {
		index = NewBruteForceIndex(calculator)
	}

	return &FaceClusterer{
		config:     config,
		calculator: calculator,
		index:      index,
		logger:     logger,
	}
}

// Cluster 执行人脸聚类
// 核心算法：优化的DBSCAN
func (fc *FaceClusterer) Cluster(ctx context.Context, faces []*FaceEmbedding) (*ClusterResult, error) {
	start := time.Now()

	// 1. 预过滤低质量人脸
	if fc.config.PreFilter {
		faces = fc.preFilter(faces)
	}

	if len(faces) == 0 {
		return &ClusterResult{}, nil
	}

	// 2. 构建索引
	fc.buildIndex(faces)

	// 3. 并行DBSCAN聚类
	labels := fc.parallelDBSCAN(ctx, faces)

	// 4. 构建聚类结果
	result := fc.buildClusters(faces, labels)

	// 5. 迭代优化（可选）
	if fc.config.Iterative {
		fc.iterativeOptimize(ctx, result)
	}

	result.ProcessTime = time.Since(start)

	fc.logger.Info("聚类完成",
		zap.Int("total_faces", len(faces)),
		zap.Int("clusters", result.ClusterCount),
		zap.Duration("time", result.ProcessTime),
	)

	return result, nil
}

// preFilter 预过滤低质量人脸
func (fc *FaceClusterer) preFilter(faces []*FaceEmbedding) []*FaceEmbedding {
	filtered := make([]*FaceEmbedding, 0, len(faces))
	for _, face := range faces {
		if face.Quality >= fc.config.MinQuality && face.Embedding != nil {
			filtered = append(filtered, face)
		}
	}
	return filtered
}

// buildIndex 构建搜索索引
func (fc *FaceClusterer) buildIndex(faces []*FaceEmbedding) {
	for _, face := range faces {
		_ = fc.index.Add(face.ID, face.Embedding)
	}
}

// parallelDBSCAN 并行DBSCAN算法
// 使用分批并行处理优化大规模数据性能
func (fc *FaceClusterer) parallelDBSCAN(ctx context.Context, faces []*FaceEmbedding) map[string]int {
	n := len(faces)

	// 初始化标签
	labels := make(map[string]int, n)
	for _, face := range faces {
		labels[face.ID] = -1 // -1 = 未分配/噪声
	}

	// 并行计算邻居关系
	neighborMap := fc.computeNeighborsParallel(ctx, faces)

	// DBSCAN主循环
	clusterID := 0
	epsilon := fc.config.Epsilon
	minPts := fc.config.MinPoints

	for _, face := range faces {
		// 已分配则跳过
		if labels[face.ID] != -1 {
			continue
		}

		// 获取邻居
		neighbors := neighborMap[face.ID]

		// 核心点判断
		if len(neighbors) < minPts {
			// 噪声点，保持-1
			continue
		}

		// 开始新聚类
		clusterID++
		labels[face.ID] = clusterID

		// 扩展聚类（使用队列优化）
		fc.expandCluster(labels, neighbors, clusterID, neighborMap, minPts)
	}

	return labels
}

// computeNeighborsParallel 并行计算邻居关系
// 核心优化：批量并行相似度计算
func (fc *FaceClusterer) computeNeighborsParallel(ctx context.Context, faces []*FaceEmbedding) map[string][]string {
	neighborMap := make(map[string][]string)

	// 分批并行处理
	batchSize := fc.config.BatchSize
	numWorkers := fc.config.MaxWorkers

	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
	}

	// 创建任务队列
	taskCh := make(chan []*FaceEmbedding, numWorkers*2)
	resultCh := make(chan neighborResult, numWorkers*2)

	// 启动worker
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go fc.neighborWorker(taskCh, resultCh, &wg)
	}

	// 分发任务
	go func() {
		for i := 0; i < len(faces); i += batchSize {
			end := i + batchSize
			if end > len(faces) {
				end = len(faces)
			}
			taskCh <- faces[i:end]
		}
		close(taskCh)
	}()

	// 收集结果
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 合并结果
	for result := range resultCh {
		for id, neighbors := range result.Neighbors {
			neighborMap[id] = neighbors
		}
	}

	return neighborMap
}

// neighborResult 邻居计算结果
type neighborResult struct {
	Neighbors map[string][]string
}

// neighborWorker 邻居计算worker
func (fc *FaceClusterer) neighborWorker(taskCh <-chan []*FaceEmbedding, resultCh chan<- neighborResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for batch := range taskCh {
		neighbors := make(map[string][]string)

		for _, face := range batch {
			// 使用索引搜索邻居
			results, err := fc.index.Search(face.Embedding, fc.config.MaxClusterSize, fc.config.Epsilon)
			if err != nil {
				continue
			}

			ids := make([]string, 0, len(results))
			for _, r := range results {
				if r.ID != face.ID {
					ids = append(ids, r.ID)
				}
			}
			neighbors[face.ID] = ids
		}

		resultCh <- neighborResult{Neighbors: neighbors}
	}
}

// expandCluster 扩展聚类
// 使用队列优化，避免递归
func (fc *FaceClusterer) expandCluster(labels map[string]int, neighbors []string, clusterID int, neighborMap map[string][]string, minPts int) {
	// 使用队列
	queue := make([]string, len(neighbors))
	copy(queue, neighbors)

	visited := make(map[string]bool)

	for len(queue) > 0 {
		// 出队
		current := queue[0]
		queue = queue[1:]

		if visited[current] {
			continue
		}
		visited[current] = true

		// 分配聚类标签
		if labels[current] == -1 {
			labels[current] = clusterID
		}

		// 如果是核心点，扩展邻居
		currentNeighbors := neighborMap[current]
		if len(currentNeighbors) >= minPts {
			for _, neighbor := range currentNeighbors {
				if !visited[neighbor] && labels[neighbor] == -1 {
					queue = append(queue, neighbor)
				}
			}
		}
	}
}

// buildClusters 构建聚类结果
func (fc *FaceClusterer) buildClusters(faces []*FaceEmbedding, labels map[string]int) *ClusterResult {
	// 按聚类ID分组
	clusterFaces := make(map[int][]*FaceEmbedding)
	unassigned := make([]*FaceEmbedding, 0)

	for _, face := range faces {
		label := labels[face.ID]
		if label == -1 {
			face.ClusterID = -1
			unassigned = append(unassigned, face)
		} else {
			face.ClusterID = label
			clusterFaces[label] = append(clusterFaces[label], face)
		}
	}

	// 构建人物聚类
	persons := make([]*PersonCluster, 0, len(clusterFaces))
	now := time.Now()

	for clusterID, faces := range clusterFaces {
		// 计算聚类中心
		center := fc.computeCenter(faces)

		// 选择代表人脸（质量最高）
		sort.Slice(faces, func(i, j int) bool {
			return faces[i].Quality > faces[j].Quality
		})

		// 计算平均质量
		var avgQuality float64
		for _, f := range faces {
			avgQuality += f.Quality
		}
		avgQuality /= float64(len(faces))

		person := &PersonCluster{
			ID:              generateClusterID(clusterID),
			FaceCount:       len(faces),
			Representative:  faces[0],
			CenterEmbedding: center,
			Faces:           faces,
			CoverPhotoID:    faces[0].PhotoID,
			AvgQuality:      avgQuality,
			CreatedAt:       now,
			UpdatedAt:       now,
		}

		persons = append(persons, person)
	}

	// 按人脸数量排序
	sort.Slice(persons, func(i, j int) bool {
		return persons[i].FaceCount > persons[j].FaceCount
	})

	return &ClusterResult{
		Persons:      persons,
		Unassigned:   unassigned,
		TotalFaces:   len(faces),
		ClusterCount: len(persons),
	}
}

// computeCenter 计算聚类中心embedding
func (fc *FaceClusterer) computeCenter(faces []*FaceEmbedding) []float32 {
	if len(faces) == 0 {
		return nil
	}

	dim := len(faces[0].Embedding)
	center := make([]float32, dim)

	for _, face := range faces {
		for i, v := range face.Embedding {
			center[i] += v
		}
	}

	n := float32(len(faces))
	for i := range center {
		center[i] /= n
	}

	return center
}

// iterativeOptimize 迭代优化聚类
// 通过重新计算聚类中心和重新分配边界点来优化聚类
func (fc *FaceClusterer) iterativeOptimize(ctx context.Context, result *ClusterResult) {
	for iter := 0; iter < fc.config.MaxIterations; iter++ {
		changed := false

		// 重新计算聚类中心
		for _, person := range result.Persons {
			person.CenterEmbedding = fc.computeCenter(person.Faces)
		}

		// 重新分配边界点（检查是否可以合并相近聚类）
		for i := 0; i < len(result.Persons); i++ {
			for j := i + 1; j < len(result.Persons); j++ {
				// 计算聚类中心相似度
				sim := fc.calculator.Calculate(
					result.Persons[i].CenterEmbedding,
					result.Persons[j].CenterEmbedding,
				)

				// 如果两个聚类中心相似度超过阈值，合并
				if sim >= fc.config.Epsilon {
					fc.mergeClusters(result.Persons[i], result.Persons[j])
					// 移除被合并的聚类
					result.Persons = append(result.Persons[:j], result.Persons[j+1:]...)
					result.ClusterCount--
					changed = true
					break
				}
			}
		}

		// 收敛判断
		if !changed {
			break
		}
	}
}

// mergeClusters 合并两个聚类
func (fc *FaceClusterer) mergeClusters(target, source *PersonCluster) {
	// 合合人脸列表
	target.Faces = append(target.Faces, source.Faces...)
	target.FaceCount = len(target.Faces)

	// 重新计算中心
	target.CenterEmbedding = fc.computeCenter(target.Faces)

	// 更新代表人脸（质量最高）
	sort.Slice(target.Faces, func(i, j int) bool {
		return target.Faces[i].Quality > target.Faces[j].Quality
	})
	target.Representative = target.Faces[0]
	target.CoverPhotoID = target.Faces[0].PhotoID

	// 更新聚类ID
	for _, face := range source.Faces {
		face.ClusterID = target.ID
	}
	target.UpdatedAt = time.Now()
}

// ==================== 增量聚类 ====================

// IncrementalClusterer 增量聚类器
// 支持新人脸的增量添加，无需重新计算全部聚类
type IncrementalClusterer struct {
	faceClusterer *FaceClusterer
	clusters      []*PersonCluster
	faceMap       map[string]*FaceEmbedding // face_id -> face
	index         ClusterIndex
	mu            sync.RWMutex
}

// NewIncrementalClusterer 创建增量聚类器
func NewIncrementalClusterer(config *ClusterConfig, logger *zap.Logger) *IncrementalClusterer {
	fc := NewFaceClusterer(config, logger)
	return &IncrementalClusterer{
		faceClusterer: fc,
		clusters:      make([]*PersonCluster, 0),
		faceMap:       make(map[string]*FaceEmbedding),
		index:         fc.index,
	}
}

// AddFace 增量添加人脸
func (ic *IncrementalClusterer) AddFace(ctx context.Context, face *FaceEmbedding) (*PersonCluster, error) {
	ic.mu.Lock()
	defer ic.mu.Unlock()

	// 添加到索引
	_ = ic.index.Add(face.ID, face.Embedding)
	ic.faceMap[face.ID] = face

	// 搜索最相似的聚类
	bestCluster := ic.findBestCluster(face)

	if bestCluster != nil {
		// 添加到现有聚类
		bestCluster.Faces = append(bestCluster.Faces, face)
		bestCluster.FaceCount++
		face.ClusterID = bestCluster.ID

		// 更新聚类中心
		bestCluster.CenterEmbedding = ic.faceClusterer.computeCenter(bestCluster.Faces)
		bestCluster.UpdatedAt = time.Now()

		return bestCluster, nil
	}

	// 创建新聚类
	newCluster := &PersonCluster{
		ID:              generateClusterID(len(ic.clusters) + 1),
		FaceCount:       1,
		Representative:  face,
		CenterEmbedding: face.Embedding,
		Faces:           []*FaceEmbedding{face},
		CoverPhotoID:    face.PhotoID,
		AvgQuality:      face.Quality,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Time{},
	}
	face.ClusterID = newCluster.ID

	ic.clusters = append(ic.clusters, newCluster)
	return newCluster, nil
}

// findBestCluster 寻找最佳匹配聚类
func (ic *IncrementalClusterer) findBestCluster(face *FaceEmbedding) *PersonCluster {
	if len(ic.clusters) == 0 {
		return nil
	}

	bestSim := ic.faceClusterer.config.Epsilon
	bestCluster := (*PersonCluster)(nil)

	for _, cluster := range ic.clusters {
		sim := ic.faceClusterer.calculator.Calculate(face.Embedding, cluster.CenterEmbedding)
		if sim > bestSim {
			bestSim = sim
			bestCluster = cluster
		}
	}

	return bestCluster
}

// GetClusters 获取所有聚类
func (ic *IncrementalClusterer) GetClusters() []*PersonCluster {
	ic.mu.RLock()
	defer ic.mu.RUnlock()
	return ic.clusters
}

// GetClusterResult 获取完整聚类结果
func (ic *IncrementalClusterer) GetClusterResult() *ClusterResult {
	ic.mu.RLock()
	defer ic.mu.RUnlock()

	unassigned := make([]*FaceEmbedding, 0)
	for _, face := range ic.faceMap {
		if face.ClusterID == -1 {
			unassigned = append(unassigned, face)
		}
	}

	return &ClusterResult{
		Persons:      ic.clusters,
		Unassigned:   unassigned,
		TotalFaces:   len(ic.faceMap),
		ClusterCount: len(ic.clusters),
	}
}

// ==================== 辅助函数 ====================

// generateClusterID 生成聚类ID
func generateClusterID(clusterNum int) string {
	return fmt.Sprintf("person_%d_%d", clusterNum, time.Now().UnixNano())
}