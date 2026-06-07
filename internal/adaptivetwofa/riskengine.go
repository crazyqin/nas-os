package adaptivetwofa

import (
	"sync"
	"time"
)

// RiskEngine 风险评估引擎
type RiskEngine struct {
	mu           sync.RWMutex
	config       *AdaptiveConfig
	userStats    map[string]*UserLoginStats
	loginHistory map[string][]*LoginHistory // userID -> 历史记录
}

// NewRiskEngine 创建风险评估引擎
func NewRiskEngine(config *AdaptiveConfig) *RiskEngine {
	return &RiskEngine{
		config:       config,
		userStats:    make(map[string]*UserLoginStats),
		loginHistory: make(map[string][]*LoginHistory),
	}
}

// EvaluateRisk 评估登录风险
func (re *RiskEngine) EvaluateRisk(ctx *LoginContext, trustedDevice *TrustedDevice) *RiskScore {
	re.mu.RLock()
	defer re.mu.RUnlock()

	factors := make([]RiskFactor, 0)
	totalScore := 0.0

	// 获取用户统计信息
	stats := re.userStats[ctx.UserID]

	// 1. 检查是否是新IP
	ipFactor := re.evaluateIPRisk(ctx, stats)
	factors = append(factors, ipFactor)
	totalScore += float64(ipFactor.Score) * ipFactor.Weight

	// 2. 检查是否是新设备
	deviceFactor := re.evaluateDeviceRisk(ctx, stats, trustedDevice)
	factors = append(factors, deviceFactor)
	totalScore += float64(deviceFactor.Score) * deviceFactor.Weight

	// 3. 检查是否是异常时间
	timeFactor := re.evaluateTimeRisk(ctx, stats)
	factors = append(factors, timeFactor)
	totalScore += float64(timeFactor.Score) * timeFactor.Weight

	// 4. 检查地理位置变化
	geoFactor := re.evaluateGeoRisk(ctx, stats)
	factors = append(factors, geoFactor)
	totalScore += float64(geoFactor.Score) * geoFactor.Weight

	// 5. 检查短时间多次登录
	rapidFactor := re.evaluateRapidLoginRisk(ctx, stats)
	factors = append(factors, rapidFactor)
	totalScore += float64(rapidFactor.Score) * rapidFactor.Weight

	// 计算最终得分 (归一化到0-100)
	score := int(totalScore)
	if score > 100 {
		score = 100
	}
	if score < 0 {
		score = 0
	}

	// 确定风险等级
	level := re.getRiskLevel(score)

	return &RiskScore{
		Score:       score,
		Level:       level,
		Factors:     factors,
		EvaluatedAt: time.Now(),
	}
}

// evaluateIPRisk 评估IP风险
func (re *RiskEngine) evaluateIPRisk(ctx *LoginContext, stats *UserLoginStats) RiskFactor {
	factor := RiskFactor{
		Name:   "ip_risk",
		Weight: re.config.NewIPWeight,
		Score:  0,
	}

	if stats == nil || len(stats.LastIPs) == 0 {
		// 新用户或无历史记录，给予中等风险
		factor.Score = 50
		factor.Description = "无历史IP记录"
		return factor
	}

	// 检查是否是已知IP
	for _, ip := range stats.LastIPs {
		if ip == ctx.IP {
			factor.Score = 0
			factor.Description = "已知IP地址"
			return factor
		}
	}

	// 新IP，根据历史IP数量调整风险
	switch {
	case len(stats.LastIPs) <= 1:
		factor.Score = 70
		factor.Description = "用户通常只使用1个IP，这是新IP"
	case len(stats.LastIPs) <= 3:
		factor.Score = 50
		factor.Description = "用户使用少量IP，这是新IP"
	default:
		factor.Score = 30
		factor.Description = "用户经常更换IP，新IP风险较低"
	}

	return factor
}

