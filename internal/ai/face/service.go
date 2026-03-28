// Package face - 人脸识别服务
// 实现本地人脸检测、特征提取、聚类分组
package face

import (
	"context"
	"fmt"
	"sync"
)

// Detector 人脸检测器接口
type Detector interface {
	// DetectFaces 从图片中检测人脸
	DetectFaces(ctx context.Context, imagePath string) ([]Face, error)
	// ExtractEmbedding 提取人脸特征向量
	ExtractEmbedding(ctx context.Context, imagePath string, faceRegion FaceRegion) ([]float32, error)
	// Close 关闭检测器
	Close() error
}

// Face 检测到的人脸信息
type Face struct {
	ID        string     `json:"id"`
	Region    FaceRegion `json:"region"`
	Embedding []float32  `json:"embedding"`
	Confidence float32   `json:"confidence"`
}

// FaceRegion 人脸区域坐标
type FaceRegion struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// FaceCluster 人脸聚类结果
type FaceCluster struct {
	ID        string   `json:"id"`        // 职群ID
	Label     string   `json:"label"`     // 人物名称（用户标注）
	FaceIDs   []string `json:"faceIds"`   // 该职群包含的人脸ID
	CoverFace string   `json:"coverFace"` // 代表人脸ID（封面）
	CreatedAt int64    `json:"createdAt"`
	UpdatedAt int64    `json:"updatedAt"`
}

// Service 人脸识别服务
type Service struct {
	detector Detector
	clusters  map[string]*FaceCluster
	faces     map[string]*Face
	mu        sync.RWMutex
}

// NewService 创建人脸识别服务
func NewService(detector Detector) *Service {
	return &Service{
		detector: detector,
		clusters: make(map[string]*FaceCluster),
		faces:    make(map[string]*Face),
	}
}

// ProcessImage 处理单张图片，检测并存储人脸
func (s *Service) ProcessImage(ctx context.Context, imagePath string) ([]string, error) {
	faces, err := s.detector.DetectFaces(ctx, imagePath)
	if err != nil {
		return nil, fmt.Errorf("detect faces failed: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	var faceIDs []string
	for _, face := range faces {
		faceID := generateFaceID(imagePath, face.Region)
		face.ID = faceID
		s.faces[faceID] = &face
		faceIDs = append(faceIDs, faceID)
	}

	return faceIDs, nil
}

// ClusterFaces 对人脸进行聚类分组
func (s *Service) ClusterFaces(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 收集所有人脸特征向量
	faceList := make([]*Face, 0, len(s.faces))
	for _, face := range s.faces {
		if len(face.Embedding) > 0 {
			faceList = append(faceList, face)
		}
	}

	if len(faceList) == 0 {
		return nil
	}

	// 执行聚类算法（基于相似度阈值）
	clusters := clusterBySimilarity(faceList, 0.6) // 0.6 为相似度阈值

	// 更新职群存储
	for i, clusterFaces := range clusters {
		clusterID := fmt.Sprintf("cluster_%d", i)
		faceIDs := make([]string, len(clusterFaces))
		for j, f := range clusterFaces {
			faceIDs[j] = f.ID
		}

		s.clusters[clusterID] = &FaceCluster{
			ID:        clusterID,
			FaceIDs:   faceIDs,
			CoverFace: faceIDs[0], // 第一个作为封面
			CreatedAt: currentTime(),
			UpdatedAt: currentTime(),
		}
	}

	return nil
}

// LabelCluster 为职群标注人名
func (s *Service) LabelCluster(clusterID, label string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, ok := s.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster not found: %s", clusterID)
	}

	cluster.Label = label
	cluster.UpdatedAt = currentTime()
	return nil
}

// GetCluster 获取指定职群
func (s *Service) GetCluster(clusterID string) (*FaceCluster, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, ok := s.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}
	return cluster, nil
}

// ListClusters 获取所有职群
func (s *Service) ListClusters() []*FaceCluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*FaceCluster, 0, len(s.clusters))
	for _, cluster := range s.clusters {
		result = append(result, cluster)
	}
	return result
}

// GetFacesByCluster 获取职群中的所有人脸
func (s *Service) GetFacesByCluster(clusterID string) ([]*Face, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cluster, ok := s.clusters[clusterID]
	if !ok {
		return nil, fmt.Errorf("cluster not found: %s", clusterID)
	}

	faces := make([]*Face, 0, len(cluster.FaceIDs))
	for _, faceID := range cluster.FaceIDs {
		if face, ok := s.faces[faceID]; ok {
			faces = append(faces, face)
		}
	}
	return faces, nil
}

// DeleteFace 删除单个人脸数据
func (s *Service) DeleteFace(faceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 从人脸存储中删除
	delete(s.faces, faceID)

	// 从各职群中移除
	for _, cluster := range s.clusters {
		for i, id := range cluster.FaceIDs {
			if id == faceID {
				cluster.FaceIDs = append(cluster.FaceIDs[:i], cluster.FaceIDs[i+1:]...)
				cluster.UpdatedAt = currentTime()
				break
			}
		}
	}

	return nil
}

// DeleteCluster 删除整个职群（含所有人脸）
func (s *Service) DeleteCluster(clusterID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cluster, ok := s.clusters[clusterID]
	if !ok {
		return fmt.Errorf("cluster not found: %s", clusterID)
	}

	// 删除职群中所有人脸
	for _, faceID := range cluster.FaceIDs {
		delete(s.faces, faceID)
	}

	// 删除职群
	delete(s.clusters, clusterID)

	return nil
}

// Close 关闭服务
func (s *Service) Close() error {
	if s.detector != nil {
		return s.detector.Close()
	}
	return nil
}

// clusterBySimilarity 基于相似度阈值的人脸聚类
func clusterBySimilarity(faces []*Face, threshold float32) [][]*Face {
	n := len(faces)
	if n == 0 {
		return nil
	}

	// 分配数组标记是否已归类
	assigned := make([]bool, n)
	clusters := make([][]*Face, 0)

	for i := 0; i < n; i++ {
		if assigned[i] {
			continue
		}

		// 创建新职群
		cluster := []*Face{faces[i]}
		assigned[i] = true

		// 寻找相似人脸
		for j := i + 1; j < n; j++ {
			if assigned[j] {
				continue
			}

			similarity := cosineSimilarity(faces[i].Embedding, faces[j].Embedding)
			if similarity >= threshold {
				cluster = append(cluster, faces[j])
				assigned[j] = true
			}
		}

		clusters = append(clusters, cluster)
	}

	return clusters
}

// cosineSimilarity 计算余弦相似度
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := 0; i < len(a); i++ {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (sqrt32(normA) * sqrt32(normB))
}

// 辅助函数
func generateFaceID(imagePath string, region FaceRegion) string {
	return fmt.Sprintf("%s_%d_%d_%d_%d", imagePath, region.X, region.Y, region.Width, region.Height)
}

func currentTime() int64 {
	return 0 // TODO: 使用实际时间
}

func sqrt32(x float32) float32 {
	return float32(0) // TODO: 实现sqrt
}