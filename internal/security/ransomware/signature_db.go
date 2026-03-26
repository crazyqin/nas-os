package ransomware

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SignatureDB 特征库管理器.
type SignatureDB struct {
	signatures  map[string]*RansomwareSignature
	extensions  map[string][]string // extension -> signature IDs
	patterns    map[string][]string // pattern -> signature IDs
	ransomNotes map[string][]string // ransom note name -> signature IDs
	mu          sync.RWMutex
	config      SignatureDBConfig
	lastUpdate  time.Time
}

// NewSignatureDB 创建特征库.
func NewSignatureDB(config SignatureDBConfig) *SignatureDB {
	db := &SignatureDB{
		signatures:  make(map[string]*RansomwareSignature),
		extensions:  make(map[string][]string),
		patterns:    make(map[string][]string),
		ransomNotes: make(map[string][]string),
		config:      config,
	}

	// 加载内置特征库
	db.loadBuiltinSignatures()

	return db
}

// loadBuiltinSignatures 加载内置特征库.
func (db *SignatureDB) loadBuiltinSignatures() {
	builtin := []RansomwareSignature{
		// ========== 常见勒索软件家族 ==========
		{
			ID:          "wannacry",
			Name:        "WannaCry",
			Family:      "WannaCry",
			Extensions:  []string{".wncry", ".wnry", ".wncryt", ".wncryry"},
			RansomNote:  []string{"!WannaDecryptor!.exe", "!WannaDecryptor!.bmp", "@WanaDecryptor@.exe", "@Please_Read_Me@.txt"},
			FirstSeen:   "2017-05-12",
			LastUpdated: "2024-01-15",
			Severity:    ThreatLevelCritical,
			Description: "WannaCry勒索软件，利用EternalBlue漏洞传播",
			Reference:   "https://www.malwarebytes.com/ransomware/wannacry",
		},
		{
			ID:          "lockbit",
			Name:        "LockBit",
			Family:      "LockBit",
			Extensions:  []string{".lockbit", ".abcd"},
			RansomNote:  []string{"Restore-My-Files.txt", "Abandoned-Files.txt"},
			FirstSeen:   "2019-09",
			LastUpdated: "2024-06-01",
			Severity:    ThreatLevelCritical,
			Description: "LockBit勒索软件即服务(RaaS)，高度活跃",
			Reference:   "https://www.cisa.gov/stopransomware/lockbit",
		},
		{
			ID:          "blackcat",
			Name:        "BlackCat/ALPHV",
			Family:      "BlackCat",
			Extensions:  []string{".blackcat", ".alphv"},
			RansomNote:  []string{"RECOVER-FILES.txt", "recovery.txt"},
			FirstSeen:   "2021-11",
			LastUpdated: "2024-05-20",
			Severity:    ThreatLevelCritical,
			Description: "BlackCat(ALPHV)勒索软件，使用Rust编写",
			Reference:   "https://www.cisa.gov/news-events/cybersecurity-advisories/aa22-321a",
		},
		{
			ID:          "ryuk",
			Name:        "Ryuk",
			Family:      "Ryuk",
			Extensions:  []string{".ryk", ".RYK"},
			RansomNote:  []string{"RyukReadMe.txt", "RyukReadMe.html"},
			FirstSeen:   "2018-08",
			LastUpdated: "2024-01-10",
			Severity:    ThreatLevelCritical,
			Description: "Ryuk勒索软件，针对企业进行定向攻击",
			Reference:   "https://www.crowdstrike.com/adversaries/trickbot/",
		},
		{
			ID:          "conti",
			Name:        "Conti",
			Family:      "Conti",
			Extensions:  []string{".conti", ".encrypted"},
			RansomNote:  []string{"CONTI_README.txt", "readme.txt"},
			FirstSeen:   "2020-07",
			LastUpdated: "2024-02-15",
			Severity:    ThreatLevelCritical,
			Description: "Conti勒索软件，与Wizard Spider相关联",
			Reference:   "https://www.cisa.gov/stopransomware/conti-ransomware",
		},
		{
			ID:          "babuk",
			Name:        "Babuk",
			Family:      "Babuk",
			Extensions:  []string{".babyk", ".babuk"},
			RansomNote:  []string{"How To Restore Your Files.txt", "how_to_decrypt.txt"},
			FirstSeen:   "2021-01",
			LastUpdated: "2024-03-01",
			Severity:    ThreatLevelHigh,
			Description: "Babuk勒索软件，针对企业网络",
			Reference:   "https://www.trendmicro.com/en_us/research/21/b/babuk-ransomware.html",
		},
		{
			ID:          "dharma",
			Name:        "Dharma/CrySiS",
			Family:      "Dharma",
			Extensions:  []string{".dharma", ".cezar", ".bip", ".[random].dharma"},
			RansomNote:  []string{"FILES ENCRYPTED.txt", "info.txt"},
			FirstSeen:   "2016-11",
			LastUpdated: "2024-01-20",
			Severity:    ThreatLevelHigh,
			Description: "Dharma勒索软件家族，通过RDP传播",
			Reference:   "https://www.bleepingcomputer.com/news/security/dharma-ransomware/",
		},
		{
			ID:          "sodinokibi",
			Name:        "Sodinokibi/REvil",
			Family:      "Sodinokibi",
			Extensions:  []string{".sodinokibi", ".sodin", ".random5char"},
			RansomNote:  []string{"!!!ReadMe!!!.txt", "!!!ALL YOUR FILES ARE ENCRYPTED!!!.txt"},
			FirstSeen:   "2019-04",
			LastUpdated: "2024-01-05",
			Severity:    ThreatLevelCritical,
			Description: "Sodinokibi(REvil)勒索软件即服务",
			Reference:   "https://www.cisa.gov/stopransomware/sodinokibi-ransomware",
		},
		{
			ID:          "maze",
			Name:        "Maze",
			Family:      "Maze",
			Extensions:  []string{".maze", ".chacha"},
			RansomNote:  []string{"DECRYPT-FILES.html", "maze.txt"},
			FirstSeen:   "2019-05",
			LastUpdated: "2024-01-15",
			Severity:    ThreatLevelCritical,
			Description: "Maze勒索软件，首次引入双重勒索策略",
			Reference:   "https://www.mcafee.com/blogs/other-blogs/mcafee-labs/ransom-maze/",
		},
		{
			ID:          "phobos",
			Name:        "Phobos",
			Family:      "Phobos",
			Extensions:  []string{".phobos", ".adage", ".alcatraz", ".actin", ".banhu", ".calix", ".cesar"},
			RansomNote:  []string{"info.txt", "decrypt_info.txt", "FILES ENCRYPTED.txt"},
			FirstSeen:   "2018-12",
			LastUpdated: "2024-04-01",
			Severity:    ThreatLevelHigh,
			Description: "Phobos勒索软件家族，变种众多",
			Reference:   "https://www.trendmicro.com/en_ph/research/19/i/phobos-ransomware.html",
		},
		{
			ID:          "ekans",
			Name:        "EKANS/Snake",
			Family:      "EKANS",
			Extensions:  []string{".fivechars", ".snake"},
			RansomNote:  []string{"Decrypt-My-Files.txt"},
			FirstSeen:   "2019-12",
			LastUpdated: "2024-02-01",
			Severity:    ThreatLevelHigh,
			Description: "EKANS勒索软件，针对工业控制系统",
			Reference:   "https://www.dragos.com/blog/industry-news/ekans-ransomware-attacking-industrial-environments/",
		},
		{
			ID:          "darkside",
			Name:        "DarkSide",
			Family:      "DarkSide",
			Extensions:  []string{".darkside", "[random].darkside"},
			RansomNote:  []string{"README.[random].txt", "SOFTWARE-PROTECT-US.txt"},
			FirstSeen:   "2020-08",
			LastUpdated: "2024-01-10",
			Severity:    ThreatLevelCritical,
			Description: "DarkSide勒索软件即服务，曾攻击Colonial Pipeline",
			Reference:   "https://www.cisa.gov/stopransomware/darkside-ransomware",
		},
		{
			ID:          "avaddon",
			Name:        "Avaddon",
			Family:      "Avaddon",
			Extensions:  []string{".avdn", ".avaddon"},
			RansomNote:  []string{"!!!readme!!!.txt", "readme.txt"},
			FirstSeen:   "2020-06",
			LastUpdated: "2024-01-20",
			Severity:    ThreatLevelHigh,
			Description: "Avaddon勒索软件，通过垃圾邮件传播",
			Reference:   "https://www.trendmicro.com/en_us/research/21/a/avaddon-ransomware.html",
		},
		{
			ID:          "hive",
			Name:        "Hive",
			Family:      "Hive",
			Extensions:  []string{".hive", ".hived", ".key"},
			RansomNote:  []string{"HOW_TO_DECRYPT.txt", "README.txt"},
			FirstSeen:   "2021-06",
			LastUpdated: "2024-03-15",
			Severity:    ThreatLevelCritical,
			Description: "Hive勒索软件，医疗行业主要目标",
			Reference:   "https://www.cisa.gov/stopransomware/hive-ransomware",
		},
		{
			ID:          "yanluowang",
			Name:        "Yanluowang",
			Family:      "Yanluowang",
			Extensions:  []string{".yanluowang"},
			RansomNote:  []string{"yanluowang.txt", "README.txt"},
			FirstSeen:   "2021-08",
			LastUpdated: "2024-02-10",
			Severity:    ThreatLevelHigh,
			Description: "Yanluowang勒索软件，中文命名",
			Reference:   "https://www.trendmicro.com/en_us/research/21/i/ygn-ransomware-group.html",
		},
		{
			ID:          "lockfile",
			Name:        "LockFile",
			Family:      "LockFile",
			Extensions:  []string{".lockfile"},
			RansomNote:  []string{"!README_LOCKFILE!.txt"},
			FirstSeen:   "2021-07",
			LastUpdated: "2024-01-25",
			Severity:    ThreatLevelHigh,
			Description: "LockFile勒索软件，利用ProxyShell漏洞",
			Reference:   "https://www.bleepingcomputer.com/news/security/lockfile-ransomware-uses-intermittent-encryption/",
		},
		{
			ID:          "quantum",
			Name:        "Quantum",
			Family:      "Quantum",
			Extensions:  []string{".quantum", ".no_more_ransom"},
			RansomNote:  []string{"!README_QUANTUM!.txt", "README_DECRYPT_FILES.txt"},
			FirstSeen:   "2021-08",
			LastUpdated: "2024-04-10",
			Severity:    ThreatLevelCritical,
			Description: "Quantum勒索软件，与Conti相关",
			Reference:   "https://www.trendmicro.com/en_us/research/22/a/quantum-ransomware.html",
		},
		{
			ID:          "astro",
			Name:        "Astro",
			Family:      "Astro",
			Extensions:  []string{".astro"},
			RansomNote:  []string{"FILES_ENCRYPTED.txt"},
			FirstSeen:   "2022-04",
			LastUpdated: "2024-03-01",
			Severity:    ThreatLevelHigh,
			Description: "Astro勒索软件",
			Reference:   "https://www.malwarebytes.com/blog/threats/astro-ransomware",
		},
		{
			ID:          "noescape",
			Name:        "NoEscape",
			Family:      "NoEscape",
			Extensions:  []string{".noescepe", ".noescape"},
			RansomNote:  []string{"RECOVER_FILES.txt", "how_to_restore_files.txt"},
			FirstSeen:   "2023-05",
			LastUpdated: "2024-05-01",
			Severity:    ThreatLevelHigh,
			Description: "NoEscape勒索软件，模仿Babuk",
			Reference:   "https://www.trendmicro.com/en_us/research/23/d/noescape-ransomware.html",
		},
		{
			ID:          "incransom",
			Name:        "IncRansom",
			Family:      "IncRansom",
			Extensions:  []string{".incransom"},
			RansomNote:  []string{"README.inc", "FILES.txt"},
			FirstSeen:   "2023-07",
			LastUpdated: "2024-06-01",
			Severity:    ThreatLevelCritical,
			Description: "IncRansom勒索软件，双重勒索",
			Reference:   "https://www.trendmicro.com/en_us/research/23/j/incransom-ransomware.html",
		},
		// ========== 通用勒索软件特征 ==========
		{
			ID:          "generic-crypt",
			Name:        "Generic Crypto Ransomware",
			Family:      "Generic",
			Extensions:  []string{".encrypted", ".crypto", ".crypted", ".lock", ".locked", ".enc", ".cry", ".ransom"},
			FirstSeen:   "2010-01-01",
			LastUpdated: "2024-01-01",
			Severity:    ThreatLevelMedium,
			Description: "通用加密勒索软件特征",
			Reference:   "https://www.nomoreransom.org/",
		},
		{
			ID:          "generic-ransom-note",
			Name:        "Generic Ransom Note Pattern",
			Family:      "Generic",
			RansomNote:  []string{"readme.txt", "decrypt_instructions.txt", "how_to_decrypt.txt", "restore_files.txt", "!!!readme!!!.txt", "ransom_note.txt", "decrypt_files.txt", "!!!_readme_!!!.txt"},
			FirstSeen:   "2010-01-01",
			LastUpdated: "2024-01-01",
			Severity:    ThreatLevelMedium,
			Description: "通用勒索信文件名特征",
			Reference:   "https://www.nomoreransom.org/",
		},
	}

	for _, sig := range builtin {
		sig := sig // 创建副本
		db.signatures[sig.ID] = &sig

		// 建立扩展名索引
		for _, ext := range sig.Extensions {
			ext = strings.ToLower(ext)
			db.extensions[ext] = append(db.extensions[ext], sig.ID)
		}

		// 建立勒索信索引
		for _, note := range sig.RansomNote {
			note = strings.ToLower(note)
			db.ransomNotes[note] = append(db.ransomNotes[note], sig.ID)
		}
	}
}