// evaluateDeviceRisk 评估设备风险
func (re *RiskEngine) evaluateDeviceRisk(ctx *LoginContext, stats *UserLoginStats, trustedDevice *TrustedDevice) RiskFactor {
	factor := RiskFactor{
		Name:   "device_risk",
		Weight: re.config.NewDeviceWeight,
		Score:  0,
	}

	// 如果是信任设备，风险为0
	if trustedDevice != nil && !trustedDevice.IsExpired() {
		factor.Score = 0
		factor.Description = "信任设备"
		return factor
	}

	// 检查设备指纹
	if ctx.DeviceFingerprint == "" {
		factor.Score = 60
		factor.Description = "无设备指纹信息"
		return factor
	}

	if stats == nil || len(stats.LastDevices) == 0 {
		factor.Score = 50
		factor.Description = "无历史设备记录"
		return factor
	}

	// 检查是否是已知设备
	for _, device := range stats.LastDevices {
		if device == ctx.DeviceFingerprint {
			factor.Score = 10
			factor.Description = "已知设备（未信任）"
			return factor
		}
	}

	factor.Score = 80
	factor.Description = "新设备"
	return factor
}

// evaluateTimeRisk 评估登录时间风险
func (re *RiskEngine) evaluateTimeRisk(ctx *LoginContext, stats *UserLoginStats) RiskFactor {
	factor := RiskFactor{
		Name:   "time_risk",
		Weight: re.config.UnusualTimeWeight,
		Score:  0,
	}

	hour := ctx.Timestamp.Hour()

	// 凌晨2-5点视为高风险时段
	if hour >= 2 && hour <= 5 {
		factor.Score = 70
		factor.Description = "凌晨时段登录"
		return factor
	}

	// 如果没有历史数据，无法判断
	if stats == nil || stats.NormalHours == nil {
		// 工作时间(9-18)视为正常
		if hour >= 9 && hour <= 18 {
			factor.Score = 10
			factor.Description = "工作时间登录"
		} else {
			factor.Score = 30
			factor.Description = "非工作时间登录"
		}
		return factor
	}

	// 检查用户在该时段的历史登录频率
	totalLogins := 0
	hourLogins := stats.NormalHours[hour]
	for _, count := range stats.NormalHours {
		totalLogins += count
	}

	if totalLogins == 0 {
		factor.Score = 30
		factor.Description = "无历史登录数据"
		return factor
	}

	// 计算该时段的登录比例
	hourRatio := float64(hourLogins) / float64(totalLogins)

	switch {
	case hourRatio > 0.3:
		factor.Score = 0
		factor.Description = "用户常用此时段登录"
	case hourRatio > 0.1:
		factor.Score = 20
		factor.Description = "用户偶尔在此时段登录"
	case hourRatio > 0:
		factor.Score = 50
		factor.Description = "用户很少在此时段登录"
	default:
		factor.Score = 70
		factor.Description = "用户从未在此时段登录"
	}

	return factor
}

// evaluateGeoRisk 评估地理位置风险
func (re *RiskEngine) evaluateGeoRisk(ctx *LoginContext, stats *UserLoginStats) RiskFactor {
	factor := RiskFactor{
		Name:   "geo_risk",
		Weight: re.config.GeoChangeWeight,
		Score:  0,
	}

	// 如果没有地理位置信息，无法评估
	if ctx.GeoLocation == nil {
		factor.Score = 30
		factor.Description = "无地理位置信息"
		return factor
	}

	if stats == nil || len(stats.LastLocations) == 0 {
		factor.Score = 20
		factor.Description = "无历史地理位置记录"
		return factor
	}

	// 检查是否在同一国家
	lastLocation := stats.LastLocations[len(stats.LastLocations)-1]
	if lastLocation.Country == ctx.GeoLocation.Country {
		// 同一国家，检查城市
		if lastLocation.City == ctx.GeoLocation.City {
			factor.Score = 0
			factor.Description = "同一城市"
		} else {
			factor.Score = 30
			factor.Description = "同一国家不同城市"
		}
	} else {
		// 不同国家，计算距离
		distance := calculateDistance(
			lastLocation.Latitude, lastLocation.Longitude,
			ctx.GeoLocation.Latitude, ctx.GeoLocation.Longitude,
		)

		switch {
		case distance < 100: // 100km内
			factor.Score = 20
			factor.Description = "附近地区"
		case distance < 500: // 500km内
			factor.Score = 40
			factor.Description = "较远地区"
		case distance < 2000: // 2000km内
			factor.Score = 60
			factor.Description = "远距离"
		default:
			factor.Score = 90
			factor.Description = "跨国登录"
		}
	}

	return factor
}

