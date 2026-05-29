// Package filedejavu 提供重复文件智能检测功能
package filedejavu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	chunkSize = 64 * 1024 // 64KB 分块读取
)

// Detector 重复文件检测器
type Detector struct {
	mu       sync.RWMutex
	config   *ScanConfig
	result   *ScanResult
	cancel   context.CancelFunc
}

// NewDetector 创建检测器
func NewDetector(config *ScanConfig) *Detector {
	if config == nil {
		config = DefaultScanConfig()
	}
	if config.Threshold <= 0 || config.Threshold > 1 {
		config.Threshold = 0.85
	}
	if config.MaxWorkers <= 0 {
		config.MaxWorkers = 4
	}
	return &Detector{
		config: config,
	}
}

// Config 返回当前配置
func (d *Detector) Config() *ScanConfig {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.config
}

// Result 返回扫描结果
func (d *Detector) Result() *ScanResult {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.result
}

// Scan 执行扫描
func (d *Detector) Scan(ctx context.Context) (*ScanResult, error) {
	ctx, cancel := context.WithCancel(ctx)
	d.mu.Lock()
	d.cancel = cancel
	d.result = &ScanResult{
		StartTime: time.Now(),
		Status:    "running",
	}
	d.mu.Unlock()

	defer func() {
		cancel()
		d.mu.Lock()
		d.result.EndTime = time.Now()
		d.result.Duration = d.result.EndTime.Sub(d.result.StartTime)
		if d.result.Status == "running" {
			d.result.Status = "completed"
		}
		d.mu.Unlock()
	}()

	// 第一阶段：收集文件信息
	files, err := d.collectFiles(ctx)
	if err != nil {
		d.mu.Lock()
		d.result.Status = "error"
		d.result.Error = err.Error()
		d.mu.Unlock()
		return nil, err
	}

	d.mu.Lock()
	d.result.TotalFiles = int64(len(files))
	for _, f := range files {
		d.result.TotalSize += f.Size
	}
	d.mu.Unlock()

	// 第二阶段：快速预筛（按大小分组）
	sizeGroups := d.groupBySize(files)

	// 第三阶段：计算哈希并分组
	hashGroups := d.groupByHash(ctx, sizeGroups)

	// 第四阶段：感知哈希检测相似图片
	if d.config.ScanImages {
		d.detectSimilarImages(ctx, files, hashGroups)
	}

	// 构建重复组
	d.buildDuplicateGroups(hashGroups)

	return d.result, nil
}

// Cancel 取消扫描
func (d *Detector) Cancel() {
	d.mu.RLock()
	cancel := d.cancel
	d.mu.RUnlock()
	if cancel != nil {
		cancel()
		d.mu.Lock()
		if d.result != nil {
			d.result.Status = "cancelled"
		}
		d.mu.Unlock()
	}
}

// collectFiles 收集文件信息
func (d *Detector) collectFiles(ctx context.Context) ([]*FileFingerprint, error) {
	var files []*FileFingerprint
	var mu sync.Mutex

	for _, root := range d.config.Paths {
		err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 跳过无法访问的文件
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			// 跳过目录
			if info.IsDir() {
				return nil
			}

			// 检查排除模式
			if d.isExcluded(path) {
				return nil
			}

			// 检查文件大小
			if info.Size() < d.config.MinFileSize {
				return nil
			}
			if d.config.MaxFileSize > 0 && info.Size() > d.config.MaxFileSize {
				return nil
			}

			fp := &FileFingerprint{
				Path:    path,
				Size:    info.Size(),
				ModTime: info.ModTime(),
				IsImage: IsImageFile(path),
			}

			mu.Lock()
			files = append(files, fp)
			mu.Unlock()

			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk %s: %w", root, err)
		}
	}

	return files, nil
}

// isExcluded 检查是否匹配排除模式
func (d *Detector) isExcluded(path string) bool {
	for _, pattern := range d.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}
	return false
}

// groupBySize 按文件大小分组（快速预筛）
func (d *Detector) groupBySize(files []*FileFingerprint) map[int64][]*FileFingerprint {
	groups := make(map[int64][]*FileFingerprint)
	for _, f := range files {
		groups[f.Size] = append(groups[f.Size], f)
	}
	// 过滤掉只有一个文件的组
	for size, group := range groups {
		if len(group) < 2 {
			delete(groups, size)
		}
	}
	return groups
}

