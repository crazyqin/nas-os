// Package face 人脸识别服务实现
package face

import (
	"context"
	"image"
	"sort"
	"sync"
	"time"
)

// ServiceImpl 人脸识别服务实现
type ServiceImpl struct {
	config       *RecognitionConfig
	detector     Detector
	recognizer   Recognizer
	clusterer    Clusterer
	labelManager LabelManager

	embeddings map[string][]float32
	embMu      sync.RWMutex

	stats Stats
	mu    sync.RWMutex
}

// NewService 创建人脸识别服务
func NewService(config *RecognitionConfig) (*ServiceImpl, error) {
	if config == nil {
		config = DefaultRecognitionConfig()
	}

	var detector Detector
	var err error
	if config.UseExternalService {
		detector, err = NewExternalDetector(config)
	} else {
		detector, err = NewLocalDetector(config)
	}
	if err != nil {
		return nil, err
	}

	var recognizer Recognizer
	if config.UseExternalService {
		recognizer, err = NewExternalRecognizer(config)
	} else {
		recognizer, err = NewLocalRecognizer(config)
	}
	if err != nil {
		detector.Close()
		return nil, err
	}

	clusterer := NewDBSCANClusterer(config)
	labelManager := NewMemoryLabelManager()

	return &ServiceImpl{
		config:       config,
		detector:     detector,
		recognizer:   recognizer,
		clusterer:    clusterer,
		labelManager: labelManager,
		embeddings:   make(map[string][]float32),
		stats: Stats{
			ModelInfo: ModelInfo{
				DetectionModel:   config.DetectionModel,
				RecognitionModel: config.RecognitionModel,
				UseGPU:           config.UseGPU,
				ExternalService:  config.UseExternalService,
			},
		},
	}, nil
}

// DetectFaces 检测人脸
func (s *ServiceImpl) DetectFaces(ctx context.Context, img Image, photoID string) (*DetectionResult, error) {
	result, err := s.detector.Detect(ctx, img)
	if err != nil {
		return nil, err
	}

	for i := range result.Faces {
		result.Faces[i].PhotoID = photoID
	}

	s.mu.Lock()
	s.stats.TotalFaces += len(result.Faces)
	s.stats.PhotosWith++
	s.stats.LastProcessed = time.Now()
	s.mu.Unlock()

	return result, nil
}

// ExtractEmbeddings 提取人脸嵌入向量
func (s *ServiceImpl) ExtractEmbeddings(ctx context.Context, img Image, faces []Face) ([]Face, error) {
	result := make([]Face, len(faces))

	for i, face := range faces {
		embedding, err := s.recognizer.ExtractEmbedding(ctx, img, &face)
		if err != nil {
			result[i] = face
			continue
		}

		face.Embedding = embedding
		result[i] = face

		s.embMu.Lock()
		s.embeddings[face.ID] = embedding
		s.embMu.Unlock()
	}

	return result, nil
}

// RecognizeFaces 检测并识别人脸
func (s *ServiceImpl) RecognizeFaces(ctx context.Context, img Image, photoID string) ([]Face, error) {
	detection, err := s.detector.Detect(ctx, img)
	if err != nil {
		return nil, err
	}

	if len(detection.Faces) == 0 {
		return []Face{}, nil
	}

	faces := detection.Faces
	for i := range faces {
		faces[i].PhotoID = photoID

		embedding, err := s.recognizer.ExtractEmbedding(ctx, img, &faces[i])
		if err != nil {
			continue
		}
		faces[i].Embedding = embedding

		s.embMu.Lock()
		s.embeddings[faces[i].ID] = embedding
		s.embMu.Unlock()
	}

	persons, _, _ := s.labelManager.ListPersons(ctx, 1000, 0)
	for i := range faces {
		if len(faces[i].Embedding) == 0 {
			continue
		}

		personID, _ := s.clusterer.Assign(ctx, &faces[i], persons, s.embeddings)
		if personID != "" {
			faces[i].PersonID = personID
			person, _ := s.labelManager.GetPerson(ctx, personID)
			if person != nil {
				faces[i].PersonName = person.Name
			}
		}
	}

	s.mu.Lock()
	s.stats.TotalFaces += len(faces)
	s.stats.LastProcessed = time.Now()
	s.mu.Unlock()

	return faces, nil
}

// ClusterFaces 人脸聚类
func (s *ServiceImpl) ClusterFaces(ctx context.Context, photoIDs []string) (*ClusterResult, error) {
	allFaces := make([]Face, 0)

	for _, photoID := range photoIDs {
		faces, err := s.labelManager.GetFacesByPhoto(ctx, photoID)
		if err != nil {
			continue
		}

		for i := range faces {
			s.embMu.RLock()
			if emb, exists := s.embeddings[faces[i].ID]; exists {
				faces[i].Embedding = emb
			}
			s.embMu.RUnlock()
		}

		allFaces = append(allFaces, faces...)
	}

	result, err := s.clusterer.Cluster(ctx, allFaces)
	if err != nil {
		return nil, err
	}

	for _, person := range result.Persons {
		existingPerson, _ := s.labelManager.GetPerson(ctx, person.ID)
		if existingPerson == nil {
			s.labelManager.CreatePerson(ctx, person.Name)
		}
	}

	s.mu.Lock()
	s.stats.TotalPersons = len(result.Persons)
	s.stats.Unassigned = len(result.Unassigned)
	s.mu.Unlock()

	return result, nil
}

