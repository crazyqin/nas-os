// Package ransomshield - 威胁情报集成
// IOC 管理、多源情报聚合、哈希/IP/域名匹配、自动更新
package ransomware

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 威胁情报管理器
// ============================================================

// ThreatIntelligence 威胁情报管理器.
type ThreatIntelligence struct {
	mu sync.RWMutex

	// iocs 失陷指标库 (type -> []IOC)
	iocs map[IOCType][]IOC

	// signatures 勒索软件签名
	signatures map[string]*RansomwareSignature

	// feeds 情报源
	fees []IntelFeed

	// matchCache 匹配缓存
	matchCache map[string]*MatchResult

	// stats 统计
	stats IntelStats

	// dataDir 数据目录
	dataDir string

	// httpClient HTTP 客户端
	httpClient *http.Client

	// running 运行状态
	running  bool
	stopChan chan struct{}
}

// IOCType IOC 类型.
type IOCType string

const (
	IOCTypeHash   IOCType = "hash"   // 文件哈希
	IOCTypeIP     IOCType = "ip"     // IP 地址
	IOCTypeDomain IOCType = "domain" // 域名
	IOCTypeURL    IOCType = "url"    // URL
	IOCTypeEmail  IOCType = "email"  // 邮箱
	IOCTypeMutex  IOCType = "mutex"  // 互斥体
	IOCTypeRegKey IOCType = "regkey" // 注册表键
	IOCTypeYARA   IOCType = "yara"   // YARA 规则
)

// IOC 失陷指标.
type IOC struct {
	ID          string      `json:"id"`
	Type        IOCType     `json:"type"`
	Value       string      `json:"value"`
	ThreatLevel ThreatLevel `json:"threat_level"`
	Confidence  float64     `json:"confidence"` // 0-1
	Source      string      `json:"source"`
	Description string      `json:"description"`
	Tags        []string    `json:"tags"`
	FirstSeen   time.Time   `json:"first_seen"`
	LastSeen    time.Time   `json:"last_seen"`
	ExpiresAt   *time.Time  `json:"expires_at,omitempty"`
	Active      bool        `json:"active"`
}

