// Package containerregistry 提供私有容器镜像仓库功能
// 对标 TrueNAS TrueCharts 容器镜像管理 / 群晖 Container Manager 镜像仓库
// 支持 OCI Distribution Spec / 镜像存储 / Tag管理 / 垃圾回收 / Web UI
package containerregistry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// Registry 容器镜像仓库.
type Registry struct {
	mu       sync.RWMutex
	config   *RegistryConfig
	images   map[string]*Image
	manifests map[string]*Manifest
	blobs    map[string]*Blob
	stats    *RegistryStats
}

// RegistryConfig 仓库配置.
type RegistryConfig struct {
	StorageRoot    string        `json:"storageRoot"`    // 存储根目录
	MaxBlobSize    int64         `json:"maxBlobSize"`    // 单层最大大小（字节）
	MaxImageSize   int64         `json:"maxImageSize"`   // 镜像最大大小
	GCInterval     time.Duration `json:"gcInterval"`     // GC间隔
	GCRetention    time.Duration `json:"gcRetention"`    // GC保留时间
	EnableAuth     bool          `json:"enableAuth"`     // 启用认证
	AnonymousRead  bool          `json:"anonymousRead"`  // 允许匿名读取
	DefaultDomain  string        `json:"defaultDomain"`  // 默认域名
}

// Image 镜像信息.
type Image struct {
	Name         string            `json:"name"`         // 镜像名（如 library/nginx）
	Tags         map[string]string `json:"tags"`         // tag -> digest
	LastUpdated  time.Time         `json:"lastUpdated"`  // 最后更新
	TotalSize    int64             `json:"totalSize"`    // 总大小
	PullCount    int64             `json:"pullCount"`    // 拉取次数
	Labels       map[string]string `json:"labels"`       // 标签
}

// Manifest 镜像清单.
type Manifest struct {
	Digest      string    `json:"digest"`      // 内容摘要
	SchemaVersion int     `json:"schemaVersion"` // Schema版本
	MediaType   string    `json:"mediaType"`   // 媒体类型
	Config      *Descriptor `json:"config"`    // 配置描述
	Layers      []*Descriptor `json:"layers"`  // 层描述
	Size        int64     `json:"size"`        // 总大小
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间
}

// Descriptor 描述符.
type Descriptor struct {
	MediaType string `json:"mediaType"` // 媒体类型
	Digest    string `json:"digest"`    // 内容摘要
	Size      int64  `json:"size"`      // 大小
}

// Blob 二进制层.
type Blob struct {
	Digest    string    `json:"digest"`    // 内容摘要
	Size      int64     `json:"size"`      // 大小
	Path      string    `json:"path"`      // 存储路径
	RefCount  int       `json:"refCount"`  // 引用计数
	CreatedAt time.Time `json:"createdAt"` // 创建时间
}

// RegistryStats 仓库统计.
type RegistryStats struct {
	mu           sync.RWMutex
	TotalImages  int       `json:"totalImages"`  // 总镜像数
	TotalBlobs   int       `json:"totalBlobs"`   // 总层数
	TotalSize    int64     `json:"totalSize"`    // 总大小
	TotalPulls   int64     `json:"totalPulls"`   // 总拉取次数
	TotalPushes  int64     `json:"totalPushes"`  // 总推送次数
	LastGC       time.Time `json:"lastGC"`       // 上次GC时间
	GCCleaned    int64     `json:"gcCleaned"`    // GC清理大小
}

// GarbageCollectionResult GC结果.
type GarbageCollectionResult struct {
	BlobsDeleted   int   `json:"blobsDeleted"`   // 删除的层数
	SpaceFreed     int64 `json:"spaceFreed"`     // 释放空间
	OrphansRemoved int   `json:"orphansRemoved"` // 移除的孤儿
	Duration       time.Duration `json:"duration"` // 耗时
}

// NewRegistry 创建新的镜像仓库.
func NewRegistry(config *RegistryConfig) *Registry {
	if config == nil {
		config = &RegistryConfig{
			StorageRoot:   "/var/lib/nas-registry",
			MaxBlobSize:   1 << 30, // 1GB
			MaxImageSize:  10 << 30, // 10GB
			GCInterval:    24 * time.Hour,
			GCRetention:   7 * 24 * time.Hour,
			DefaultDomain: "registry.local",
		}
	}
	return &Registry{
		config:    config,
		images:    make(map[string]*Image),
		manifests: make(map[string]*Manifest),
		blobs:     make(map[string]*Blob),
		stats:     &RegistryStats{},
	}
}

