package airecommend

import (
	"math"
	"sort"
	"time"
)

// AddUser 添加用户
func (e *Engine) AddUser(userID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.users[userID]; !exists {
		e.users[userID] = &UserProfile{
			UserID:      userID,
			Preferences: make(map[string]float64),
			LastActive:  time.Now(),
		}
	}
}

// AddFile 添加文件
func (e *Engine) AddFile(file *FileItem) {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.files[file.FileID] = file
}

// AddAccessRecord 添加访问记录
func (e *Engine) AddAccessRecord(record *AccessRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()

	record.Timestamp = time.Now()
	e.accessLog = append(e.accessLog, *record)

	// 更新用户画像
	if user, exists := e.users[record.UserID]; exists {
		user.AccessHistory = append(user.AccessHistory, *record)
		user.LastActive = time.Now()

		// 更新偏好
		if file, ok := e.files[record.FileID]; ok {
			if user.Preferences == nil {
				user.Preferences = make(map[string]float64)
			}
			user.Preferences[file.Type] += 0.1
		}
	}
}

// GetRecommendations 获取推荐
func (e *Engine) GetRecommendations(userID string, limit int) []Recommendation {
	e.mu.RLock()

	// 检查缓存
	if entry, exists := e.cache[userID]; exists && time.Now().Before(entry.ExpiresAt) {
		e.mu.RUnlock()
		if limit > 0 && limit < len(entry.Recommendations) {
			return entry.Recommendations[:limit]
		}
		return entry.Recommendations
	}
	e.mu.RUnlock()

	// 缓存未命中，计算推荐
	user, exists := e.users[userID]
	if !exists {
		return nil
	}

	recommendations := e.calculateRecommendations(user, limit)

	// 更新缓存
	e.mu.Lock()
	e.cache[userID] = &CacheEntry{
		Recommendations: recommendations,
		ExpiresAt:       time.Now().Add(e.config.CacheTTL),
	}
	e.mu.Unlock()

	return recommendations
}

// calculateRecommendations 计算推荐
func (e *Engine) calculateRecommendations(user *UserProfile, limit int) []Recommendation {
	if limit <= 0 {
		limit = e.config.MaxResults
	}

	// 获取用户已访问的文件
	accessedFiles := make(map[string]bool)
	for _, record := range user.AccessHistory {
		accessedFiles[record.FileID] = true
	}

	type scoredFile struct {
		fileID string
		score  float64
		reason string
	}

	var candidates []scoredFile

	e.mu.RLock()
	defer e.mu.RUnlock()

	for fileID, file := range e.files {
		// 跳过已访问的文件（可选：可以推荐相关文件）
		_ = accessedFiles[fileID]

		// 计算混合评分
		timeScore := e.calculateTimeScore(user, fileID)
		freqScore := e.calculateFrequencyScore(user, fileID)
		collabScore := e.calculateCollaborativeScore(user.UserID, fileID)
		contentScore := e.calculateContentScore(user, file)

		totalScore := timeScore*e.config.Weights.TimeDecay +
			freqScore*e.config.Weights.Frequency +
			collabScore*e.config.Weights.Collaborative +
			contentScore*e.config.Weights.Content

		if totalScore > 0.1 { // 最小阈值
			reason := generateReason(timeScore, freqScore, collabScore, contentScore)
			candidates = append(candidates, scoredFile{
				fileID: fileID,
				score:  totalScore,
				reason: reason,
			})
		}
	}

	// 按分数排序
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// 取 top N
	if limit > len(candidates) {
		limit = len(candidates)
	}

	result := make([]Recommendation, limit)
	now := time.Now()
	for i := 0; i < limit; i++ {
		candidate := candidates[i]
		file := e.files[candidate.fileID]
		result[i] = Recommendation{
			FileID:    candidate.fileID,
			Name:      file.Name,
			Path:      file.Path,
			Score:     candidate.score,
			Reason:    candidate.reason,
			UpdatedAt: now,
		}
	}

	return result
}

// calculateTimeScore 计算时间衰减分数
func (e *Engine) calculateTimeScore(user *UserProfile, fileID string) float64 {
	var lastAccess time.Time
	for i := len(user.AccessHistory) - 1; i >= 0; i-- {
		if user.AccessHistory[i].FileID == fileID {
			lastAccess = user.AccessHistory[i].Timestamp
			break
		}
	}

	if lastAccess.IsZero() {
		return 0
	}

	hoursAgo := time.Since(lastAccess).Hours()
	return math.Pow(e.config.DecayFactor, hoursAgo/24) // 按天衰减
}

