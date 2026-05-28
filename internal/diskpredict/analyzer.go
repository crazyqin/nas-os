package diskpredict

import (
	"math"
)

// Analyzer SMART 数据分析器
type Analyzer struct {
	// 关键 SMART 属性权重配置
	weights map[uint8]float64
}

// NewAnalyzer 创建分析器
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		weights: map[uint8]float64{
			5:   25.0,  // Reallocated Sectors Count（重分配扇区计数）- 高权重
			187: 15.0,  // Reported Uncorrectable Errors（报告的不可纠正错误）
			188: 10.0,  // Command Timeout（命令超时）
			197: 20.0,  // Current Pending Sector Count（当前待处理扇区计数）- 高权重
			198: 20.0,  // Offline Uncorrectable（离线不可纠正扇区计数）- 高权重
			9:   10.0,  // Power-On Hours（通电时间）
		},
	}
}

// AnalyzeSMARTData 分析 SMART 数据并返回属性分析结果
func (a *Analyzer) AnalyzeSMARTData(data *SMARTData) []AttributeAnalysis {
	analyses := make([]AttributeAnalysis, 0, len(data.Attributes))

	for _, attr := range data.Attributes {
		analysis := a.analyzeAttribute(attr)
		analyses = append(analyses, analysis)
	}

	return analyses
}

// analyzeAttribute 分析单个 SMART 属性
func (a *Analyzer) analyzeAttribute(attr SMARTAttribute) AttributeAnalysis {
	analysis := AttributeAnalysis{
		ID:        attr.ID,
		Name:      attr.Name,
		Value:     attr.Value,
		Threshold: attr.Threshold,
	}

	// 获取权重，如果没有配置则使用默认权重
	weight, exists := a.weights[attr.ID]
	if !exists {
		weight = 1.0
	}
	analysis.Weight = weight

	// 计算属性得分
	analysis.Score = a.calculateAttributeScore(attr)
	analysis.WeightedScore = analysis.Score * weight

	// 判断状态
	analysis.Status, analysis.Message = a.getAttributeStatus(attr, analysis.Score)

	return analysis
}

// calculateAttributeScore 计算单个属性的健康得分（0-100）
func (a *Analyzer) calculateAttributeScore(attr SMARTAttribute) float64 {
	// 如果属性已经失败，直接返回0
	if attr.IsFailed {
		return 0
	}

	// 对于关键属性，使用更严格的评分
	if attr.IsCritical {
		return a.calculateCriticalAttributeScore(attr)
	}

	// 普通属性评分
	return a.calculateNormalAttributeScore(attr)
}

// calculateCriticalAttributeScore 计算关键属性得分
func (a *Analyzer) calculateCriticalAttributeScore(attr SMARTAttribute) float64 {
	if attr.RawValue == 0 {
		return 100.0 // 没有错误，满分
	}

	// 根据属性ID使用不同的评分策略
	switch attr.ID {
	case 5: // Reallocated Sectors Count
		return a.scoreReallocatedSectors(attr.RawValue)
	case 197: // Current Pending Sector Count
		return a.scorePendingSectors(attr.RawValue)
	case 198: // Offline Uncorrectable
		return a.scoreOfflineUncorrectable(attr.RawValue)
	case 187: // Reported Uncorrectable Errors
		return a.scoreUncorrectableErrors(attr.RawValue)
	default:
		// 通用关键属性评分：基于值与阈值的比例
		if attr.Threshold > 0 {
			ratio := float64(attr.Value) / float64(attr.Threshold)
			if ratio >= 2.0 {
				return 100.0
			} else if ratio >= 1.0 {
				return 50.0 + (ratio-1.0)*50.0
			}
			return math.Max(0, ratio*50.0)
		}
		return 50.0
	}
}

// scoreReallocatedSectors 评分重分配扇区
func (a *Analyzer) scoreReallocatedSectors(rawValue uint64) float64 {
	switch {
	case rawValue == 0:
		return 100.0
	case rawValue <= 10:
		return 90.0
	case rawValue <= 50:
		return 70.0
	case rawValue <= 100:
		return 50.0
	case rawValue <= 500:
		return 30.0
	default:
		return math.Max(0, 10.0-float64(rawValue)/100.0)
	}
}

// scorePendingSectors 评分待处理扇区
func (a *Analyzer) scorePendingSectors(rawValue uint64) float64 {
	switch {
	case rawValue == 0:
		return 100.0
	case rawValue <= 5:
		return 80.0
	case rawValue <= 20:
		return 60.0
	case rawValue <= 50:
		return 40.0
	case rawValue <= 100:
		return 20.0
	default:
		return math.Max(0, 5.0-float64(rawValue)/200.0)
	}
}