// evaluateRapidLoginRisk 评估短时间多次登录风险
func (re *RiskEngine) evaluateRapidLoginRisk(ctx *LoginContext, stats *UserLoginStats) RiskFactor {
	factor := RiskFactor{
		Name:   "rapid_login_risk",
		Weight: re.config.RapidLoginWeight,
		Score:  0,
	}

	if stats == nil {
		factor.Score = 0
		factor.Description = "无历史数据"
		return factor
	}

	// 检查窗口期内的登录次数
	windowStart := ctx.Timestamp.Add(-re.config.RapidLoginWindow)
	recentLogins := 0

	for _, t := range stats.LoginTimes {
		if t.After(windowStart) {
			recentLogins++
		}
	}

	if recentLogins >= re.config.RapidLoginThreshold {
		factor.Score = 90
		factor.Description = "短时间多次登录"
	} else if recentLogins >= re.config.RapidLoginThreshold/2 {
		factor.Score = 50
		factor.Description = "登录频率略高"
	} else {
		factor.Score = 0
		factor.Description = "正常登录频率"
	}

	return factor
}

// getRiskLevel 根据分数确定风险等级
func (re *RiskEngine) getRiskLevel(score int) RiskLevel {
	switch {
	case score < re.config.LowRiskThreshold:
		return RiskLow
	case score < re.config.MediumRiskThreshold:
		return RiskMedium
	case score < re.config.HighRiskThreshold:
		return RiskHigh
	default:
		return RiskCritical
	}
}

// RecordLogin 记录登录历史
func (re *RiskEngine) RecordLogin(ctx *LoginContext, success bool, riskScore int) {
	re.mu.Lock()
	defer re.mu.Unlock()

	// 更新用户统计
	stats, exists := re.userStats[ctx.UserID]
	if !exists {
		stats = &UserLoginStats{
			UserID:      ctx.UserID,
			NormalHours: make(map[int]int),
		}
		re.userStats[ctx.UserID] = stats
	}

	stats.mu.Lock()
	stats.LastLoginTime = ctx.Timestamp
	stats.TotalLogins++

	if !success {
		stats.FailedAttempts++
	}

	// 更新IP列表 (保留最近10个)
	stats.LastIPs = append(stats.LastIPs, ctx.IP)
	if len(stats.LastIPs) > 10 {
		stats.LastIPs = stats.LastIPs[len(stats.LastIPs)-10:]
	}

	// 更新设备列表 (保留最近10个)
	if ctx.DeviceFingerprint != "" {
		stats.LastDevices = append(stats.LastDevices, ctx.DeviceFingerprint)
		if len(stats.LastDevices) > 10 {
			stats.LastDevices = stats.LastDevices[len(stats.LastDevices)-10:]
		}
	}

	// 更新地理位置列表 (保留最近10个)
	if ctx.GeoLocation != nil {
		stats.LastLocations = append(stats.LastLocations, *ctx.GeoLocation)
		if len(stats.LastLocations) > 10 {
			stats.LastLocations = stats.LastLocations[len(stats.LastLocations)-10:]
		}
	}

	// 更新登录时间列表 (保留最近100个)
	stats.LoginTimes = append(stats.LoginTimes, ctx.Timestamp)
	if len(stats.LoginTimes) > 100 {
		stats.LoginTimes = stats.LoginTimes[len(stats.LoginTimes)-100:]
	}

	// 更新小时分布
	hour := ctx.Timestamp.Hour()
	stats.NormalHours[hour]++
	stats.mu.Unlock()

	// 添加到登录历史
	history := &LoginHistory{
		UserID:            ctx.UserID,
		IP:                ctx.IP,
		UserAgent:         ctx.UserAgent,
		DeviceFingerprint: ctx.DeviceFingerprint,
		GeoLocation:       ctx.GeoLocation,
		Success:           success,
		RiskScore:         riskScore,
		Timestamp:         ctx.Timestamp,
	}

	re.loginHistory[ctx.UserID] = append(re.loginHistory[ctx.UserID], history)
	// 保留最近1000条历史记录
	if len(re.loginHistory[ctx.UserID]) > 1000 {
		re.loginHistory[ctx.UserID] = re.loginHistory[ctx.UserID][len(re.loginHistory[ctx.UserID])-1000:]
	}
}

