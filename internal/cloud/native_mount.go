// Package cloud provides native cloud storage mounting capabilities
// Inspired by 飞牛fnOS native cloud drive integration
package cloud

import (
	"context"
	"fmt"
	"time"
)

// MountProvider defines the interface for cloud storage providers
type MountProvider interface {
	// Name returns the provider name (e.g., "115", "quark", "baidu")
	Name() string
	
	// Mount mounts the cloud storage to local path
	Mount(ctx context.Context, config *MountConfig) error
	
	// Unmount unmounts the cloud storage
	Unmount(ctx context.Context, mountPath string) error
	
	// Status returns the current mount status
	Status(ctx context.Context, mountPath string) (*MountStatus, error)
}

// MountConfig contains configuration for mounting cloud storage
type MountConfig struct {
	// Provider is the cloud storage provider name
	Provider string `json:"provider"`
	
	// MountPath is the local mount point
	MountPath string `json:"mountPath"`
	
	// Credentials for authentication (encrypted)
	Credentials map[string]string `json:"credentials"`
	
	// Options for mount behavior
	Options MountOptions `json:"options"`
}

// MountOptions contains mount behavior options
type MountOptions struct {
	// ReadOnly mounts the storage as read-only
	ReadOnly bool `json:"readOnly"`
	
	// AllowOther allows other users to access the mount
	AllowOther bool `json:"allowOther"`
	
	// CacheSize sets the cache size in MB
	CacheSize int `json:"cacheSize"`
	
	// RefreshInterval sets the refresh interval for file list
	RefreshInterval time.Duration `json:"refreshInterval"`
}

// MountStatus represents the current status of a mount
type MountStatus struct {
	// Provider is the cloud storage provider
	Provider string `json:"provider"`
	
	// MountPath is the local mount point
	MountPath string `json:"mountPath"`
	
	// Status is the current status (mounted, unmounted, error)
	Status string `json:"status"`
	
	// UsedSpace is the used space in bytes
	UsedSpace int64 `json:"usedSpace"`
	
	// TotalSpace is the total space in bytes
	TotalSpace int64 `json:"totalSpace"`
	
	// LastSync is the last sync time
	LastSync time.Time `json:"lastSync"`
	
	// Error contains any error message
	Error string `json:"error,omitempty"`
}

// NativeMountService manages native cloud storage mounting
type NativeMountService struct {
	providers map[string]MountProvider
	mounts    map[string]*MountStatus
}

// NewNativeMountService creates a new native mount service
func NewNativeMountService() *NativeMountService {
	return &NativeMountService{
		providers: make(map[string]MountProvider),
		mounts:    make(map[string]*MountStatus),
	}
}

// RegisterProvider registers a cloud storage provider
func (s *NativeMountService) RegisterProvider(provider MountProvider) {
	s.providers[provider.Name()] = provider
}

// Mount mounts a cloud storage
func (s *NativeMountService) Mount(ctx context.Context, config *MountConfig) error {
	provider, ok := s.providers[config.Provider]
	if !ok {
		return fmt.Errorf("unknown provider: %s", config.Provider)
	}
	
	if err := provider.Mount(ctx, config); err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}
	
	s.mounts[config.MountPath] = &MountStatus{
		Provider:  config.Provider,
		MountPath: config.MountPath,
		Status:    "mounted",
		LastSync:  time.Now(),
	}
	
	return nil
}

// Unmount unmounts a cloud storage
func (s *NativeMountService) Unmount(ctx context.Context, mountPath string) error {
	status, ok := s.mounts[mountPath]
	if !ok {
		return fmt.Errorf("mount not found: %s", mountPath)
	}
	
	provider, ok := s.providers[status.Provider]
	if !ok {
		return fmt.Errorf("provider not found: %s", status.Provider)
	}
	
	if err := provider.Unmount(ctx, mountPath); err != nil {
		return fmt.Errorf("unmount failed: %w", err)
	}
	
	delete(s.mounts, mountPath)
	return nil
}

// ListMounts returns all current mounts
func (s *NativeMountService) ListMounts() []*MountStatus {
	result := make([]*MountStatus, 0, len(s.mounts))
	for _, status := range s.mounts {
		result = append(result, status)
	}
	return result
}

// GetStatus returns the status of a specific mount
func (s *NativeMountService) GetStatus(ctx context.Context, mountPath string) (*MountStatus, error) {
	status, ok := s.mounts[mountPath]
	if !ok {
		return nil, fmt.Errorf("mount not found: %s", mountPath)
	}
	
	provider, ok := s.providers[status.Provider]
	if !ok {
		return status, nil
	}
	
	currentStatus, err := provider.Status(ctx, mountPath)
	if err != nil {
		status.Status = "error"
		status.Error = err.Error()
		return status, nil
	}
	
	return currentStatus, nil
}

// SupportedProviders returns list of supported cloud storage providers
func (s *NativeMountService) SupportedProviders() []string {
	providers := make([]string, 0, len(s.providers))
	for name := range s.providers {
		providers = append(providers, name)
	}
	return providers
}