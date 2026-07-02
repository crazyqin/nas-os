// Package resmetering 资源计量服务实现
package resmetering

// 本文件包含 Service 的方法实现
// 类型定义在 types.go 中

// RecordSample 批量记录采样数据的便捷方法.
func (s *Service) RecordSample(samples []Sample) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.samples = append(s.samples, samples...)

	// 超出上限时截断
	if len(s.samples) > s.maxSamples {
		s.samples = s.samples[len(s.samples)-s.maxSamples:]
	}
}

// GetSampleCount 返回当前采样总数.
func (s *Service) GetSampleCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.samples)
}

// Clear 清空所有采样数据.
func (s *Service) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = s.samples[:0]
}