// GetUserStats 获取用户统计信息
func (re *RiskEngine) GetUserStats(userID string) *UserLoginStats {
	re.mu.RLock()
	defer re.mu.RUnlock()
	return re.userStats[userID]
}

// GetLoginHistory 获取用户登录历史
func (re *RiskEngine) GetLoginHistory(userID string, limit int) []*LoginHistory {
	re.mu.RLock()
	defer re.mu.RUnlock()

	history := re.loginHistory[userID]
	if limit <= 0 || limit > len(history) {
		limit = len(history)
	}

	// 返回最近的记录
	start := len(history) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*LoginHistory, limit)
	copy(result, history[start:])
	return result
}

// calculateDistance 计算两点之间的距离 (Haversine公式, 单位: km)
func calculateDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadius = 6371.0 // 地球半径 (km)

	// 转换为弧度
	lat1Rad := lat1 * (3.141592653589793 / 180)
	lat2Rad := lat2 * (3.141592653589793 / 180)
	deltaLat := (lat2 - lat1) * (3.141592653589793 / 180)
	deltaLon := (lon2 - lon1) * (3.141592653589793 / 180)

	// Haversine公式
	a := sin(deltaLat/2)*sin(deltaLat/2) +
		cos(lat1Rad)*cos(lat2Rad)*
			sin(deltaLon/2)*sin(deltaLon/2)
	c := 2 * atan2(sqrt(a), sqrt(1-a))

	return earthRadius * c
}

// sin 正弦函数 (使用math包的近似实现，避免依赖)
func sin(x float64) float64 {
	// Taylor级数展开
	x = normalizeAngle(x)
	result := x
	term := x
	for i := 1; i < 10; i++ {
		term *= -x * x / float64(2*i*(2*i+1))
		result += term
	}
	return result
}

// cos 余弦函数
func cos(x float64) float64 {
	return sin(x + 3.141592653589793/2)
}

// sqrt 平方根 (使用Newton法)
func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	if x == 0 {
		return 0
	}

	z := x / 2
	for i := 0; i < 50; i++ {
		z = (z + x/z) / 2
	}
	return z
}

// atan2 反正切2
func atan2(y, x float64) float64 {
	if x > 0 {
		return atan(y / x)
	}
	if x < 0 && y >= 0 {
		return atan(y/x) + 3.141592653589793
	}
	if x < 0 && y < 0 {
		return atan(y/x) - 3.141592653589793
	}
	if x == 0 && y > 0 {
		return 3.141592653589793 / 2
	}
	if x == 0 && y < 0 {
		return -3.141592653589793 / 2
	}
	return 0
}

// atan 反正切 (使用Taylor级数)
func atan(x float64) float64 {
	if x > 1 || x < -1 {
		return sign(x)*3.141592653589793/2 - atan(1/x)
	}

	result := x
	term := x
	for i := 1; i < 20; i++ {
		term *= -x * x
		result += term / float64(2*i+1)
	}
	return result
}

// sign 符号函数
func sign(x float64) float64 {
	if x > 0 {
		return 1
	}
	if x < 0 {
		return -1
	}
	return 0
}

// normalizeAngle 角度归一化到 [-π, π]
func normalizeAngle(x float64) float64 {
	pi := 3.141592653589793
	for x > pi {
		x -= 2 * pi
	}
	for x < -pi {
		x += 2 * pi
	}
	return x
}