// RansomwareSignature 勒索软件签名.
type RansomwareSignature struct {
	ID         string      `json:"id"`
	Family     string      `json:"family"` // 勒索软件家族
	Name       string      `json:"name"`
	Hashes     []string    `json:"hashes"`      // 已知哈希
	FileExts   []string    `json:"file_exts"`   // 加密后扩展名
	RansomNote []string    `json:"ransom_note"` // 勒索信特征
	Behaviors  []string    `json:"behaviors"`   // 行为特征
	IOCs       []string    `json:"iocs"`        // 关联IOC
	Severity   ThreatLevel `json:"severity"`
	FirstSeen  time.Time   `json:"first_seen"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

// IntelFeed 情报源.
type IntelFeed struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Type        string    `json:"type"` // otorio, alienvault, custom, file
	Enabled     bool      `json:"enabled"`
	IntervalMin int       `json:"interval_min"`
	APIKey      string    `json:"api_key,omitempty"`
	LastFetch   time.Time `json:"last_fetch"`
	LastStatus  string    `json:"last_status"`
	IOCCount    int       `json:"ioc_count"`
}

// MatchResult 匹配结果.
type MatchResult struct {
	Matched     bool                 `json:"matched"`
	IOC         *IOC                 `json:"ioc,omitempty"`
	Signature   *RansomwareSignature `json:"signature,omitempty"`
	Confidence  float64              `json:"confidence"`
	Description string               `json:"description"`
	MatchedAt   time.Time            `json:"matched_at"`
}

// IntelStats 情报统计.
type IntelStats struct {
	TotalIOCs      int       `json:"total_iocs"`
	ActiveIOCs     int       `json:"active_iocs"`
	Signatures     int       `json:"signatures"`
	FeedsActive    int       `json:"feeds_active"`
	MatchesTotal   int64     `json:"matches_total"`
	LastUpdateTime time.Time `json:"last_update_time"`
	TotalLookups   int64     `json:"total_lookups"`
	CacheHits      int64     `json:"cache_hits"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewThreatIntelligence 创建威胁情报管理器.
func NewThreatIntelligence(dataDir string) *ThreatIntelligence {
	ti := &ThreatIntelligence{
		iocs:       make(map[IOCType][]IOC),
		signatures: make(map[string]*RansomwareSignature),
		matchCache: make(map[string]*MatchResult),
		dataDir:    dataDir,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		stopChan:   make(chan struct{}),
	}

	ti.initBuiltinSignatures()
	ti.initDefaultFeeds()
	return ti
}

// initBuiltinSignatures 初始化内置勒索软件签名.
func (ti *ThreatIntelligence) initBuiltinSignatures() {
	ti.signatures = map[string]*RansomwareSignature{
		"wannacry": {
			ID: "wannacry", Family: "WannaCry", Name: "WannaCry 勒索软件",
			FileExts:   []string{".wncry", ".wncryt"},
			RansomNote: []string{"@Please_Read_Me@.txt", "WNcry@2Ol7"},
			Behaviors:  []string{"ms17-010", "eternalblue", "kill_switch"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2017, 5, 12, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"ryuk": {
			ID: "ryuk", Family: "Ryuk", Name: "Ryuk 勒索软件",
			FileExts:   []string{".ryk", ".ryuk"},
			RansomNote: []string{"RyukReadMe.txt", "UNIQUE_ID_DO_NOT_REMOVE.txt"},
			Behaviors:  []string{"shadow_copy_deletion", "disable_recovery", "network_spread"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2018, 8, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"conti": {
			ID: "conti", Family: "Conti", Name: "Conti 勒索软件",
			FileExts:   []string{".conti", ".CONTI"},
			RansomNote: []string{"CONTI_README.txt", "readme.txt"},
			Behaviors:  []string{"double_extortion", "data_exfil", "rapid_encrypt"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"lockbit": {
			ID: "lockbit", Family: "LockBit", Name: "LockBit 勒索软件",
			FileExts:   []string{".lockbit", ".abcd", ".lockbit3.0"},
			RansomNote: []string{"LockBit_Ransomware.hta", "Restore-My-Files.txt"},
			Behaviors:  []string{"self_spread", "disable_defender", "delete_backups"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2019, 9, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"revil": {
			ID: "revil", Family: "REvil/Sodinokibi", Name: "REvil 勒索软件",
			FileExts:   []string{".revil", ".sodinokibi", ".random_ext"},
			RansomNote: []string{"[ransomware_id]-readme.txt"},
			Behaviors:  []string{"double_extortion", "ransomware_as_service"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2019, 4, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"maze": {
			ID: "maze", Family: "Maze", Name: "Maze 勒索软件",
			FileExts:   []string{".maze"},
			RansomNote: []string{"DECRYPT-FILES.txt"},
			Behaviors:  []string{"data_exfil", "double_extortion", "vm_escape"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2019, 5, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
		"blackcat": {
			ID: "blackcat", Family: "BlackCat/ALPHV", Name: "BlackCat 勒索软件",
			FileExts:   []string{".alphv"},
			RansomNote: []string{"RECOVER-[id]-FILES.txt"},
			Behaviors:  []string{"rust_impl", "cross_platform", "triple_extortion"},
			Severity:   ThreatLevelCritical,
			FirstSeen:  time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC),
			UpdatedAt:  time.Now(),
		},
	}
}

// initDefaultFeeds 初始化默认情报源.
func (ti *ThreatIntelligence) initDefaultFeeds() {
	ti.fees = []IntelFeed{
		{
			ID: "builtin", Name: "内置规则库", URL: "",
			Type: "file", Enabled: true, IntervalMin: 0,
		},
		{
			ID: "alienvault-otx", Name: "AlienVault OTX",
			URL:  "https://otx.alienvault.com/api/v1/indicators/export",
			Type: "alienvault", Enabled: false, IntervalMin: 60,
		},
		{
			ID: "ransomwaretracker", Name: "Ransomware Tracker",
			URL:  "https://ransomwaretracker.abuse.ch/downloads/",
			Type: "abuse_ch", Enabled: false, IntervalMin: 30,
		},
	}
}

// Start 启动情报管理器.
func (ti *ThreatIntelligence) Start() {
	ti.mu.Lock()
	if ti.running {
		ti.mu.Unlock()
		return
	}
	ti.running = true
	ti.mu.Unlock()

	// 加载本地数据
	ti.loadLocalData()

	// 启动更新循环
	go ti.updateLoop()

	log.Println("[Intel] 威胁情报管理器已启动")
}

// Stop 停止情报管理器.
func (ti *ThreatIntelligence) Stop() {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	if !ti.running {
		return
	}
	close(ti.stopChan)
	ti.running = false
	log.Println("[Intel] 威胁情报管理器已停止")
}

// ============================================================
// IOC 管理
// ============================================================

// AddIOC 添加 IOC.
func (ti *ThreatIntelligence) AddIOC(ioc IOC) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if ioc.ID == "" {
		ioc.ID = uuid.New().String()
	}
	if ioc.FirstSeen.IsZero() {
		ioc.FirstSeen = time.Now()
	}
	ioc.LastSeen = time.Now()
	ioc.Active = true

	ti.iocs[ioc.Type] = append(ti.iocs[ioc.Type], ioc)
	ti.stats.TotalIOCs++
	ti.stats.ActiveIOCs++

	// 清除相关缓存
	delete(ti.matchCache, ioc.Value)
}

// RemoveIOC 移除 IOC.
func (ti *ThreatIntelligence) RemoveIOC(id string) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	for iocType, iocs := range ti.iocs {
		for i, ioc := range iocs {
			if ioc.ID == id {
				ti.iocs[iocType] = append(iocs[:i], iocs[i+1:]...)
				ti.stats.TotalIOCs--
				if ioc.Active {
					ti.stats.ActiveIOCs--
				}
				return
			}
		}
	}
}

// AddSignature 添加勒索软件签名.
func (ti *ThreatIntelligence) AddSignature(sig RansomwareSignature) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if sig.ID == "" {
		sig.ID = uuid.New().String()
	}
	sig.UpdatedAt = time.Now()
	ti.signatures[sig.ID] = &sig
	ti.stats.Signatures = len(ti.signatures)
}

// GetIOCs 获取指定类型的 IOC.
func (ti *ThreatIntelligence) GetIOCs(iocType IOCType) []IOC {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	iocs := ti.iocs[iocType]
	result := make([]IOC, len(iocs))
	copy(result, iocs)
	return result
}

// GetSignatures 获取所有签名.
func (ti *ThreatIntelligence) GetSignatures() []RansomwareSignature {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	result := make([]RansomwareSignature, 0, len(ti.signatures))
	for _, sig := range ti.signatures {
		result = append(result, *sig)
	}
	return result
}

// ============================================================
// 匹配查询
// ============================================================

// MatchHash 匹配文件哈希.
func (ti *ThreatIntelligence) MatchHash(hash string) *MatchResult {
	ti.mu.RLock()
	if cached, ok := ti.matchCache[hash]; ok {
		ti.stats.CacheHits++
		ti.mu.RUnlock()
		return cached
	}
	ti.mu.RUnlock()

	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.stats.TotalLookups++

	hash = strings.ToLower(hash)

	// 检查 IOC 哈希
	for _, ioc := range ti.iocs[IOCTypeHash] {
		if !ioc.Active {
			continue
		}
		if strings.ToLower(ioc.Value) == hash {
			result := &MatchResult{
				Matched:     true,
				IOC:         &ioc,
				Confidence:  ioc.Confidence,
				Description: fmt.Sprintf("哈希命中IOC: %s", ioc.Description),
				MatchedAt:   time.Now(),
			}
			ti.matchCache[hash] = result
			ti.stats.MatchesTotal++
			return result
		}
	}

	// 检查勒索软件签名哈希
	for _, sig := range ti.signatures {
		for _, sigHash := range sig.Hashes {
			if strings.ToLower(sigHash) == hash {
				result := &MatchResult{
					Matched:     true,
					Signature:   sig,
					Confidence:  0.99,
					Description: fmt.Sprintf("匹配勒索软件签名: %s (%s)", sig.Name, sig.Family),
					MatchedAt:   time.Now(),
				}
				ti.matchCache[hash] = result
				ti.stats.MatchesTotal++
				return result
			}
		}
	}

	// 未匹配
	result := &MatchResult{Matched: false, MatchedAt: time.Now()}
	ti.matchCache[hash] = result
	return result
}

// MatchExtension 匹配文件扩展名.
func (ti *ThreatIntelligence) MatchExtension(ext string) *MatchResult {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.stats.TotalLookups++
	ext = strings.ToLower(ext)

	for _, sig := range ti.signatures {
		for _, sigExt := range sig.FileExts {
			if strings.ToLower(sigExt) == ext {
				result := &MatchResult{
					Matched:     true,
					Signature:   sig,
					Confidence:  0.85,
					Description: fmt.Sprintf("扩展名匹配勒索软件: %s (%s)", sig.Name, sig.Family),
					MatchedAt:   time.Now(),
				}
				ti.stats.MatchesTotal++
				return result
			}
		}
	}

	return &MatchResult{Matched: false, MatchedAt: time.Now()}
}

// MatchRansomNote 匹配勒索信.
func (ti *ThreatIntelligence) MatchRansomNote(filename string) *MatchResult {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.stats.TotalLookups++
	filename = strings.ToLower(filename)

	var best *MatchResult
	for _, sig := range ti.signatures {
		for _, note := range sig.RansomNote {
			noteLower := strings.ToLower(note)
			if strings.Contains(filename, noteLower) {
				confidence := 0.95
				if filename == noteLower {
					confidence = 1.0
				}
				result := &MatchResult{
					Matched:     true,
					Signature:   sig,
					Confidence:  confidence,
					Description: fmt.Sprintf("勒索信匹配: %s (%s)", sig.Name, sig.Family),
					MatchedAt:   time.Now(),
				}
				if best == nil || result.Confidence > best.Confidence || len(noteLower) > len(best.Signature.RansomNote[0]) {
					best = result
				}
			}
		}
	}
	if best != nil {
		ti.stats.MatchesTotal++
		return best
	}

	return &MatchResult{Matched: false, MatchedAt: time.Now()}
}

// MatchIP 匹配 IP 地址.
func (ti *ThreatIntelligence) MatchIP(ip string) *MatchResult {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.stats.TotalLookups++

	for _, ioc := range ti.iocs[IOCTypeIP] {
		if !ioc.Active {
			continue
		}
		if ioc.Value == ip {
			result := &MatchResult{
				Matched:     true,
				IOC:         &ioc,
				Confidence:  ioc.Confidence,
				Description: fmt.Sprintf("IP命中IOC: %s", ioc.Description),
				MatchedAt:   time.Now(),
			}
			ti.stats.MatchesTotal++
			return result
		}
	}

	return &MatchResult{Matched: false, MatchedAt: time.Now()}
}

// MatchDomain 匹配域名.
func (ti *ThreatIntelligence) MatchDomain(domain string) *MatchResult {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	ti.stats.TotalLookups++
	domain = strings.ToLower(domain)

	for _, ioc := range ti.iocs[IOCTypeDomain] {
		if !ioc.Active {
			continue
		}
		if strings.ToLower(ioc.Value) == domain || strings.HasSuffix(domain, "."+strings.ToLower(ioc.Value)) {
			result := &MatchResult{
				Matched:     true,
				IOC:         &ioc,
				Confidence:  ioc.Confidence,
				Description: fmt.Sprintf("域名命中IOC: %s", ioc.Description),
				MatchedAt:   time.Now(),
			}
			ti.stats.MatchesTotal++
			return result
		}
	}

	return &MatchResult{Matched: false, MatchedAt: time.Now()}
}

// ComputeFileHash 计算文件 SHA256.
func ComputeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// ============================================================
// 情报更新
// ============================================================

// updateLoop 情报更新循环.
func (ti *ThreatIntelligence) updateLoop() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ti.stopChan:
			return
		case <-ticker.C:
			ti.updateFeeds()
		}
	}
}

// updateFeeds 更新所有情报源.
func (ti *ThreatIntelligence) updateFeeds() {
	ti.mu.RLock()
	var feeds []IntelFeed
	for _, feed := range ti.fees {
		if feed.Enabled {
			feeds = append(feeds, feed)
		}
	}
	ti.mu.RUnlock()

	for _, feed := range feeds {
		if !feed.needsUpdate() {
			continue
		}

		if err := ti.fetchFeed(feed); err != nil {
			log.Printf("[Intel] 更新情报源失败: %s, %v", feed.Name, err)
		}
	}
}

// needsUpdate 检查是否需要更新.
func (f IntelFeed) needsUpdate() bool {
	if f.IntervalMin <= 0 {
		return false
	}
	return time.Since(f.LastFetch) > time.Duration(f.IntervalMin)*time.Minute
}

// fetchFeed 获取情报源数据.
func (ti *ThreatIntelligence) fetchFeed(feed IntelFeed) error {
	if feed.URL == "" {
		return nil
	}

	log.Printf("[Intel] 正在更新情报源: %s", feed.Name)

	resp, err := ti.httpClient.Get(feed.URL)
	if err != nil {
		return fmt.Errorf("HTTP 请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP 状态码: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取响应失败: %w", err)
	}

	// 根据来源类型解析
	count := ti.parseFeedData(feed.Type, body)

	ti.mu.Lock()
	for i := range ti.fees {
		if ti.fees[i].ID == feed.ID {
			ti.fees[i].LastFetch = time.Now()
			ti.fees[i].LastStatus = "ok"
			ti.fees[i].IOCCount += count
			break
		}
	}
	ti.stats.LastUpdateTime = time.Now()
	ti.mu.Unlock()

	log.Printf("[Intel] 情报源更新完成: %s, 新增 %d 条IOC", feed.Name, count)
	return nil
}

// parseFeedData 解析情报源数据.
func (ti *ThreatIntelligence) parseFeedData(feedType string, data []byte) int {
	switch feedType {
	case "abuse_ch":
		return ti.parseAbuseCh(data)
	default:
		// 尝试 JSON 解析
		return ti.parseGenericJSON(data)
	}
}

// parseAbuseCh 解析 abuse.ch 格式.
func (ti *ThreatIntelligence) parseAbuseCh(data []byte) int {
	lines := strings.Split(string(data), "\n")
	count := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.Split(line, ",")
		if len(parts) >= 2 {
			ioc := IOC{
				Type:        IOCTypeHash,
				Value:       strings.TrimSpace(parts[1]),
				ThreatLevel: ThreatLevelHigh,
				Confidence:  0.8,
				Source:      "abuse.ch",
				Tags:        []string{"ransomware", "malware"},
				Active:      true,
			}
			ti.AddIOC(ioc)
			count++
		}
	}
	return count
}

// parseGenericJSON 解析通用 JSON 格式.
func (ti *ThreatIntelligence) parseGenericJSON(data []byte) int {
	var iocs []IOC
	if err := json.Unmarshal(data, &iocs); err != nil {
		return 0
	}

	count := 0
	for _, ioc := range iocs {
		if ioc.Value != "" {
			ti.AddIOC(ioc)
			count++
		}
	}
	return count
}

// ============================================================
// 本地数据持久化
// ============================================================

// loadLocalData 加载本地数据.
func (ti *ThreatIntelligence) loadLocalData() {
	if ti.dataDir == "" {
		return
	}

	iocsFile := filepath.Join(ti.dataDir, "iocs.json")
	data, err := os.ReadFile(iocsFile)
	if err == nil {
		var iocs []IOC
		if err := json.Unmarshal(data, &iocs); err == nil {
			for _, ioc := range iocs {
				ti.AddIOC(ioc)
			}
		}
	}
}

// SaveLocalData 保存本地数据.
func (ti *ThreatIntelligence) SaveLocalData() error {
	if ti.dataDir == "" {
		return fmt.Errorf("数据目录未设置")
	}

	os.MkdirAll(ti.dataDir, 0755)

	ti.mu.RLock()
	var allIOCs []IOC
	for _, iocs := range ti.iocs {
		allIOCs = append(allIOCs, iocs...)
	}
	ti.mu.RUnlock()

	data, err := json.MarshalIndent(allIOCs, "", "  ")
	if err != nil {
		return err
	}

	iocsFile := filepath.Join(ti.dataDir, "iocs.json")
	return os.WriteFile(iocsFile, data, 0644)
}

// ============================================================
// 查询接口
// ============================================================

// GetStats 获取统计信息.
func (ti *ThreatIntelligence) GetStats() IntelStats {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	stats := ti.stats
	stats.Signatures = len(ti.signatures)

	activeFeeds := 0
	for _, feed := range ti.fees {
		if feed.Enabled {
			activeFeeds++
		}
	}
	stats.FeedsActive = activeFeeds

	activeIOCs := 0
	for _, iocs := range ti.iocs {
		for _, ioc := range iocs {
			if ioc.Active {
				activeIOCs++
			}
		}
	}
	stats.ActiveIOCs = activeIOCs
	stats.TotalIOCs = len(allIOCs(ti.iocs))

	return stats
}

// GetFeeds 获取所有情报源.
func (ti *ThreatIntelligence) GetFeeds() []IntelFeed {
	ti.mu.RLock()
	defer ti.mu.RUnlock()

	result := make([]IntelFeed, len(ti.fees))
	copy(result, ti.fees)
	return result
}

// AddFeed 添加情报源.
func (ti *ThreatIntelligence) AddFeed(feed IntelFeed) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	if feed.ID == "" {
		feed.ID = uuid.New().String()
	}
	ti.fees = append(ti.fees, feed)
}

// ClearCache 清除匹配缓存.
func (ti *ThreatIntelligence) ClearCache() {
	ti.mu.Lock()
	defer ti.mu.Unlock()
	ti.matchCache = make(map[string]*MatchResult)
}

// allIOCs 获取所有 IOC 列表.
func allIOCs(iocs map[IOCType][]IOC) []IOC {
	var result []IOC
	for _, list := range iocs {
		result = append(result, list...)
	}
	return result
}

// containsAny 检查字符串是否包含任一子串.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