// calculateFrequencyScore 计算频率分数
func (e *Engine) calculateFrequencyScore(user *UserProfile, fileID string) float64 {
	count := 0
	for _, record := range user.AccessHistory {
		if record.FileID == fileID {
			count++
		}
	}

	if count < e.config.MinAccesses {
		return float64(count) / float64(e.config.MinAccesses) * 0.5
	}

	return math.Min(float64(count)/10.0, 1.0) // 最高1.0
}

// calculateCollaborativeScore 计算协同过滤分数
func (e *Engine) calculateCollaborativeScore(userID, fileID string) float64 {
	// 找到访问过该文件的其他用户
	var similarUsers []string
	for _, record := range e.accessLog {
		if record.FileID == fileID && record.UserID != userID {
			similarUsers = append(similarUsers, record.UserID)
		}
	}

	if len(similarUsers) == 0 {
		return 0
	}

	// 计算相似用户访问该文件的频率
	accessCount := make(map[string]int)
	for _, uid := range similarUsers {
		accessCount[uid]++
	}

	// 计算相似度分数（简化版本：基于共同访问的文件数）
	maxScore := 0.0
	for _, uid := range similarUsers {
		similarity := e.calculateUserSimilarity(userID, uid)
		if similarity > maxScore {
			maxScore = similarity
		}
	}

	return maxScore
}

// calculateUserSimilarity 计算用户相似度
func (e *Engine) calculateUserSimilarity(user1, user2 string) float64 {
	u1, ok1 := e.users[user1]
	u2, ok2 := e.users[user2]
	if !ok1 || !ok2 {
		return 0
	}

	// 基于共同访问文件计算相似度
	common := 0
	files1 := make(map[string]bool)
	for _, r := range u1.AccessHistory {
		files1[r.FileID] = true
	}

	for _, r := range u2.AccessHistory {
		if files1[r.FileID] {
			common++
		}
	}

	total := len(u1.AccessHistory) + len(u2.AccessHistory) - common
	if total == 0 {
		return 0
	}

	return float64(common) / float64(total)
}

// calculateContentScore 计算内容相似度分数
func (e *Engine) calculateContentScore(user *UserProfile, file *FileItem) float64 {
	if len(user.Preferences) == 0 {
		return 0.5 // 默认中等分数
	}

	// 基于文件类型的偏好
	if pref, ok := user.Preferences[file.Type]; ok {
		return math.Min(pref, 1.0)
	}

	return 0.3 // 未知类型给较低分
}

// generateReason 生成推荐理由
func generateReason(timeScore, freqScore, collabScore, contentScore float64) string {
	reasons := []string{}

	if timeScore > 0.7 {
		reasons = append(reasons, "最近访问过")
	}
	if freqScore > 0.5 {
		reasons = append(reasons, "经常访问")
	}
	if collabScore > 0.5 {
		reasons = append(reasons, "相似用户也喜欢")
	}
	if contentScore > 0.7 {
		reasons = append(reasons, "符合你的偏好")
	}

	if len(reasons) == 0 {
		return "可能感兴趣"
	}

	result := reasons[0]
	for i := 1; i < len(reasons); i++ {
		result += "、" + reasons[i]
	}
	return result
}

// InvalidateCache 使缓存失效
func (e *Engine) InvalidateCache(userID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.cache, userID)
}

// InvalidateAllCache 使所有缓存失效
func (e *Engine) InvalidateAllCache() {
	e.mu.Lock()
	defer e.mu.Unlock()

	e.cache = make(map[string]*CacheEntry)
}

// GetUserProfile 获取用户画像
func (e *Engine) GetUserProfile(userID string) *UserProfile {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.users[userID]
}

// GetFile 获取文件信息
func (e *Engine) GetFile(fileID string) *FileItem {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return e.files[fileID]
}

// GetAccessLog 获取访问日志
func (e *Engine) GetAccessLog(userID string, limit int) []AccessRecord {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var records []AccessRecord
	for _, record := range e.accessLog {
		if record.UserID == userID {
			records = append(records, record)
		}
	}

	if limit > 0 && limit < len(records) {
		return records[len(records)-limit:]
	}
	return records
}
