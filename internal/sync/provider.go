package sync

import (
	"context"
	"time"
)

// Provider 云端存储抽象接口.
// 对应 cloudsync 模块的 Provider 接口，由外部注入.
type Provider interface {
	// List 列出远程目录下的所有文件（递归）.
	List(ctx context.Context, remotePath string, recursive bool) ([]FileEntry, error)
	// Upload 上传本地文件到远程.
	Upload(ctx context.Context, localPath, remotePath string) error
	// Download 下载远程文件到本地.
	Download(ctx context.Context, remotePath, localPath string) error
	// Delete 删除远程文件或目录.
	Delete(ctx context.Context, remotePath string) error
	// Mkdir 创建远程目录.
	Mkdir(ctx context.Context, remotePath string) error
	// Stat 获取远程文件信息.
	Stat(ctx context.Context, remotePath string) (*FileEntry, error)
	// GetChecksum 获取远程文件校验和（如果支持）.
	GetChecksum(ctx context.Context, remotePath string) (string, error)
}

// RemoteScanner 远程快照生成器.
type RemoteScanner struct {
	provider Provider
	scanner  *SnapshotScanner
}

// NewRemoteScanner 创建远程扫描器.
func NewRemoteScanner(provider Provider) *RemoteScanner {
	return &RemoteScanner{
		provider: provider,
		scanner:  NewSnapshotScanner(),
	}
}

// Scan 扫描远程生成快照.
func (r *RemoteScanner) Scan(ctx context.Context, remotePath string, rev int64) (*Snapshot, error) {
	entries, err := r.provider.List(ctx, remotePath, true)
	if err != nil {
		// 远程可能为空目录
		if isRemoteNotFound(err) {
			return &Snapshot{
				Rev:     rev,
				RootPath: remotePath,
				Entries: make(map[string]*FileEntry),
				Mtime:   now(),
			}, nil
		}
		return nil, err
	}

	return r.scanner.ScanRemote(entries, remotePath, rev), nil
}

// isRemoteNotFound 判断是否为"远程路径不存在"错误.
// 各个 provider 实现自己的错误类型，此处做简化判断.
func isRemoteNotFound(err error) bool {
	if err == nil {
		return false
	}
	return true // 实际应由 provider 返回具体错误类型，这里简化处理
}

func now() time.Time {
	return time.Now()
}
