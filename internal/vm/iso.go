package vm

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// ISOManager ISO 镜像管理器
type ISOManager struct {
	mu      sync.RWMutex
	isoPath string
	isos    map[string]*ISOImage
	logger  *zap.Logger
}

// NewISOManager 创建 ISO 管理器
func NewISOManager(isoPath string, logger *zap.Logger) (*ISOManager, error) {
	if isoPath == "" {
		isoPath = DefaultISOStoragePath
	}
	
	if err := os.MkdirAll(isoPath, 0755); err != nil {
		return nil, fmt.Errorf("创建 ISO 存储目录失败：%w", err)
	}
	
	m := &ISOManager{
		isoPath: isoPath,
		isos:    make(map[string]*ISOImage),
		logger:  logger,
	}
	
	// 加载现有 ISO
	if err := m.loadISOs(); err != nil {
		logger.Warn("加载 ISO 列表失败", zap.Error(err))
	}
	
	// 添加内置 ISO 下载源
	m.addBuiltInISOs()
	
	return m, nil
}

// loadISOs 加载现有 ISO 文件
func (m *ISOManager) loadISOs() error {
	files, err := os.ReadDir(m.isoPath)
	if err != nil {
		return err
	}
	
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		
		ext := filepath.Ext(file.Name())
		if ext != ".iso" && ext != ".ISO" {
			continue
		}
		
		filePath := filepath.Join(m.isoPath, file.Name())
		info, err := os.Stat(filePath)
		if err != nil {
			continue
		}
		
		isoID := "iso-" + uuid.New().String()[:8]
		iso := &ISOImage{
			ID:         isoID,
			Name:       file.Name(),
			Path:       filePath,
			Size:       uint64(info.Size()),
			CreatedAt:  info.ModTime(),
			UpdatedAt:  info.ModTime(),
			IsUploaded: true,
		}
		
		m.isos[isoID] = iso
	}
	
	return nil
}

// addBuiltInISOs 添加内置 ISO 下载源
func (m *ISOManager) addBuiltInISOs() {
	builtInISOs := []ISOImage{
		{
			ID:         "ubuntu-2204-lts",
			Name:       "Ubuntu 22.04 LTS",
			URL:        "https://releases.ubuntu.com/22.04/ubuntu-22.04.3-live-server-amd64.iso",
			OS:         "ubuntu",
			IsUploaded: false,
		},
		{
			ID:         "ubuntu-2004-lts",
			Name:       "Ubuntu 20.04 LTS",
			URL:        "https://releases.ubuntu.com/20.04/ubuntu-20.04.6-live-server-amd64.iso",
			OS:         "ubuntu",
			IsUploaded: false,
		},
		{
			ID:         "debian-11",
			Name:       "Debian 11",
			URL:        "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-11.8.0-amd64-netinst.iso",
			OS:         "debian",
			IsUploaded: false,
		},
		{
			ID:         "debian-12",
			Name:       "Debian 12",
			URL:        "https://cdimage.debian.org/debian-cd/current/amd64/iso-cd/debian-12.2.0-amd64-netinst.iso",
			OS:         "debian",
			IsUploaded: false,
		},
		{
			ID:         "centos-stream-9",
			Name:       "CentOS Stream 9",
			URL:        "https://mirror.stream.centos.org/9-stream/BaseOS/x86_64/iso/CentOS-Stream-9-latest-x86_64-boot.iso",
			OS:         "centos",
			IsUploaded: false,
		},
		{
			ID:         "windows-11",
			Name:       "Windows 11 ISO",
			URL:        "https://www.microsoft.com/software-download/windows11",
			OS:         "windows",
			IsUploaded: false,
		},
		{
			ID:         "windows-10",
			Name:       "Windows 10 ISO",
			URL:        "https://www.microsoft.com/software-download/windows10",
			OS:         "windows",
			IsUploaded: false,
		},
		{
			ID:         "almalinux-9",
			Name:       "AlmaLinux 9",
			URL:        "https://repo.almalinux.org/almalinux/9/isos/x86_64/AlmaLinux-9-latest-x86_64-boot.iso",
			OS:         "almalinux",
			IsUploaded: false,
		},
	}
	
	for _, iso := range builtInISOs {
		m.isos[iso.ID] = &iso
	}
}

