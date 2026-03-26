// Package face 单元测试
package face

import (
	"context"
	"image"
	"image/color"
	"testing"
	"time"
)

// ==================== 测试辅助函数 ====================

// createTestImage 创建测试图像
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// 填充灰色
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.RGBA{128, 128, 128, 255})
		}
	}
	return img
}

// createTestImageAdapter 创建测试图像适配器
func createTestImageAdapter(width, height int) *GoImageAdapter {
	return NewGoImageAdapter(createTestImage(width, height))
}

// ==================== 类型测试 ====================

func TestFaceStruct(t *testing.T) {
	face := Face{
		ID:     "face_123",
		PhotoID: "photo_456",
		BoundingBox: BoundingBox{
			X:      0.1,
			Y:      0.2,
			Width:  0.3,
			Height: 0.4,
		},
		Confidence: 0.95,
		Quality:    0.9,
		CreatedAt:  time.Now(),
	}

	if face.ID != "face_123" {
		t.Errorf("期望 ID 为 face_123, 得到 %s", face.ID)
	}

	if face.BoundingBox.X != 0.1 {
		t.Errorf("期望 X 为 0.1, 得到 %f", face.BoundingBox.X)
	}
}

func TestPersonStruct(t *testing.T) {
	person := Person{
		ID:        "person_123",
		Name:      "张三",
		FaceCount: 10,
		CreatedAt: time.Now(),
	}

	if person.Name != "张三" {
		t.Errorf("期望名称为 张三, 得到 %s", person.Name)
	}

	if person.FaceCount != 10 {
		t.Errorf("期望人脸数为 10, 得到 %d", person.FaceCount)
	}
}

func TestRecognitionConfigDefaults(t *testing.T) {
	config := DefaultRecognitionConfig()

	if config.MinFaceSize != 30 {
		t.Errorf("期望 MinFaceSize 为 30, 得到 %d", config.MinFaceSize)
	}

	if config.ConfidenceThresh != 0.8 {
		t.Errorf("期望 ConfidenceThresh 为 0.8, 得到 %f", config.ConfidenceThresh)
	}

	if config.ClusterThresh != 0.6 {
		t.Errorf("期望 ClusterThresh 为 0.6, 得到 %f", config.ClusterThresh)
	}
}

// ==================== 图像适配器测试 ====================

func TestGoImageAdapter(t *testing.T) {
	img := createTestImage(100, 100)
	adapter := NewGoImageAdapter(img)

	// 测试Bounds
	w, h := adapter.Bounds()
	if w != 100 || h != 100 {
		t.Errorf("期望尺寸 100x100, 得到 %dx%d", w, h)
	}

	// 测试ToRGB
	rgb, err := adapter.ToRGB()
	if err != nil {
		t.Errorf("ToRGB 失败: %v", err)
	}
	if len(rgb) != 100*100*3 {
		t.Errorf("期望 RGB 长度 %d, 得到 %d", 100*100*3, len(rgb))
	}

	// 测试ToRGBA
	rgba, err := adapter.ToRGBA()
	if err != nil {
		t.Errorf("ToRGBA 失败: %v", err)
	}
	if len(rgba) != 100*100*4 {
		t.Errorf("期望 RGBA 长度 %d, 得到 %d", 100*100*4, len(rgba))
	}

	// 测试GetPixel
	r, g, b, a := adapter.GetPixel(50, 50)
	if r != 128 || g != 128 || b != 128 || a != 255 {
		t.Errorf("期望像素 (128,128,128,255), 得到 (%d,%d,%d,%d)", r, g, b, a)
	}
}

// ==================== 检测器测试 ====================

