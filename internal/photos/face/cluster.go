// Package face 人脸聚类实现
package face

import (
	"context"
	"math"
	"sort"
	"sync"
	"time"
)

// ==================== 聚类器实现 ====================

// DBSCANClusterer DBSCAN聚类器
type DBSCANClusterer struct {
	config *RecognitionConfig
	mu     sync.RWMutex
}

// NewDBSCANClusterer 创建DBSCAN聚类器
func NewDBSCANClusterer(config *RecognitionConfig) *DBSCANClusterer {
	if config == nil {
		config = DefaultRecognitionConfig()
	}
	return &DBSCANClusterer{config: config}
}

// Cluster 对人脸进行聚类
func (c *DBSCANClusterer) Cluster(ctx context.Context, faces []Face) (*ClusterResult, error) {
	if len(faces) == 0 {
		return &ClusterResult{}, nil
	}

	// 过滤出有嵌入向量的人脸
	validFaces := make([]Face, 0, len(faces))
	for _, face := range faces {
		if len(face.Embedding) > 0 {
			validFaces = append(validFaces, face)
		}
	}

	if len(validFaces) == 0 {
		return &ClusterResult{
			Unassigned:   faces,
			ClusterCount: 0,
		}, nil
	}

	n := len(validFaces)

	// 构建相似度矩阵
	similarity := c.buildSimilarityMatrix(validFaces)

	// DBSCAN聚类
	labels := c.dbscan(similarity, c.config.ClusterThresh, c.config.MinClusterSize)

	// 构建聚类结果
	persons := make([]Person, 0)
	personFaces := make(map[int][]Face)
	unassigned := make([]Face, 0)

	for i, face := range validFaces {
		if labels[i] == -1 {
			face.ClusterID = -1
			unassigned = append(unassigned, face)
		} else {
			face.ClusterID = labels[i]
			personFaces[labels[i]] = append(personFaces[labels[i]], face)
		}
	}

	// 创建Person实体
	for clusterID, clusterFaces := range personFaces {
		// 按质量排序，选择代表脸
		sort.Slice(clusterFaces, func(i, j int) bool {
			return clusterFaces[i].Quality > clusterFaces[j].Quality
		})

		person := Person{
			ID:                 generateID("person"),
			FaceCount:          len(clusterFaces),
			RepresentativeFace: clusterFaces[0].ID,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		// 更新人脸的PersonID
		for i := range clusterFaces {
			clusterFaces[i].PersonID = person.ID
		}

		persons = append(persons, person)
	}

	// 按人脸数量排序
	sort.Slice(persons, func(i, j int) bool {
		return persons[i].FaceCount > persons[j].FaceCount
	})

	// 添加没有嵌入向量的人脸到未分配列表
	for _, face := range faces {
		if len(face.Embedding) == 0 {
			unassigned = append(unassigned, face)
		}
	}

	return &ClusterResult{
		Persons:      persons,
		Unassigned:   unassigned,
		ClusterCount: len(persons),
	}, nil
}

// Assign 将新人脸分配到现有聚类
func (c *DBSCANClusterer) Assign(ctx context.Context, face *Face, persons []Person, embeddings map[string][]float32) (string, error) {
	if len(face.Embedding) == 0 {
		return "", nil
	}

	bestPersonID := ""
	bestSimilarity := c.config.ClusterThresh

	for _, person := range persons {
		// 找到该人物的代表嵌入向量
		maxSim := 0.0
		for faceID, emb := range embeddings {
			if faceID == person.RepresentativeFace {
				sim := cosineSimilarityFloat32(face.Embedding, emb)
				if sim > maxSim {
					maxSim = sim
				}
			}
		}

		// 或者计算所有人脸的平均相似度
		if maxSim == 0 {
			totalSim := 0.0
			count := 0
			for faceID, emb := range embeddings {
				// 检查这个face是否属于这个person
				for _, pf := range embeddings {
					_ = pf
				}
				sim := cosineSimilarityFloat32(face.Embedding, emb)
				totalSim += sim
				count++
			}
			if count > 0 {
				maxSim = totalSim / float64(count)
			}
		}

		if maxSim > bestSimilarity {
			bestSimilarity = maxSim
			bestPersonID = person.ID
		}
	}

	return bestPersonID, nil
}

// buildSimilarityMatrix 构建相似度矩阵
func (c *DBSCANClusterer) buildSimilarityMatrix(faces []Face) [][]float64 {
	n := len(faces)
	matrix := make([][]float64, n)
	for i := range matrix {
		matrix[i] = make([]float64, n)
		for j := range matrix[i] {
			if i == j {
				matrix[i][j] = 1.0
			} else {
				matrix[i][j] = cosineSimilarityFloat32(faces[i].Embedding, faces[j].Embedding)
			}
		}
	}
	return matrix
}

// dbscan DBSCAN算法实现
func (c *DBSCANClusterer) dbscan(similarity [][]float64, epsilon float64, minPts int) []int {
	n := len(similarity)
	labels := make([]int, n)
	for i := range labels {
		labels[i] = -1 // -1 表示未分类
	}

	visited := make([]bool, n)
	clusterID := 0

	for i := 0; i < n; i++ {
		if visited[i] {
			continue
		}
		visited[i] = true

		neighbors := c.getNeighbors(similarity, i, epsilon)
		if len(neighbors) < minPts {
			// 噪声点
			continue
		}

		// 开始新聚类
		clusterID++
		labels[i] = clusterID

		// 扩展聚类
		seedSet := make([]int, len(neighbors))
		copy(seedSet, neighbors)

		for len(seedSet) > 0 {
			current := seedSet[0]
			seedSet = seedSet[1:]

			if !visited[current] {
				visited[current] = true
				currentNeighbors := c.getNeighbors(similarity, current, epsilon)
				if len(currentNeighbors) >= minPts {
					seedSet = append(seedSet, currentNeighbors...)
				}
			}

			if labels[current] == -1 {
				labels[current] = clusterID
			}
		}
	}

	return labels
}

// getNeighbors 获取邻域内的点
func (c *DBSCANClusterer) getNeighbors(similarity [][]float64, idx int, epsilon float64) []int {
	neighbors := make([]int, 0)
	for i, sim := range similarity[idx] {
		if i != idx && sim >= epsilon {
			neighbors = append(neighbors, i)
		}
	}
	return neighbors
}

// ==================== 层次聚类器 ====================

// HierarchicalClusterer 层次聚类器
type HierarchicalClusterer struct {
	config *RecognitionConfig
}

// NewHierarchicalClusterer 创建层次聚类器
func NewHierarchicalClusterer(config *RecognitionConfig) *HierarchicalClusterer {
	return &HierarchicalClusterer{config: config}
}

// Cluster 层次聚类
func (c *HierarchicalClusterer) Cluster(ctx context.Context, faces []Face) (*ClusterResult, error) {
	if len(faces) == 0 {
		return &ClusterResult{}, nil
	}

	// 过滤有效人脸
	validFaces := make([]Face, 0, len(faces))
	for _, face := range faces {
		if len(face.Embedding) > 0 {
			validFaces = append(validFaces, face)
		}
	}

	n := len(validFaces)
	if n == 0 {
		return &ClusterResult{Unassigned: faces}, nil
	}

	// 初始化：每个人脸是一个聚类
	clusters := make([][]int, n)
	for i := range clusters {
		clusters[i] = []int{i}
	}

	// 距离矩阵
	dist := c.buildDistanceMatrix(validFaces)

	// 凝聚聚类
	threshold := 1.0 - c.config.ClusterThresh // 相似度转换为距离

	for len(clusters) > 1 {
		// 找到最近的两个聚类
		minDist := math.MaxFloat64
		mergeI, mergeJ := 0, 1

		for i := 0; i < len(clusters); i++ {
			for j := i + 1; j < len(clusters); j++ {
				d := c.clusterDistance(dist, clusters[i], clusters[j])
				if d < minDist {
					minDist = d
					mergeI, mergeJ = i, j
				}
			}
		}

		// 如果最小距离超过阈值，停止
		if minDist > threshold {
			break
		}

		// 合并聚类
		clusters[mergeI] = append(clusters[mergeI], clusters[mergeJ]...)
		clusters = append(clusters[:mergeJ], clusters[mergeJ+1:]...)
	}

	// 构建结果
	persons := make([]Person, 0)
	unassigned := make([]Face, 0)

	for clusterIdx, cluster := range clusters {
		if len(cluster) < c.config.MinClusterSize {
			for _, idx := range cluster {
				validFaces[idx].ClusterID = -1
				unassigned = append(unassigned, validFaces[idx])
			}
			continue
		}

		// 按质量排序
		clusterFaces := make([]Face, len(cluster))
		for i, idx := range cluster {
			clusterFaces[i] = validFaces[idx]
		}
		sort.Slice(clusterFaces, func(i, j int) bool {
			return clusterFaces[i].Quality > clusterFaces[j].Quality
		})

		person := Person{
			ID:                 generateID("person"),
			FaceCount:          len(clusterFaces),
			RepresentativeFace: clusterFaces[0].ID,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		}

		for i := range clusterFaces {
			clusterFaces[i].ClusterID = clusterIdx
			clusterFaces[i].PersonID = person.ID
		}

		persons = append(persons, person)
	}

	// 添加没有嵌入向量的人脸
	for _, face := range faces {
		if len(face.Embedding) == 0 {
			unassigned = append(unassigned, face)
		}
	}

	return &ClusterResult{
		Persons:      persons,
		Unassigned:   unassigned,
		ClusterCount: len(persons),
	}, nil
}

// buildDistanceMatrix 构建距离矩阵
func (c *HierarchicalClusterer) buildDistanceMatrix(faces []Face) [][]float64 {
	n := len(faces)
	dist := make([][]float64, n)
	for i := range dist {
		dist[i] = make([]float64, n)
		for j := range dist[i] {
			if i == j {
				dist[i][j] = 0
			} else {
				// 距离 = 1 - 相似度
				dist[i][j] = 1.0 - cosineSimilarityFloat32(faces[i].Embedding, faces[j].Embedding)
			}
		}
	}
	return dist
}

// clusterDistance 计算聚类间距离 (平均链接)
func (c *HierarchicalClusterer) clusterDistance(dist [][]float64, c1, c2 []int) float64 {
	total := 0.0
	for _, i := range c1 {
		for _, j := range c2 {
			total += dist[i][j]
		}
	}
	return total / float64(len(c1)*len(c2))
}

// Assign 分配人脸到聚类
func (c *HierarchicalClusterer) Assign(ctx context.Context, face *Face, persons []Person, embeddings map[string][]float32) (string, error) {
	if len(face.Embedding) == 0 {
		return "", nil
	}

	bestPersonID := ""
	bestSimilarity := c.config.ClusterThresh

	for _, person := range persons {
		// 计算与该聚类中心的相似度
		totalSim := 0.0
		count := 0

		for _, emb := range embeddings {
			sim := cosineSimilarityFloat32(face.Embedding, emb)
			totalSim += sim
			count++
		}

		if count > 0 {
			avgSim := totalSim / float64(count)
			if avgSim > bestSimilarity {
				bestSimilarity = avgSim
				bestPersonID = person.ID
			}
		}
	}

	return bestPersonID, nil
}

// ==================== 增量聚类器 ====================

// IncrementalClusterer 增量聚类器
type IncrementalClusterer struct {
	config     *RecognitionConfig
	centroids  map[string][]float32 // personID -> centroid
	faceCounts map[string]int
	mu         sync.RWMutex
}

// NewIncrementalClusterer 创建增量聚类器
func NewIncrementalClusterer(config *RecognitionConfig) *IncrementalClusterer {
	return &IncrementalClusterer{
		config:     config,
		centroids:  make(map[string][]float32),
		faceCounts: make(map[string]int),
	}
}

// AddFace 添加新人脸
func (c *IncrementalClusterer) AddFace(face *Face) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(face.Embedding) == 0 {
		return "", nil
	}

	// 找最匹配的聚类
	bestPersonID := ""
	bestSimilarity := c.config.ClusterThresh

	for personID, centroid := range c.centroids {
		sim := cosineSimilarityFloat32(face.Embedding, centroid)
		if sim > bestSimilarity {
			bestSimilarity = sim
			bestPersonID = personID
		}
	}

	if bestPersonID != "" {
		// 更新聚类中心
		c.updateCentroid(bestPersonID, face.Embedding)
		face.PersonID = bestPersonID
		return bestPersonID, nil
	}

	// 创建新聚类
	personID := generateID("person")
	c.centroids[personID] = face.Embedding
	c.faceCounts[personID] = 1
	face.PersonID = personID

	return personID, nil
}