// GetSignature 获取指定特征.
func (db *SignatureDB) GetSignature(id string) (*RansomwareSignature, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	sig, ok := db.signatures[id]
	return sig, ok
}

// GetSignaturesByExtension 通过扩展名查找特征.
func (db *SignatureDB) GetSignaturesByExtension(ext string) []*RansomwareSignature {
	db.mu.RLock()
	defer db.mu.RUnlock()

	ext = strings.ToLower(ext)
	ids, ok := db.extensions[ext]
	if !ok {
		return nil
	}

	var sigs []*RansomwareSignature
	for _, id := range ids {
		if sig, ok := db.signatures[id]; ok {
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// GetSignaturesByRansomNote 通过勒索信文件名查找特征.
func (db *SignatureDB) GetSignaturesByRansomNote(filename string) []*RansomwareSignature {
	db.mu.RLock()
	defer db.mu.RUnlock()

	filename = strings.ToLower(filename)
	ids, ok := db.ransomNotes[filename]
	if !ok {
		return nil
	}

	var sigs []*RansomwareSignature
	for _, id := range ids {
		if sig, ok := db.signatures[id]; ok {
			sigs = append(sigs, sig)
		}
	}
	return sigs
}

// GetAllSignatures 获取所有特征.
func (db *SignatureDB) GetAllSignatures() []*RansomwareSignature {
	db.mu.RLock()
	defer db.mu.RUnlock()

	sigs := make([]*RansomwareSignature, 0, len(db.signatures))
	for _, sig := range db.signatures {
		sigs = append(sigs, sig)
	}
	return sigs
}

// GetExtensions 获取所有已知的勒索软件扩展名.
func (db *SignatureDB) GetExtensions() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var exts []string
	for ext := range db.extensions {
		exts = append(exts, ext)
	}
	return exts
}

// GetRansomNotes 获取所有已知的勒索信文件名.
func (db *SignatureDB) GetRansomNotes() []string {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var notes []string
	for note := range db.ransomNotes {
		notes = append(notes, note)
	}
	return notes
}

// MatchExtension 匹配文件扩展名.
func (db *SignatureDB) MatchExtension(filename string) []*RansomwareSignature {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return nil
	}
	return db.GetSignaturesByExtension(ext)
}

// MatchRansomNote 匹配勒索信文件名.
func (db *SignatureDB) MatchRansomNote(filename string) []*RansomwareSignature {
	name := strings.ToLower(filepath.Base(filename))
	return db.GetSignaturesByRansomNote(name)
}

// AddSignature 添加自定义特征.
func (db *SignatureDB) AddSignature(sig RansomwareSignature) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	db.signatures[sig.ID] = &sig

	// 更新索引
	for _, ext := range sig.Extensions {
		ext = strings.ToLower(ext)
		db.extensions[ext] = append(db.extensions[ext], sig.ID)
	}

	for _, note := range sig.RansomNote {
		note = strings.ToLower(note)
		db.ransomNotes[note] = append(db.ransomNotes[note], sig.ID)
	}

	return nil
}