func TestLocalDetectorCreation(t *testing.T) {
	config := DefaultRecognitionConfig()
	detector, err := NewLocalDetector(config)

	if err != nil {
		t.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()
}

func TestLocalDetectorDetect(t *testing.T) {
	config := DefaultRecognitionConfig()
	detector, err := NewLocalDetector(config)
	if err != nil {
		t.Fatalf("创建检测器失败: %v", err)
	}
	defer detector.Close()

	img := createTestImageAdapter(200, 200)
	ctx := context.Background()

	result, err := detector.Detect(ctx, img)
	if err != nil {
		t.Errorf("检测失败: %v", err)
	}

	if result.Width != 200 || result.Height != 200 {
		t.Errorf("期望图像尺寸 200x200, 得到 %dx%d", result.Width, result.Height)
	}
}

func TestHOGBackend(t *testing.T) {
	config := DefaultRecognitionConfig()
	backend, err := NewHOGBackend(config)
	if err != nil {
		t.Fatalf("创建 HOG 后端失败: %v", err)
	}

	// 创建测试数据
	rgb := make([]byte, 100*100*3)
	for i := range rgb {
		rgb[i] = 128
	}

	faces, err := backend.Detect(context.Background(), rgb, 100, 100)
	if err != nil {
		t.Errorf("检测失败: %v", err)
	}

	// 测试图像可能没有人脸
	t.Logf("检测到 %d 个人脸", len(faces))
}

// ==================== 识别器测试 ====================

func TestLocalRecognizerCreation(t *testing.T) {
	config := DefaultRecognitionConfig()
	recognizer, err := NewLocalRecognizer(config)

	if err != nil {
		t.Fatalf("创建识别器失败: %v", err)
	}
	defer recognizer.Close()
}

func TestCosineSimilarity(t *testing.T) {
	// 相同向量
	emb1 := []float32{1.0, 0.0, 0.0}
	emb2 := []float32{1.0, 0.0, 0.0}

	sim := cosineSimilarityFloat32(emb1, emb2)
	if sim != 1.0 {
		t.Errorf("期望相似度为 1.0, 得到 %f", sim)
	}

	// 正交向量
	emb3 := []float32{0.0, 1.0, 0.0}
	sim = cosineSimilarityFloat32(emb1, emb3)
	if sim != 0.0 {
		t.Errorf("期望正交向量相似度为 0, 得到 %f", sim)
	}

	// 相反向量
	emb4 := []float32{-1.0, 0.0, 0.0}
	sim = cosineSimilarityFloat32(emb1, emb4)
	if sim != -1.0 {
		t.Errorf("期望相反向量相似度为 -1, 得到 %f", sim)
	}
}

func TestL2Normalize(t *testing.T) {
	vec := []float32{3.0, 4.0} // 模为5
	normalized := l2Normalize(vec)

	// 检查模为1
	var sum float64
	for _, v := range normalized {
		sum += float64(v) * float64(v)
	}

	if sum < 0.99 || sum > 1.01 {
		t.Errorf("期望归一化后模为1, 得到 %f", sum)
	}
}

// ==================== 聚类器测试 ====================

func TestDBSCANClusterer(t *testing.T) {
	config := DefaultRecognitionConfig()
	clusterer := NewDBSCANClusterer(config)

	// 创建测试人脸数据
	faces := []Face{
		{
			ID:         "face_1",
			Embedding:  []float32{1.0, 0.0, 0.0},
			Quality:    0.9,
			CreatedAt:  time.Now(),
		},
		{
			ID:         "face_2",
			Embedding:  []float32{0.95, 0.1, 0.0}, // 与face_1相似
			Quality:    0.85,
			CreatedAt:  time.Now(),
		},
		{
			ID:         "face_3",
			Embedding:  []float32{0.0, 1.0, 0.0}, // 与face_1不同
			Quality:    0.9,
			CreatedAt:  time.Now(),
		},
	}

	ctx := context.Background()
	result, err := clusterer.Cluster(ctx, faces)
	if err != nil {
		t.Errorf("聚类失败: %v", err)
	}

	t.Logf("聚类结果: %d 个人物, %d 未分配", result.ClusterCount, len(result.Unassigned))
}

func TestIncrementalClusterer(t *testing.T) {
	config := DefaultRecognitionConfig()
	clusterer := NewIncrementalClusterer(config)

	// 添加人脸
	face1 := &Face{
		ID:        "face_1",
		Embedding: []float32{1.0, 0.0, 0.0},
	}

	personID, err := clusterer.AddFace(face1)
	if err != nil {
		t.Errorf("添加人脸失败: %v", err)
	}

	if personID == "" {
		t.Error("期望返回人物ID")
	}

	// 添加相似人脸
	face2 := &Face{
		ID:        "face_2",
		Embedding: []float32{0.95, 0.1, 0.0},
	}

	personID2, _ := clusterer.AddFace(face2)
	if personID2 != personID {
		t.Errorf("期望相似人脸分配到同一人物")
	}

	// 获取人物列表
	persons := clusterer.GetPersons()
	if len(persons) != 1 {
		t.Errorf("期望1个人物, 得到 %d", len(persons))
	}
}

// ==================== 标签管理器测试 ====================

func TestMemoryLabelManager(t *testing.T) {
	manager := NewMemoryLabelManager()
	ctx := context.Background()

	// 创建人物
	person, err := manager.CreatePerson(ctx, "张三")
	if err != nil {
		t.Fatalf("创建人物失败: %v", err)
	}

	if person.Name != "张三" {
		t.Errorf("期望名称 张三, 得到 %s", person.Name)
	}

	// 获取人物
	p, err := manager.GetPerson(ctx, person.ID)
	if err != nil {
		t.Errorf("获取人物失败: %v", err)
	}

	if p.ID != person.ID {
		t.Errorf("期望ID %s, 得到 %s", person.ID, p.ID)
	}

	// 根据名称获取
	p2, err := manager.GetPersonByName(ctx, "张三")
	if err != nil {
		t.Errorf("根据名称获取人物失败: %v", err)
	}

	if p2.ID != person.ID {
		t.Error("根据名称获取的人物ID不匹配")
	}
}

func TestLabelManagerAssignFace(t *testing.T) {
	manager := NewMemoryLabelManager()
	ctx := context.Background()

	// 创建人物
	person, _ := manager.CreatePerson(ctx, "李四")

	// 添加人脸
	face := &Face{
		ID:        "face_1",
		PhotoID:   "photo_1",
		CreatedAt: time.Now(),
	}
	manager.AddFace(face)

	// 分配人脸
	err := manager.AssignFaceToPerson(ctx, "face_1", person.ID)
	if err != nil {
		t.Errorf("分配人脸失败: %v", err)
	}

	// 验证分配
	faces, err := manager.GetFacesByPerson(ctx, person.ID)
	if err != nil {
		t.Errorf("获取人物人脸失败: %v", err)
	}

	if len(faces) != 1 {
		t.Errorf("期望1个人脸, 得到 %d", len(faces))
	}

	// 取消分配
	err = manager.UnassignFace(ctx, "face_1")
	if err != nil {
		t.Errorf("取消分配失败: %v", err)
	}

	faces, _ = manager.GetFacesByPerson(ctx, person.ID)
	if len(faces) != 0 {
		t.Errorf("期望0个人脸, 得到 %d", len(faces))
	}
}

func TestLabelManagerListPersons(t *testing.T) {
	manager := NewMemoryLabelManager()
	ctx := context.Background()

	// 创建多个人物
	for i := 0; i < 5; i++ {
		manager.CreatePerson(ctx, string(rune('A'+i)))
	}

	// 列出人物
	persons, total, err := manager.ListPersons(ctx, 10, 0)
	if err != nil {
		t.Errorf("列出人物失败: %v", err)
	}

	if total != 5 {
		t.Errorf("期望总数5, 得到 %d", total)
	}

	if len(persons) != 5 {
		t.Errorf("期望5个人物, 得到 %d", len(persons))
	}
}

func TestLabelManagerDeletePerson(t *testing.T) {
	manager := NewMemoryLabelManager()
	ctx := context.Background()

	// 创建人物
	person, _ := manager.CreatePerson(ctx, "王五")

	// 添加人脸
	face := &Face{
		ID:        "face_1",
		CreatedAt: time.Now(),
	}
	manager.AddFace(face)
	manager.AssignFaceToPerson(ctx, "face_1", person.ID)

	// 删除人物
	err := manager.DeletePerson(ctx, person.ID)
	if err != nil {
		t.Errorf("删除人物失败: %v", err)
	}

	// 验证删除
	_, err = manager.GetPerson(ctx, person.ID)
	if err == nil {
		t.Error("期望人物不存在")
	}
}

// ==================== 服务测试 ====================

func TestServiceCreation(t *testing.T) {
	config := DefaultRecognitionConfig()
	service, err := NewService(config)

	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()
}

func TestServiceDetectFaces(t *testing.T) {
	config := DefaultRecognitionConfig()
	service, err := NewService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	img := createTestImageAdapter(200, 200)
	ctx := context.Background()

	result, err := service.DetectFaces(ctx, img, "photo_1")
	if err != nil {
		t.Errorf("检测人脸失败: %v", err)
	}

	if result.Width != 200 || result.Height != 200 {
		t.Errorf("期望图像尺寸 200x200, 得到 %dx%d", result.Width, result.Height)
	}
}

func TestServiceRecognizeFaces(t *testing.T) {
	config := DefaultRecognitionConfig()
	service, err := NewService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	img := createTestImageAdapter(200, 200)
	ctx := context.Background()

	faces, err := service.RecognizeFaces(ctx, img, "photo_1")
	if err != nil {
		t.Errorf("识别人脸失败: %v", err)
	}

	t.Logf("识别到 %d 个人脸", len(faces))
}

func TestServiceStats(t *testing.T) {
	config := DefaultRecognitionConfig()
	service, err := NewService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	ctx := context.Background()
	stats, err := service.GetStats(ctx)
	if err != nil {
		t.Errorf("获取统计失败: %v", err)
	}

	if stats.ModelInfo.DetectionModel != config.DetectionModel {
		t.Errorf("检测模型不匹配")
	}
}

func TestServiceCreatePerson(t *testing.T) {
	config := DefaultRecognitionConfig()
	service, err := NewService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}
	defer service.Close()

	ctx := context.Background()
	person, err := service.CreatePerson(ctx, "测试用户")
	if err != nil {
		t.Errorf("创建人物失败: %v", err)
	}

	if person.Name != "测试用户" {
		t.Errorf("期望名称 测试用户, 得到 %s", person.Name)
	}
}

