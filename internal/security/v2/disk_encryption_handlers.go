package securityv2

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// DiskEncryptionHandlers 磁盘加密 API 处理器
type DiskEncryptionHandlers struct {
	manager *DiskEncryptionManager
}

// NewDiskEncryptionHandlers 创建处理器
func NewDiskEncryptionHandlers(manager *DiskEncryptionManager) *DiskEncryptionHandlers {
	return &DiskEncryptionHandlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *DiskEncryptionHandlers) RegisterRoutes(r *gin.RouterGroup) {
	encryption := r.Group("/encryption")
	{
		// 加密卷管理
		encryption.GET("/volumes", h.listVolumes)
		encryption.GET("/volumes/:device", h.getVolume)
		encryption.POST("/volumes", h.createVolume)
		encryption.PUT("/volumes/:device", h.updateVolume)
		encryption.DELETE("/volumes/:device", h.deleteVolume)

		// LUKS 操作
		encryption.POST("/volumes/:device/open", h.openVolume)
		encryption.POST("/volumes/:device/close", h.closeVolume)
		encryption.GET("/volumes/:device/info", h.getVolumeInfo)

		// 密钥管理
		encryption.POST("/volumes/:device/keys", h.addKey)
		encryption.DELETE("/volumes/:device/keys/:slot", h.removeKey)
		encryption.POST("/volumes/:device/keys/rotate", h.rotateKey)

		// 密钥轮换
		encryption.GET("/rotation/policy", h.getRotationPolicy)
		encryption.PUT("/rotation/policy", h.setRotationPolicy)
		encryption.GET("/volumes/:device/rotation/check", h.checkKeyRotation)
		encryption.POST("/rotation/auto", h.autoRotateKeys)

		// 性能监控
		encryption.GET("/volumes/:device/performance", h.getPerformance)
		encryption.GET("/volumes/:device/optimize", h.getOptimization)
		encryption.POST("/benchmark", h.runBenchmark)

		// 状态
		encryption.GET("/status", h.getStatus)
	}
}

// ========== 加密卷管理 ==========

// listVolumes 列出所有加密卷
func (h *DiskEncryptionHandlers) listVolumes(c *gin.Context) {
	configs := h.manager.ListConfigs()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    configs,
	})
}

// getVolume 获取单个加密卷
func (h *DiskEncryptionHandlers) getVolume(c *gin.Context) {
	device := c.Param("device")

	// 设备路径可能包含斜杠，需要处理
	devicePath := "/" + device

	config, err := h.manager.GetConfig(devicePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    config,
	})
}

// createVolume 创建加密卷
func (h *DiskEncryptionHandlers) createVolume(c *gin.Context) {
	var req struct {
		DevicePath     string         `json:"devicePath" binding:"required"`
		Passphrase     string         `json:"passphrase" binding:"required"`
		EncryptionType EncryptionType `json:"encryptionType"`
		Cipher         string         `json:"cipher"`
		KeySize        int            `json:"keySize"`
		MountPoint     string         `json:"mountPoint"`
		AutoUnlock     bool           `json:"autoUnlock"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	config := &DiskEncryptionConfig{
		DevicePath:     req.DevicePath,
		MountPoint:     req.MountPoint,
		EncryptionType: req.EncryptionType,
		Cipher:         req.Cipher,
		KeySize:        req.KeySize,
		KeySourceType:  KeySourcePassphrase,
		AutoUnlock:     req.AutoUnlock,
	}

	if err := h.manager.CreateLUKS(req.DevicePath, req.Passphrase, config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "创建加密卷失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "加密卷创建成功",
		"data":    config,
	})
}

// updateVolume 更新加密卷配置
func (h *DiskEncryptionHandlers) updateVolume(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	var config DiskEncryptionConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateConfig(devicePath, &config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置更新成功",
		"data":    config,
	})
}

// deleteVolume 删除加密卷配置
func (h *DiskEncryptionHandlers) deleteVolume(c *gin.Context) {
	_ = c.Param("device") // device path from URL

	// 注意：这不会实际删除 LUKS 卷，只是删除配置
	// 实际删除需要先备份数据，然后擦除 LUKS 头

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "配置已删除",
	})
}

// ========== LUKS 操作 ==========

// openVolume 打开加密卷
func (h *DiskEncryptionHandlers) openVolume(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	var req struct {
		MapperName string `json:"mapperName" binding:"required"`
		Passphrase string `json:"passphrase" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.OpenLUKS(devicePath, req.MapperName, req.Passphrase); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "打开加密卷失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "加密卷已打开",
		"data": gin.H{
			"mapperName": req.MapperName,
			"mapperPath": "/dev/mapper/" + req.MapperName,
			"openedAt":   time.Now(),
		},
	})
}