// ListISOs 获取 ISO 列表
func (m *ISOManager) ListISOs() []*ISOImage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	isos := make([]*ISOImage, 0, len(m.isos))
	for _, iso := range m.isos {
		isos = append(isos, iso)
	}
	
	return isos
}

// GetISO 获取 ISO 信息
func (m *ISOManager) GetISO(isoID string) (*ISOImage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	iso, exists := m.isos[isoID]
	if !exists {
		return nil, fmt.Errorf("ISO %s 不存在", isoID)
	}
	
	return iso, nil
}

// UploadISO 上传 ISO 文件
func (m *ISOManager) UploadISO(ctx context.Context, name string, reader io.Reader) (*ISOImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// 生成文件名
	fileName := fmt.Sprintf("%s_%s.iso", name, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(m.isoPath, fileName)
	
	// 创建文件
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败：%w", err)
	}
	defer file.Close()
	
	// 复制内容
	written, err := io.Copy(file, reader)
	if err != nil {
		os.Remove(filePath)
		return nil, fmt.Errorf("写入文件失败：%w", err)
	}
	
	isoID := "iso-" + uuid.New().String()[:8]
	now := time.Now()
	
	iso := &ISOImage{
		ID:         isoID,
		Name:       fileName,
		Path:       filePath,
		Size:       uint64(written),
		CreatedAt:  now,
		UpdatedAt:  now,
		IsUploaded: true,
	}
	
	m.isos[isoID] = iso
	
	m.logger.Info("ISO 上传成功", zap.String("isoId", isoID), zap.String("name", fileName))
	
	return iso, nil
}

// DownloadISO 下载 ISO 镜像
func (m *ISOManager) DownloadISO(ctx context.Context, isoID string, progressChan chan<- int64) (*ISOImage, error) {
	m.mu.Lock()
	iso, exists := m.isos[isoID]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("ISO %s 不存在", isoID)
	}
	
	if iso.IsUploaded || iso.URL == "" {
		m.mu.Unlock()
		return nil, fmt.Errorf("该 ISO 不支持下载")
	}
	
	// 检查是否已下载
	if iso.Path != "" {
		if _, err := os.Stat(iso.Path); err == nil {
			m.mu.Unlock()
			return iso, nil
		}
	}
	m.mu.Unlock()
	
	// 创建 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, "GET", iso.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败：%w", err)
	}
	
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("下载失败：%w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	
	// 创建文件
	fileName := filepath.Base(iso.URL)
	filePath := filepath.Join(m.isoPath, fileName)
	file, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("创建文件失败：%w", err)
	}
	defer file.Close()
	
	// 带进度跟踪的复制
	var totalWritten int64
	buf := make([]byte, 32*1024)
	for {
		nr, er := resp.Body.Read(buf)
		if nr > 0 {
			nw, ew := file.Write(buf[0:nr])
			if nw < nr {
				return nil, fmt.Errorf("写入失败")
			}
			totalWritten += int64(nw)
			
			// 发送进度
			if progressChan != nil && resp.ContentLength > 0 {
				select {
				case progressChan <- totalWritten:
				default:
				}
			}
			
			if ew != nil {
				return nil, ew
			}
		}
		if er != nil {
			if er == io.EOF {
				break
			}
			return nil, er
		}
	}
	
	// 更新 ISO 信息
	m.mu.Lock()
	defer m.mu.Unlock()
	
	iso.Path = filePath
	iso.Size = uint64(totalWritten)
	iso.IsUploaded = true
	iso.UpdatedAt = time.Now()
	
	m.logger.Info("ISO 下载成功", zap.String("isoId", isoID), zap.String("path", filePath))
	
	return iso, nil
}

// DeleteISO 删除 ISO 文件
func (m *ISOManager) DeleteISO(isoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	iso, exists := m.isos[isoID]
	if !exists {
		return fmt.Errorf("ISO %s 不存在", isoID)
	}
	
	if !iso.IsUploaded {
		return fmt.Errorf("内置 ISO 不能删除")
	}
	
	// 删除文件
	if iso.Path != "" {
		if err := os.Remove(iso.Path); err != nil {
			return fmt.Errorf("删除文件失败：%w", err)
		}
	}
	
	delete(m.isos, isoID)
	
	m.logger.Info("ISO 删除成功", zap.String("isoId", isoID))
	
	return nil
}