// groupByHash 计算 SHA-256 并分组
func (d *Detector) groupByHash(ctx context.Context, sizeGroups map[int64][]*FileFingerprint) map[string][]*FileFingerprint {
	hashGroups := make(map[string][]*FileFingerprint)
	var mu sync.Mutex

	// 使用 worker pool 并行计算哈希
	jobs := make(chan *FileFingerprint, 100)
	var wg sync.WaitGroup

	for i := 0; i < d.config.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fp := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				hash, err := ComputeSHA256(fp.Path)
				if err != nil {
					continue
				}
				fp.SHA256 = hash

				mu.Lock()
				hashGroups[hash] = append(hashGroups[hash], fp)
				mu.Unlock()
			}
		}()
	}

	// 发送任务
	go func() {
		for _, group := range sizeGroups {
			for _, fp := range group {
				select {
				case <-ctx.Done():
					break
				case jobs <- fp:
				}
			}
		}
		close(jobs)
	}()

	wg.Wait()

	// 过滤掉只有一个文件的组
	for hash, group := range hashGroups {
		if len(group) < 2 {
			delete(hashGroups, hash)
		}
	}

	return hashGroups
}

// detectSimilarImages 检测相似图片
func (d *Detector) detectSimilarImages(ctx context.Context, files []*FileFingerprint, hashGroups map[string][]*FileFingerprint) {
	// 收集所有图片文件
	var images []*FileFingerprint
	for _, f := range files {
		if f.IsImage {
			images = append(images, f)
		}
	}

	if len(images) < 2 {
		return
	}

	// 计算感知哈希
	var mu sync.Mutex
	jobs := make(chan *FileFingerprint, 100)
	var wg sync.WaitGroup

	for i := 0; i < d.config.MaxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for fp := range jobs {
				select {
				case <-ctx.Done():
					return
				default:
				}

				phash, err := ComputePHash(fp.Path)
				if err != nil {
					continue
				}
				mu.Lock()
				fp.PerceptHash = phash
				mu.Unlock()
			}
		}()
	}

	go func() {
		for _, img := range images {
			select {
			case <-ctx.Done():
				break
			case jobs <- img:
			}
		}
		close(jobs)
	}()

	wg.Wait()

	// 按感知哈希相似度分组
	// 简化实现：使用汉明距离比较
	processed := make(map[string]bool)
	for i, img1 := range images {
		if img1.PerceptHash == "" || processed[img1.Path] {
			continue
		}

		group := []*FileFingerprint{img1}
		processed[img1.Path] = true

		for j := i + 1; j < len(images); j++ {
			img2 := images[j]
			if img2.PerceptHash == "" || processed[img2.Path] {
				continue
			}

			sim := ComparePHash(img1.PerceptHash, img2.PerceptHash)
			if sim >= d.config.Threshold {
				group = append(group, img2)
				processed[img2.Path] = true
			}
		}

		if len(group) >= 2 {
			// 创建虚拟哈希用于分组
			simKey := fmt.Sprintf("sim_%s_%d", img1.PerceptHash, time.Now().UnixNano())
			hashGroups[simKey] = group
		}
	}
}

// buildDuplicateGroups 构建重复组
func (d *Detector) buildDuplicateGroups(hashGroups map[string][]*FileFingerprint) {
	for hash, files := range hashGroups {
		if len(files) < 2 {
			continue
		}

		group := &DuplicateGroup{
			ID: fmt.Sprintf("grp_%x", sha256.Sum256([]byte(hash))),
			Files: files,
		}

		// 判断重复类型
		if strings.HasPrefix(hash, "sim_") {
			group.Type = DupSimilar
			group.SimScore = d.config.Threshold
		} else {
			group.Type = DupExact
			group.Hash = hash
		}

		// 计算可节省空间（保留一个，其余可删除）
		if len(files) > 1 {
			group.Savings = files[0].Size * int64(len(files)-1)
		}

		// 推荐保留的文件
		group.Recommend = d.recommendKeep(files)

		d.result.AddGroup(group)
	}
}