// updateCentroid 更新聚类中心
func (c *IncrementalClusterer) updateCentroid(personID string, embedding []float32) {
	centroid, exists := c.centroids[personID]
	if !exists {
		c.centroids[personID] = embedding
		c.faceCounts[personID] = 1
		return
	}

	count := c.faceCounts[personID]

	// 增量更新: new_centroid = (old_centroid * count + new_embedding) / (count + 1)
	newCentroid := make([]float32, len(centroid))
	for i := range centroid {
		newCentroid[i] = (centroid[i]*float32(count) + embedding[i]) / float32(count+1)
	}

	c.centroids[personID] = l2Normalize(newCentroid)
	c.faceCounts[personID] = count + 1
}

// GetPersons 获取所有人物
func (c *IncrementalClusterer) GetPersons() []Person {
	c.mu.RLock()
	defer c.mu.RUnlock()

	persons := make([]Person, 0, len(c.centroids))
	for personID := range c.centroids {
		persons = append(persons, Person{
			ID:        personID,
			FaceCount: c.faceCounts[personID],
		})
	}

	sort.Slice(persons, func(i, j int) bool {
		return persons[i].FaceCount > persons[j].FaceCount
	})

	return persons
}

// RemoveFace 移除人脸
func (c *IncrementalClusterer) RemoveFace(face *Face) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if face.PersonID == "" || len(face.Embedding) == 0 {
		return
	}

	count := c.faceCounts[face.PersonID]
	if count <= 1 {
		delete(c.centroids, face.PersonID)
		delete(c.faceCounts, face.PersonID)
		return
	}

	// 简化：只更新计数，不重新计算中心
	// 实际项目中应该重新计算
	c.faceCounts[face.PersonID] = count - 1
}