package main

import (
	"fmt"
	"log"
	"nas-os/internal/lab/smartbandwidth"
)

func main() {
	// 创建智能带宽管理器
	manager := smartbandwidth.NewManager(&smartbandwidth.SmartBandwidthConfig{
		TotalBandwidthMbps: 1000,
		Enabled:            true,
		Interface:          "eth0",
		AdjustInterval:     30,
	})

	// 创建带宽规则
	videoRule := &smartbandwidth.BandwidthRule{
		Name:         "Video Streaming",
		TrafficClass: smartbandwidth.TrafficClassStreaming,
		Priority:     8,
		MinMbps:      50,
		MaxMbps:      500,
	}

	created, err := manager.SetBandwidthLimit(videoRule)
	if err != nil {
		log.Fatalf("Failed to create rule: %v", err)
	}
	fmt.Printf("Created rule: %s (ID: %s)\n", created.Name, created.ID)

	// 创建QoS策略
	policy := &smartbandwidth.QoSPolicy{
		Name:     "Critical Services",
		Priority: 10,
		MinMbps:  100,
		MaxMbps:  800,
	}

	qos, err := manager.ApplyQoSPolicy(policy)
	if err != nil {
		log.Fatalf("Failed to apply QoS policy: %v", err)
	}
	fmt.Printf("Applied QoS policy: %s (ID: %s)\n", qos.Name, qos.ID)

	// 流量分类示例
	class := manager.ClassifyTraffic(
		"192.168.1.1",
		"192.168.1.100",
		12345,
		443,
		"tcp",
	)
	fmt.Printf("Traffic class for port 443: %s\n", class)

	// 获取带宽使用情况
	usage := manager.GetBandwidthUsage()
	fmt.Printf("Bandwidth usage: %.2f/%.2f Mbps (%.1f%%)\n",
		usage.UsedMbps, usage.TotalMbps, usage.Utilization)

	// 获取流量类型汇总
	summary := manager.GetClassSummary()
	for class, s := range summary {
		fmt.Printf("Class %s: %d rules, %.2f Mbps\n", class, s.RuleCount, s.TotalMbps)
	}

	// 创建流量配置文件
	profile := &smartbandwidth.TrafficProfile{
		Name:         "AI Inference",
		TrafficClass: smartbandwidth.TrafficClassAIInference,
		Priority:     9,
		MinMbps:      100,
		MaxMbps:      600,
		Description:  "AI model inference services",
	}

	createdProfile, err := manager.CreateTrafficProfile(profile)
	if err != nil {
		log.Fatalf("Failed to create profile: %v", err)
	}
	fmt.Printf("Created profile: %s (ID: %s)\n", createdProfile.Name, createdProfile.ID)

	// 触发动态调整
	if err := manager.AdjustDynamic(); err != nil {
		log.Printf("Dynamic adjustment failed: %v", err)
	} else {
		fmt.Println("Dynamic adjustment completed")
	}

	// 列出所有规则
	rules := manager.ListBandwidthRules()
	fmt.Printf("Total bandwidth rules: %d\n", len(rules))

	// 列出所有QoS策略
	policies := manager.ListQoSPolicies()
	fmt.Printf("Total QoS policies: %d\n", len(policies))

	// 获取所有统计
	stats := manager.GetAllBandwidthStats()
	fmt.Printf("Total stats entries: %d\n", len(stats))
}