// PushImage 推送镜像.
func (r *Registry) PushImage(name, tag string, manifest *Manifest, layers []*Blob) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 验证镜像大小
	var totalSize int64
	for _, layer := range layers {
		totalSize += layer.Size
	}
	if totalSize > r.config.MaxImageSize {
		return fmt.Errorf("image size %d exceeds limit %d", totalSize, r.config.MaxImageSize)
	}

	// 存储层
	for _, layer := range layers {
		r.blobs[layer.Digest] = layer
	}

	// 存储清单
	r.manifests[manifest.Digest] = manifest

	// 更新镜像信息
	img, exists := r.images[name]
	if !exists {
		img = &Image{
			Name:   name,
			Tags:   make(map[string]string),
			Labels: make(map[string]string),
		}
		r.images[name] = img
	}
	img.Tags[tag] = manifest.Digest
	img.LastUpdated = time.Now()
	img.TotalSize = totalSize

	// 更新统计
	r.stats.mu.Lock()
	r.stats.TotalPushes++
	r.stats.TotalImages = len(r.images)
	r.stats.TotalBlobs = len(r.blobs)
	r.stats.TotalSize += totalSize
	r.stats.mu.Unlock()

	return nil
}

// PullImage 拉取镜像清单.
func (r *Registry) PullImage(name, tag string) (*Manifest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	img, exists := r.images[name]
	if !exists {
		return nil, fmt.Errorf("image %s not found", name)
	}

	digest, exists := img.Tags[tag]
	if !exists {
		return nil, fmt.Errorf("tag %s not found for image %s", tag, name)
	}

	manifest, exists := r.manifests[digest]
	if !exists {
		return nil, fmt.Errorf("manifest %s not found", digest)
	}

	// 更新拉取统计
	r.stats.mu.Lock()
	r.stats.TotalPulls++
	r.stats.mu.Unlock()

	img.PullCount++

	return manifest, nil
}

// ListImages 列出所有镜像.
func (r *Registry) ListImages() []*Image {
	r.mu.RLock()
	defer r.mu.RUnlock()

	images := make([]*Image, 0, len(r.images))
	for _, img := range r.images {
		images = append(images, img)
	}
	return images
}

// GetImageTags 获取镜像的所有Tag.
func (r *Registry) GetImageTags(name string) (map[string]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	img, exists := r.images[name]
	if !exists {
		return nil, fmt.Errorf("image %s not found", name)
	}

	tags := make(map[string]string)
	for tag, digest := range img.Tags {
		tags[tag] = digest
	}
	return tags, nil
}

// DeleteTag 删除镜像Tag.
func (r *Registry) DeleteTag(name, tag string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	img, exists := r.images[name]
	if !exists {
		return fmt.Errorf("image %s not found", name)
	}

	if _, exists := img.Tags[tag]; !exists {
		return fmt.Errorf("tag %s not found for image %s", tag, name)
	}

	delete(img.Tags, tag)
	img.LastUpdated = time.Now()

	// 如果没有tag了，删除镜像
	if len(img.Tags) == 0 {
		delete(r.images, name)
	}

	return nil
}

// GarbageCollection 执行垃圾回收.
func (r *Registry) GarbageCollection() *GarbageCollectionResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	start := time.Now()
	result := &GarbageCollectionResult{}

	// 收集所有被引用的blob
	referenced := make(map[string]bool)
	for _, manifest := range r.manifests {
		if manifest.Config != nil {
			referenced[manifest.Config.Digest] = true
		}
		for _, layer := range manifest.Layers {
			referenced[layer.Digest] = true
		}
	}

	// 删除未引用的blob
	for digest, blob := range r.blobs {
		if !referenced[digest] {
			result.BlobsDeleted++
			result.SpaceFreed += blob.Size
			delete(r.blobs, digest)
		}
	}

	// 更新统计
	r.stats.mu.Lock()
	r.stats.LastGC = time.Now()
	r.stats.GCCleaned = result.SpaceFreed
	r.stats.TotalBlobs = len(r.blobs)
	r.stats.mu.Unlock()

	result.Duration = time.Since(start)
	return result
}

// GetStats 获取仓库统计.
func (r *Registry) GetStats() *RegistryStats {
	r.stats.mu.RLock()
	defer r.stats.mu.RUnlock()

	return &RegistryStats{
		TotalImages: r.stats.TotalImages,
		TotalBlobs:  r.stats.TotalBlobs,
		TotalSize:   r.stats.TotalSize,
		TotalPulls:  r.stats.TotalPulls,
		TotalPushes: r.stats.TotalPushes,
		LastGC:      r.stats.LastGC,
		GCCleaned:   r.stats.GCCleaned,
	}
}

// SearchImages 搜索镜像.
func (r *Registry) SearchImages(query string) []*Image {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []*Image
	for _, img := range r.images {
		if contains(img.Name, query) {
			results = append(results, img)
		}
	}
	return results
}

// GenerateDigest 生成内容摘要.
func GenerateDigest(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("sha256:%x", hash)
}

// MarshalManifest 序列化清单.
func MarshalManifest(m *Manifest) ([]byte, error) {
	return json.Marshal(m)
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