// ==================== 辅助函数测试 ====================

func TestNMS(t *testing.T) {
	faces := []Face{
		{BoundingBox: BoundingBox{X: 0.1, Y: 0.1, Width: 0.2, Height: 0.2}, Confidence: 0.9},
		{BoundingBox: BoundingBox{X: 0.15, Y: 0.15, Width: 0.2, Height: 0.2}, Confidence: 0.8}, // 重叠
		{BoundingBox: BoundingBox{X: 0.5, Y: 0.5, Width: 0.2, Height: 0.2}, Confidence: 0.85}, // 不重叠
	}

	result := nms(faces, 0.3)

	// NMS应该保留非重叠的高置信度人脸
	if len(result) < 1 || len(result) > 2 {
		t.Errorf("期望保留1-2个人脸, 得到 %d", len(result))
	}
}

func TestComputeIOU(t *testing.T) {
	// 完全重叠
	iou := computeIOU(
		BoundingBox{X: 0, Y: 0, Width: 1, Height: 1},
		BoundingBox{X: 0, Y: 0, Width: 1, Height: 1},
	)
	if iou != 1.0 {
		t.Errorf("期望 IoU 为 1.0, 得到 %f", iou)
	}

	// 无重叠
	iou = computeIOU(
		BoundingBox{X: 0, Y: 0, Width: 0.5, Height: 0.5},
		BoundingBox{X: 0.6, Y: 0.6, Width: 0.5, Height: 0.5},
	)
	if iou != 0 {
		t.Errorf("期望 IoU 为 0, 得到 %f", iou)
	}

	// 部分重叠
	iou = computeIOU(
		BoundingBox{X: 0, Y: 0, Width: 0.5, Height: 0.5},
		BoundingBox{X: 0.25, Y: 0.25, Width: 0.5, Height: 0.5},
	)
	if iou <= 0 || iou >= 1 {
		t.Errorf("期望 0 < IoU < 1, 得到 %f", iou)
	}
}