// AutoLabel 自动标记人脸
func (s *ServiceImpl) AutoLabel(ctx context.Context, personID string, photoIDs []string) error {
	person, err := s.labelManager.GetPerson(ctx, personID)
	if err != nil {
		return err
	}
	_ = person // person用于验证存在性

	for _, photoID := range photoIDs {
		faces, err := s.labelManager.GetFacesByPhoto(ctx, photoID)
		if err != nil {
			continue
		}

		for _, face := range faces {
			if face.PersonID == "" {
				s.labelManager.AssignFaceToPerson(ctx, face.ID, personID)
			}
		}
	}

	return nil
}

// GetStats 获取统计信息
func (s *ServiceImpl) GetStats(ctx context.Context) (*Stats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	persons, total, _ := s.labelManager.ListPersons(ctx, 10, 0)
	topPersons := make([]PersonStats, 0, len(persons))
	for _, p := range persons {
		topPersons = append(topPersons, PersonStats{
			PersonID:   p.ID,
			PersonName: p.Name,
			FaceCount:  p.FaceCount,
		})
	}

	stats := s.stats
	stats.TotalPersons = total
	stats.TopPersons = topPersons

	return &stats, nil
}

// Close 关闭服务
func (s *ServiceImpl) Close() error {
	if s.detector != nil {
		s.detector.Close()
	}
	if s.recognizer != nil {
		s.recognizer.Close()
	}
	return nil
}

// ProcessImage 处理图像
func (s *ServiceImpl) ProcessImage(ctx context.Context, img image.Image, photoID string) ([]Face, error) {
	adapter := NewGoImageAdapter(img)
	return s.RecognizeFaces(ctx, adapter, photoID)
}

// BatchProcessImages 批量处理图像
func (s *ServiceImpl) BatchProcessImages(ctx context.Context, images map[string]image.Image) (map[string][]Face, error) {
	results := make(map[string][]Face)
	var mu sync.Mutex

	var wg sync.WaitGroup
	sem := make(chan struct{}, s.config.NumWorkers)

	for photoID, img := range images {
		wg.Add(1)
		go func(pid string, i image.Image) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			faces, err := s.ProcessImage(ctx, i, pid)
			if err != nil {
				return
			}

			mu.Lock()
			results[pid] = faces
			mu.Unlock()
		}(photoID, img)
	}

	wg.Wait()
	return results, nil
}

// CreatePerson 创建人物
func (s *ServiceImpl) CreatePerson(ctx context.Context, name string) (*Person, error) {
	return s.labelManager.CreatePerson(ctx, name)
}

// GetPerson 获取人物
func (s *ServiceImpl) GetPerson(ctx context.Context, personID string) (*Person, error) {
	return s.labelManager.GetPerson(ctx, personID)
}

// ListPersons 列出人物
func (s *ServiceImpl) ListPersons(ctx context.Context, limit, offset int) ([]Person, int, error) {
	return s.labelManager.ListPersons(ctx, limit, offset)
}

// UpdatePerson 更新人物
func (s *ServiceImpl) UpdatePerson(ctx context.Context, personID string, updates map[string]interface{}) error {
	return s.labelManager.UpdatePerson(ctx, personID, updates)
}

// DeletePerson 删除人物
func (s *ServiceImpl) DeletePerson(ctx context.Context, personID string) error {
	return s.labelManager.DeletePerson(ctx, personID)
}

// AssignFace 分配人脸
func (s *ServiceImpl) AssignFace(ctx context.Context, faceID, personID string) error {
	return s.labelManager.AssignFaceToPerson(ctx, faceID, personID)
}

// UnassignFace 取消分配
func (s *ServiceImpl) UnassignFace(ctx context.Context, faceID string) error {
	return s.labelManager.UnassignFace(ctx, faceID)
}

// GetFacesByPerson 获取人物的人脸
func (s *ServiceImpl) GetFacesByPerson(ctx context.Context, personID string) ([]Face, error) {
	return s.labelManager.GetFacesByPerson(ctx, personID)
}

// GetFacesByPhoto 获取照片的人脸
func (s *ServiceImpl) GetFacesByPhoto(ctx context.Context, photoID string) ([]Face, error) {
	return s.labelManager.GetFacesByPhoto(ctx, photoID)
}

// SearchSimilarFaces 搜索相似人脸
func (s *ServiceImpl) SearchSimilarFaces(ctx context.Context, embedding []float32, threshold float64, topK int) []FaceSearchResult {
	s.embMu.RLock()
	defer s.embMu.RUnlock()

	results := make([]FaceSearchResult, 0)

	for faceID, emb := range s.embeddings {
		sim := cosineSimilarityFloat32(embedding, emb)
		if sim >= threshold {
			results = append(results, FaceSearchResult{
				FaceID: faceID,
				Score:  sim,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > topK {
		results = results[:topK]
	}

	return results
}

// CompareFaces 比较两个人脸
func (s *ServiceImpl) CompareFaces(emb1, emb2 []float32) float64 {
	return s.recognizer.Compare(emb1, emb2)
}

// VerifyFace 验证人脸是否属于指定人物
func (s *ServiceImpl) VerifyFace(ctx context.Context, embedding []float32, personID string) (bool, float64, error) {
	faces, err := s.labelManager.GetFacesByPerson(ctx, personID)
	if err != nil {
		return false, 0, err
	}

	if len(faces) == 0 {
		return false, 0, nil
	}

	maxSim := 0.0
	s.embMu.RLock()
	for _, face := range faces {
		if emb, exists := s.embeddings[face.ID]; exists {
			sim := cosineSimilarityFloat32(embedding, emb)
			if sim > maxSim {
				maxSim = sim
			}
		}
	}
	s.embMu.RUnlock()

	threshold := s.config.ClusterThresh
	return maxSim >= threshold, maxSim, nil
}