// recommendKeep 推荐保留的文件
func (d *Detector) recommendKeep(files []*FileFingerprint) *FileFingerprint {
	if len(files) == 0 {
		return nil
	}

	switch d.config.KeepStrategy {
	case KeepNewest:
		keep := files[0]
		for _, f := range files[1:] {
			if f.ModTime.After(keep.ModTime) {
				keep = f
			}
		}
		return keep

	case KeepOldest:
		keep := files[0]
		for _, f := range files[1:] {
			if f.ModTime.Before(keep.ModTime) {
				keep = f
			}
		}
		return keep

	case KeepLargest:
		keep := files[0]
		for _, f := range files[1:] {
			if f.Size > keep.Size {
				keep = f
			}
		}
		return keep

	case KeepFirst:
		sorted := make([]*FileFingerprint, len(files))
		copy(sorted, files)
		sort.Slice(sorted, func(i, j int) bool {
			return sorted[i].Path < sorted[j].Path
		})
		return sorted[0]

	default:
		return files[0]
	}
}

// ComputeSHA256 计算文件 SHA-256（分块读取，避免内存溢出）
func ComputeSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	buf := make([]byte, chunkSize)

	for {
		n, err := f.Read(buf)
		if n > 0 {
			h.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// ComputePHash 计算图片感知哈希
// 这是一个简化实现，生产环境应使用图像处理库
func ComputePHash(path string) (string, error) {
	// 读取文件前 8KB 作为简化的感知哈希输入
	// 实际实现应该：缩放图片 → 灰度化 → DCT → 提取低频 → 生成 64-bit 哈希
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	buf := make([]byte, 8192)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return "", err
	}

	h := sha256.Sum256(buf[:n])
	return hex.EncodeToString(h[:8]), nil
}

// ComparePHash 比较两个感知哈希的相似度
// 返回 0-1 之间的相似度分数
func ComparePHash(hash1, hash2 string) float64 {
	if hash1 == hash2 {
		return 1.0
	}

	if len(hash1) != len(hash2) {
		return 0.0
	}

	// 计算汉明距离
	bits1 := hexToBits(hash1)
	bits2 := hexToBits(hash2)

	if len(bits1) != len(bits2) {
		return 0.0
	}

	same := 0
	for i := range bits1 {
		if bits1[i] == bits2[i] {
			same++
		}
	}

	return float64(same) / float64(len(bits1))
}

// hexToBits 将十六进制字符串转换为比特数组
func hexToBits(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil
	}
	bits := make([]byte, len(b)*8)
	for i, byt := range b {
		for j := 0; j < 8; j++ {
			bits[i*8+j] = (byt >> (7 - j)) & 1
		}
	}
	return bits
}

// BatchDedup 批量去重操作
func (d *Detector) BatchDedup(ctx context.Context, req *BatchDedupRequest) (*BatchDedupResult, error) {
	d.mu.RLock()
	result := d.result
	d.mu.RUnlock()

	if result == nil {
		return nil, fmt.Errorf("no scan result available")
	}

	res := &BatchDedupResult{}
	groups := result.GetGroups()

	for _, group := range groups {
		select {
		case <-ctx.Done():
			return res, ctx.Err()
		default:
		}

		// 检查是否在请求的组 ID 列表中
		if len(req.GroupIDs) > 0 {
			found := false
			for _, id := range req.GroupIDs {
				if id == group.ID {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		if group.Recommend == nil {
			continue
		}

		keepPath := group.Recommend.Path
		for _, fp := range group.Files {
			if fp.Path == keepPath {
				continue
			}

			if req.DryRun {
				switch req.Action {
				case ActionDelete:
					res.DeletedFiles++
				case ActionSymlink:
					res.SymlinkFiles++
				case ActionHardlink:
					res.HardlinkFiles++
				}
				res.SavedBytes += fp.Size
				continue
			}

			err := d.executeAction(req.Action, fp.Path, keepPath)
			if err != nil {
				res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", fp.Path, err))
				continue
			}

			switch req.Action {
			case ActionDelete:
				res.DeletedFiles++
			case ActionSymlink:
				res.SymlinkFiles++
			case ActionHardlink:
				res.HardlinkFiles++
			}
			res.SavedBytes += fp.Size
		}

		res.ProcessedGroups++
	}

	return res, nil
}

// executeAction 执行去重动作
func (d *Detector) executeAction(action DedupAction, target, source string) error {
	switch action {
	case ActionDelete:
		return os.Remove(target)

	case ActionRecycle:
		recyclePath := target + ".recycled"
		return os.Rename(target, recyclePath)

	case ActionSymlink:
		if err := os.Remove(target); err != nil {
			return err
		}
		return os.Symlink(source, target)

	case ActionHardlink:
		if err := os.Remove(target); err != nil {
			return err
		}
		return os.Link(source, target)

	case ActionReport:
		return nil

	default:
		return fmt.Errorf("unknown action: %s", action)
	}
}