// scoreOfflineUncorrectable 评分离线不可纠正扇区
func (a *Analyzer) scoreOfflineUncorrectable(rawValue uint64) float64 {
	// 与待处理扇区类似的评分逻辑
	return a.scorePendingSectors(rawValue)
}

// scoreUncorrectableErrors 评分不可纠正错误
func (a *Analyzer) scoreUncorrectableErrors(rawValue uint64) float64 {
	switch {
	case rawValue == 0:
		return 100.0
	case rawValue <= 5:
		return 85.0
	case rawValue <= 20:
		return 65.0
	case rawValue <= 50:
		return 45.0
	case rawValue <= 100:
		return 25.0
	default:
		return math.Max(0, 10.0-float64(rawValue)/200.0)
	}
}

// calculateNormalAttributeScore 计算普通属性得分
func (a *Analyzer) calculateNormalAttributeScore(attr SMARTAttribute) float64 {
	if attr.Threshold == 0 {
		// 没有阈值，基于值本身评分（值越高越好，最大253）
		return math.Min(100.0, float64(attr.Value)/253.0*100.0)
	}

	// 基于值与阈值的比例
	ratio := float64(attr.Value) / float64(attr.Threshold)
	if ratio >= 2.0 {
		return 100.0
	} else if ratio >= 1.0 {
		return 50.0 + (ratio-1.0)*50.0
	}
	return math.Max(0, ratio*50.0)
}

// getAttributeStatus 获取属性状态和消息
func (a *Analyzer) getAttributeStatus(attr SMARTAttribute, score float64) (string, string) {
	if attr.IsFailed {
		return "critical", "属性已失败"
	}

	if score >= 80 {
		return "normal", "状态正常"
	} else if score >= 60 {
		return "warning", "存在轻微异常"
	} else if score >= 40 {
		return "warning", "需要关注"
	} else if score >= 20 {
		return "critical", "建议更换磁盘"
	}
	return "critical", "强烈建议立即更换磁盘"
}

// AnalyzeTemperature 分析温度并返回得分
func (a *Analyzer) AnalyzeTemperature(temperature int) (float64, string) {
	switch {
	case temperature < 0:
		return 0, "温度异常低"
	case temperature <= 30:
		return 100, "温度正常"
	case temperature <= 40:
		return 90, "温度正常"
	case temperature <= 45:
		return 80, "温度略高"
	case temperature <= 50:
		return 60, "温度偏高，建议改善散热"
	case temperature <= 55:
		return 40, "温度过高，可能影响寿命"
	case temperature <= 60:
		return 20, "温度危险，可能导致故障"
	default:
		return 0, "温度极高，立即停止使用"
	}
}

// AnalyzePowerOnHours 分析通电时间并返回得分
func (a *Analyzer) AnalyzePowerOnHours(hours uint64) (float64, string) {
	switch {
	case hours <= 0:
		return 100, "新磁盘"
	case hours <= 8760:  // 1年
		return 100, "通电时间正常"
	case hours <= 17520: // 2年
		return 90, "通电时间正常"
	case hours <= 26280: // 3年
		return 80, "通电时间较长"
	case hours <= 35040: // 4年
		return 70, "通电时间较长"
	case hours <= 43800: // 5年
		return 60, "通电时间很长"
	case hours <= 52560: // 6年
		return 50, "通电时间很长"
	case hours <= 61320: // 7年
		return 40, "通电时间超长"
	case hours <= 70080: // 8年
		return 30, "通电时间超长"
	case hours <= 78840: // 9年
		return 20, "建议更换磁盘"
	default:
		return 10, "强烈建议更换磁盘"
	}
}

// IdentifyRiskFactors 识别风险因素
func (a *Analyzer) IdentifyRiskFactors(analyses []AttributeAnalysis, temperature int, powerOnHours uint64) []string {
	riskFactors := make([]string, 0)

	// 检查关键属性
	for _, analysis := range analyses {
		if analysis.Status == "critical" {
			switch analysis.ID {
			case 5:
				riskFactors = append(riskFactors, "存在重分配扇区，磁盘可能有坏道")
			case 197:
				riskFactors = append(riskFactors, "存在待处理扇区，数据完整性受影响")
			case 198:
				riskFactors = append(riskFactors, "存在离线不可纠正扇区")
			case 187:
				riskFactors = append(riskFactors, "报告了不可纠正错误")
			case 188:
				riskFactors = append(riskFactors, "存在命令超时问题")
			default:
				riskFactors = append(riskFactors, "属性 "+analysis.Name+" 异常")
			}
		}
	}

	// 检查温度
	if temperature > 50 {
		riskFactors = append(riskFactors, "温度过高")
	}

	// 检查通电时间
	if powerOnHours > 43800 { // 5年
		riskFactors = append(riskFactors, "通电时间过长")
	}

	return riskFactors
}
