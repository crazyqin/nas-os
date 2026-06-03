package bluetoothprovision

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultScanner 实现BLE扫描器
type DefaultScanner struct {
	mu        sync.RWMutex
	scanning  bool
	devices   map[string]*BLEDevice
	cancel    context.CancelFunc
	onDevice  func(BLEDevice) // 设备发现回调
}

// NewDefaultScanner 创建默认BLE扫描器
func NewDefaultScanner() *DefaultScanner {
	return &DefaultScanner{
		devices: make(map[string]*BLEDevice),
	}
}

// WithOnDeviceCallback 设置设备发现回调
func (s *DefaultScanner) WithOnDeviceCallback(fn func(BLEDevice)) *DefaultScanner {
	s.onDevice = fn
	return s
}

// Scan 执行BLE设备扫描
func (s *DefaultScanner) Scan(req ScanRequest) ([]BLEDevice, error) {
	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return nil, fmt.Errorf("扫描已在进行中")
	}
	s.scanning = true

	// 清空旧设备
	s.devices = make(map[string]*BLEDevice)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(req.Duration)*time.Second)
	s.cancel = cancel
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.cancel = nil
		s.mu.Unlock()
	}()

	log.Printf("[BLE Scanner] 开始扫描, 时长: %d秒", req.Duration)

	// 模拟BLE扫描过程
	if err := s.simulateScan(ctx, req); err != nil {
		return nil, fmt.Errorf("扫描失败: %w", err)
	}

	// 收集结果
	s.mu.RLock()
	result := make([]BLEDevice, 0, len(s.devices))
	for _, d := range s.devices {
		// 应用过滤条件
		if req.MinRSSI != 0 && d.RSSI < req.MinRSSI {
			continue
		}
		if len(req.Filter) > 0 && !matchServices(d.Services, req.Filter) {
			continue
		}
		result = append(result, *d)
	}
	s.mu.RUnlock()

	// 限制设备数量
	if req.MaxDevices > 0 && len(result) > req.MaxDevices {
		result = result[:req.MaxDevices]
	}

	log.Printf("[BLE Scanner] 扫描完成, 发现 %d 个设备", len(result))
	return result, nil
}

// simulateScan 模拟BLE扫描（实际实现需要调用系统BLE API）
func (s *DefaultScanner) simulateScan(ctx context.Context, req ScanRequest) error {
	// 模拟发现设备
	mockDevices := []BLEDevice{
		{
			ID:           uuid.New().String(),
			Name:         "SmartLight-01",
			Address:      "AA:BB:CC:DD:EE:01",
			RSSI:         -45,
			Services:     []string{"0000fff0-0000-1000-8000-00805f9b34fb"},
			Manufacturer: "XiaoMi",
			Model:        "YLDP06YL",
			FirmwareVer:  "1.2.3",
			Discovered:   time.Now(),
		},
		{
			ID:           uuid.New().String(),
			Name:         "NAS-Provision",
			Address:      "AA:BB:CC:DD:EE:02",
			RSSI:         -52,
			Services:     []string{"0000fff0-0000-1000-8000-00805f9b34fb", "0000ffe0-0000-1000-8000-00805f9b34fb"},
			Manufacturer: "NAS-OS",
			Model:        "Provisioner-v1",
			FirmwareVer:  "2.0.0",
			Discovered:   time.Now(),
		},
		{
			ID:           uuid.New().String(),
			Name:         "IoT-Sensor-Hub",
			Address:      "AA:BB:CC:DD:EE:03",
			RSSI:         -68,
			Services:     []string{"0000ffe0-0000-1000-8000-00805f9b34fb"},
			Manufacturer: "ESP32",
			Model:        "WROOM-32",
			FirmwareVer:  "0.9.1",
			Discovered:   time.Now(),
		},
	}

	for i, d := range mockDevices {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(200 * time.Millisecond) // 模拟发现延迟

			s.mu.Lock()
			s.devices[d.ID] = &d
			s.mu.Unlock()

			if s.onDevice != nil {
				s.onDevice(d)
			}
			log.Printf("[BLE Scanner] 发现设备 [%d/%d]: %s (%s)", i+1, len(mockDevices), d.Name, d.Address)
		}
	}

	// 等待扫描时长结束或取消
	<-ctx.Done()
	return nil
}

// StopScan 停止扫描
func (s *DefaultScanner) StopScan() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.scanning {
		return fmt.Errorf("没有正在进行的扫描")
	}

	if s.cancel != nil {
		s.cancel()
	}

	return nil
}

// Connect 连接到BLE设备
func (s *DefaultScanner) Connect(deviceID string) error {
	s.mu.RLock()
	device, ok := s.devices[deviceID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	log.Printf("[BLE Scanner] 连接到设备: %s (%s)", device.Name, device.Address)

	// 模拟连接过程
	time.Sleep(100 * time.Millisecond)

	s.mu.Lock()
	device.Connected = true
	s.mu.Unlock()

	return nil
}

// Disconnect 断开BLE设备连接
func (s *DefaultScanner) Disconnect(deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	device, ok := s.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	log.Printf("[BLE Scanner] 断开设备: %s (%s)", device.Name, device.Address)
	device.Connected = false
	return nil
}

// IsScanning 是否正在扫描
func (s *DefaultScanner) IsScanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanning
}

// GetDevices 获取已发现设备列表
func (s *DefaultScanner) GetDevices() []BLEDevice {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]BLEDevice, 0, len(s.devices))
	for _, d := range s.devices {
		result = append(result, *d)
	}
	return result
}

// GetDevice 获取指定设备
func (s *DefaultScanner) GetDevice(deviceID string) (*BLEDevice, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	device, ok := s.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}
	return device, nil
}

// matchServices 检查设备服务是否匹配过滤条件
func matchServices(deviceServices, filter []string) bool {
	if len(filter) == 0 {
		return true
	}
	filterMap := make(map[string]bool, len(filter))
	for _, f := range filter {
		filterMap[f] = true
	}
	for _, s := range deviceServices {
		if filterMap[s] {
			return true
		}
	}
	return false
}
