// Package face 人脸聚类实现
package face

import (
	"context"
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

	similarity := c.buildSimilarityMatrix(validFaces)
	labels := c.dbscan(similarity, c.config.ClusterThresh, c.config.MinClusterSize)

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

	for _, clusterFaces := range personFaces {
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
			clusterFaces[i].PersonID = person.ID
		}

		persons = append(persons, person)
	}

	sort.Slice(persons, func(i, j int) bool {
		return persons[i].FaceCount > persons[j].FaceCount
	})

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
		maxSim := 0.0
		for faceID, emb := range embeddings {
			if faceID == person.RepresentativeFace {
				sim := cosineSimilarityFloat32(face.Embedding, emb)
				if sim > maxSim {
					maxSim = sim
				}
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
		labels[i] = -1
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
			continue
		}

		clusterID++
		labels[i] = clusterID

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

// ==================== 增量聚类器 ====================

// IncrementalClusterer 增量聚类器
type IncrementalClusterer struct {
	config     *RecognitionConfig
	centroids  map[string][]float32
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
		c.updateCentroid(bestPersonID, face.Embedding)
		face.PersonID = bestPersonID
		return bestPersonID, nil
	}

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

	c.faceCounts[face.PersonID] = count - 1
}