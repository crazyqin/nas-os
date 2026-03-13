package webdav

// QuotaProvider 配额提供者接口
type QuotaProvider interface {
	// CheckQuota 检查配额是否足够（返回剩余可用字节数）
	CheckQuota(username string) (available int64, err error)
	// ConsumeQuota 消费配额
	ConsumeQuota(username string, bytes int64) error
	// ReleaseQuota 释放配额
	ReleaseQuota(username string, bytes int64) error
	// GetUsage 获取使用情况
	GetUsage(username string) (used, total int64, err error)
}

// NoOpQuotaProvider 空配额提供者（不限制配额）
type NoOpQuotaProvider struct{}

// CheckQuota 检查配额
func (p *NoOpQuotaProvider) CheckQuota(username string) (int64, error) {
	return -1, nil // -1 表示无限制
}

// ConsumeQuota 消费配额
func (p *NoOpQuotaProvider) ConsumeQuota(username string, bytes int64) error {
	return nil
}

// ReleaseQuota 释放配额
func (p *NoOpQuotaProvider) ReleaseQuota(username string, bytes int64) error {
	return nil
}

// GetUsage 获取使用情况
func (p *NoOpQuotaProvider) GetUsage(username string) (int64, int64, error) {
	return 0, 0, nil
}
