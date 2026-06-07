package pxe

import "time"

// PXEServer represents the PXE/TFTP server configuration
type PXEServer struct {
	IP         string `json:"ip"`
	SubnetMask string `json:"subnet_mask"`
	Gateway    string `json:"gateway"`
	DNS        string `json:"dns"`
	BootFile   string `json:"boot_file"`
	Interface  string `json:"interface"`
	Enabled    bool   `json:"enabled"`
	Status     string `json:"status"` // running, stopped, error
}

// PXEClient represents a PXE boot client
type PXEClient struct {
	MACAddress string    `json:"mac_address"`
	IP         string    `json:"ip"`
	Hostname   string    `json:"hostname"`
	LastBoot   time.Time `json:"last_boot"`
	Status     string    `json:"status"` // online, offline, booting
	OSImage    string    `json:"os_image"`
}

// PXEImage represents a bootable OS image
type PXEImage struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Type      string    `json:"type"` // linux, windows, rescue
	CreatedAt time.Time `json:"created_at"`
}

// BootMenuItem represents a single entry in the PXE boot menu
type BootMenuItem struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	ImageID string `json:"image_id"`
	Kernel  string `json:"kernel"`
	Initrd  string `json:"initrd,omitempty"`
	Cmdline string `json:"cmdline,omitempty"`
	Default bool   `json:"default"`
}

// PXEConfig represents the full PXE service configuration
type PXEConfig struct {
	TFTPPath  string         `json:"tftp_path"`
	HTTPPath  string         `json:"http_path"`
	DHCPRange string         `json:"dhcp_range"`
	BootMenu  []BootMenuItem `json:"boot_menu"`
	LogLevel  string         `json:"log_level"`
}

// CreatePXEConfigRequest is the request body for updating PXE config
type CreatePXEConfigRequest struct {
	TFTPPath  *string `json:"tftp_path,omitempty"`
	HTTPPath  *string `json:"http_path,omitempty"`
	DHCPRange *string `json:"dhcp_range,omitempty"`
	LogLevel  *string `json:"log_level,omitempty"`
}

// UpdateClientRequest is the request body for updating a PXE client
type UpdateClientRequest struct {
	Hostname *string `json:"hostname,omitempty"`
	OSImage  *string `json:"os_image,omitempty"`
	Status   *string `json:"status,omitempty"`
}

// PXEStats holds aggregated PXE service statistics
type PXEStats struct {
	TotalClients    int     `json:"total_clients"`
	ActiveClients   int     `json:"active_clients"`
	TotalImages     int     `json:"total_images"`
	BootSuccessRate float64 `json:"boot_success_rate"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message,omitempty"`
}

// SuccessResponse represents a generic success
type SuccessResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}