// RemoveSignature 移除特征.
func (db *SignatureDB) RemoveSignature(id string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	sig, ok := db.signatures[id]
	if !ok {
		return nil
	}

	// 从索引中移除
	for _, ext := range sig.Extensions {
		ext = strings.ToLower(ext)
		ids := db.extensions[ext]
		for i, sigID := range ids {
			if sigID == id {
				db.extensions[ext] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	for _, note := range sig.RansomNote {
		note = strings.ToLower(note)
		ids := db.ransomNotes[note]
		for i, sigID := range ids {
			if sigID == id {
				db.ransomNotes[note] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}

	delete(db.signatures, id)
	return nil
}

// LoadFromFile 从文件加载特征库.
func (db *SignatureDB) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var signatures []RansomwareSignature
	if err := json.Unmarshal(data, &signatures); err != nil {
		return err
	}

	for _, sig := range signatures {
		if err := db.AddSignature(sig); err != nil {
			// 记录添加失败的签名，但继续处理其他签名
			continue
		}
	}

	db.lastUpdate = time.Now()
	return nil
}

// SaveToFile 保存特征库到文件.
func (db *SignatureDB) SaveToFile(path string) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var signatures []RansomwareSignature
	for _, sig := range db.signatures {
		signatures = append(signatures, *sig)
	}

	data, err := json.MarshalIndent(signatures, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// GetStats 获取特征库统计信息.
func (db *SignatureDB) GetStats() map[string]interface{} {
	db.mu.RLock()
	defer db.mu.RUnlock()

	familyCount := make(map[string]int)
	for _, sig := range db.signatures {
		familyCount[sig.Family]++
	}

	return map[string]interface{}{
		"total_signatures":   len(db.signatures),
		"total_extensions":   len(db.extensions),
		"total_ransom_notes": len(db.ransomNotes),
		"families":           familyCount,
		"last_update":        db.lastUpdate,
	}
}