// ==================== 基准测试 ====================

func BenchmarkCosineSimilarity(b *testing.B) {
	emb1 := make([]float32, 512)
	emb2 := make([]float32, 512)
	for i := range emb1 {
		emb1[i] = float32(i) / 512.0
		emb2[i] = float32(i+1) / 512.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cosineSimilarityFloat32(emb1, emb2)
	}
}

func BenchmarkL2Normalize(b *testing.B) {
	vec := make([]float32, 512)
	for i := range vec {
		vec[i] = float32(i) / 512.0
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l2Normalize(vec)
	}
}

func BenchmarkDetectFaces(b *testing.B) {
	config := DefaultRecognitionConfig()
	detector, _ := NewLocalDetector(config)
	defer detector.Close()

	img := createTestImageAdapter(640, 480)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		detector.Detect(ctx, img)
	}
}

func BenchmarkDBSCANCluster(b *testing.B) {
	config := DefaultRecognitionConfig()
	clusterer := NewDBSCANClusterer(config)

	// 创建100个测试人脸
	faces := make([]Face, 100)
	for i := range faces {
		emb := make([]float32, 512)
		for j := range emb {
			emb[j] = float32(i%10) / 10.0 + float32(j)/512.0
		}
		faces[i] = Face{
			ID:        generateID("face"),
			Embedding: emb,
			Quality:   0.9,
		}
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clusterer.Cluster(ctx, faces)
	}
}