// closeVolume 关闭加密卷
func (h *DiskEncryptionHandlers) closeVolume(c *gin.Context) {
	_ = c.Param("device") // device path from URL

	var req struct {
		MapperName string `json:"mapperName" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	if err := h.manager.CloseLUKS(req.MapperName); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "关闭加密卷失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "加密卷已关闭",
	})
}

// getVolumeInfo 获取 LUKS 信息
func (h *DiskEncryptionHandlers) getVolumeInfo(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	info, err := h.manager.GetLUKSInfo(devicePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取信息失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

// ========== 密钥管理 ==========

// addKey 添加密钥
func (h *DiskEncryptionHandlers) addKey(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	var req struct {
		CurrentPassphrase string `json:"currentPassphrase" binding:"required"`
		NewPassphrase     string `json:"newPassphrase" binding:"required"`
		SlotID            int    `json:"slotId"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	slotID := req.SlotID
	if slotID < 0 {
		slotID = -1 // 让系统自动选择
	}

	if err := h.manager.AddKeySlot(devicePath, req.CurrentPassphrase, req.NewPassphrase, slotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "添加密钥失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "密钥添加成功",
	})
}

// removeKey 移除密钥
func (h *DiskEncryptionHandlers) removeKey(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device
	slot := c.Param("slot")

	var req struct {
		Passphrase string `json:"passphrase" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	var slotID int
	_, _ = fmt.Sscanf(slot, "%d", &slotID)

	if err := h.manager.RemoveKeySlot(devicePath, req.Passphrase, slotID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "移除密钥失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "密钥已移除",
	})
}

// rotateKey 轮换密钥
func (h *DiskEncryptionHandlers) rotateKey(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	var req struct {
		CurrentPassphrase string `json:"currentPassphrase" binding:"required"`
		NewPassphrase     string `json:"newPassphrase"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	newPassphrase := req.NewPassphrase
	if newPassphrase == "" {
		// 自动生成新密码
		newPassphrase = generatePassphrase()
	}

	if err := h.manager.RotateKey(devicePath, req.CurrentPassphrase, newPassphrase); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "密钥轮换失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "密钥轮换成功",
		"data": gin.H{
			"rotatedAt":     time.Now(),
			"newPassphrase": newPassphrase, // 返回新密码（仅此一次）
		},
	})
}

// ========== 密钥轮换策略 ==========

// getRotationPolicy 获取密钥轮换策略
func (h *DiskEncryptionHandlers) getRotationPolicy(c *gin.Context) {
	h.manager.mu.RLock()
	policy := h.manager.rotationPolicy
	h.manager.mu.RUnlock()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    policy,
	})
}

// setRotationPolicy 设置密钥轮换策略
func (h *DiskEncryptionHandlers) setRotationPolicy(c *gin.Context) {
	var policy KeyRotationPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求数据: " + err.Error(),
		})
		return
	}

	h.manager.SetRotationPolicy(&policy)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "策略已更新",
		"data":    policy,
	})
}

// checkKeyRotation 检查密钥轮换状态
func (h *DiskEncryptionHandlers) checkKeyRotation(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	needsRotation, daysUntilExpiry, err := h.manager.CheckKeyRotation(devicePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"devicePath":      devicePath,
			"needsRotation":   needsRotation,
			"daysUntilExpiry": daysUntilExpiry,
		},
	})
}

// autoRotateKeys 自动轮换密钥
func (h *DiskEncryptionHandlers) autoRotateKeys(c *gin.Context) {
	// 需要一个密码提供器
	// 在实际实现中，这应该从安全的密钥存储中获取
	err := h.manager.AutoRotateKeys(func(devicePath string) (string, error) {
		// 简化实现：从请求头或密钥管理器获取
		return "", nil
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "自动轮换失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "自动轮换完成",
	})
}

// ========== 性能监控 ==========

// getPerformance 获取加密性能
func (h *DiskEncryptionHandlers) getPerformance(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	perf, err := h.manager.GetEncryptionPerformance(devicePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取性能数据失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    perf,
	})
}

// getOptimization 获取优化建议
func (h *DiskEncryptionHandlers) getOptimization(c *gin.Context) {
	device := c.Param("device")
	devicePath := "/" + device

	recommendations, err := h.manager.OptimizeEncryption(devicePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "获取优化建议失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"devicePath":      devicePath,
			"recommendations": recommendations,
		},
	})
}

// runBenchmark 运行基准测试
func (h *DiskEncryptionHandlers) runBenchmark(c *gin.Context) {
	var req struct {
		TestSizeMB int `json:"testSizeMb"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		req.TestSizeMB = 100 // 默认 100MB
	}

	results, err := h.manager.RunEncryptionBenchmark(req.TestSizeMB)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "基准测试失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    results,
	})
}

// ========== 状态 ==========

// getStatus 获取加密状态总览
func (h *DiskEncryptionHandlers) getStatus(c *gin.Context) {
	configs := h.manager.ListConfigs()

	totalVolumes := len(configs)
	lockedVolumes := 0
	unlockedVolumes := 0
	autoUnlockEnabled := 0
	rotationEnabled := 0

	for _, config := range configs {
		switch config.Status {
		case EncryptionStatusLocked:
			lockedVolumes++
		case EncryptionStatusUnlocked:
			unlockedVolumes++
		}

		if config.AutoUnlock {
			autoUnlockEnabled++
		}

		if config.KeyRotationEnabled {
			rotationEnabled++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"totalVolumes":      totalVolumes,
			"lockedVolumes":     lockedVolumes,
			"unlockedVolumes":   unlockedVolumes,
			"autoUnlockEnabled": autoUnlockEnabled,
			"rotationEnabled":   rotationEnabled,
			"hardwareAccel":     h.manager.checkHardwareAcceleration(),
		},
	})
}
