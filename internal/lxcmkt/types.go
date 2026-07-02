package lxcmkt

import "time"

// Template represents an LXC container template.
type Template struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Distro      string            `json:"distro"`  // ubuntu, debian, alpine, centos, fedora
	Version     string            `json:"version"` // e.g., "22.04", "12", "3.19"
	Arch        string            `json:"arch"`    // amd64, arm64, armhf
	Tags        []string          `json:"tags"`
	ImageURL    string            `json:"image_url"`
	Size        int64             `json:"size"` // bytes
	Downloads   int64             `json:"downloads"`
	Rating      float64           `json:"rating"` // 0-5
	RatingCount int               `json:"rating_count"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Config      TemplateConfig    `json:"config"`
	Versions    []TemplateVersion `json:"versions"`
}

// TemplateConfig represents template customization options.
type TemplateConfig struct {
	Hostname      string            `json:"hostname,omitempty"`
	Network       NetworkConfig     `json:"network"`
	Storage       []StorageMapping  `json:"storage,omitempty"`
	StartupScript string            `json:"startup_script,omitempty"`
	Environment   map[string]string `json:"environment,omitempty"`
	Resources     ResourceLimits    `json:"resources"`
	Privileged    bool              `json:"privileged"`
	Features      []string          `json:"features,omitempty"` // nesting, fuse, etc.
}

// NetworkConfig represents container network configuration.
type NetworkConfig struct {
	Interface string   `json:"interface"`        // eth0, etc.
	Type      string   `json:"type"`             // bridge, macvlan, physical
	Bridge    string   `json:"bridge,omitempty"` // lxdbr0, etc.
	IP        string   `json:"ip,omitempty"`     // static IP or empty for DHCP
	Gateway   string   `json:"gateway,omitempty"`
	DNS       []string `json:"dns,omitempty"`
	MTU       int      `json:"mtu,omitempty"`
	VLAN      int      `json:"vlan,omitempty"`
}

// StorageMapping represents host-to-container path mapping.
type StorageMapping struct {
	HostPath      string `json:"host_path"`
	ContainerPath string `json:"container_path"`
	ReadOnly      bool   `json:"read_only"`
	Size          string `json:"size,omitempty"` // e.g., "10GB"
}

// ResourceLimits represents container resource constraints.
type ResourceLimits struct {
	CPUs    string `json:"cpus,omitempty"`   // e.g., "2" or "50%"
	Memory  string `json:"memory,omitempty"` // e.g., "512MB", "2GB"
	Swap    string `json:"swap,omitempty"`
	Disk    string `json:"disk,omitempty"`     // e.g., "10GB"
	ProcMax int    `json:"proc_max,omitempty"` // max processes
}

// TemplateVersion represents a specific version of a template.
type TemplateVersion struct {
	Version   string    `json:"version"`
	ImageURL  string    `json:"image_url"`
	Checksum  string    `json:"checksum"`
	Size      int64     `json:"size"`
	ReleaseAt time.Time `json:"release_at"`
	Changelog string    `json:"changelog,omitempty"`
}

// DeployRequest represents a request to deploy an LXC container.
type DeployRequest struct {
	TemplateID string         `json:"template_id" binding:"required"`
	Version    string         `json:"version,omitempty"` // empty = latest
	Name       string         `json:"name" binding:"required"`
	Customize  TemplateConfig `json:"customize,omitempty"`
	AutoStart  bool           `json:"auto_start"`
}

// DeployResponse represents the response after deploying a container.
type DeployResponse struct {
	ContainerID   string    `json:"container_id"`
	ContainerName string    `json:"container_name"`
	TemplateID    string    `json:"template_id"`
	TemplateVer   string    `json:"template_version"`
	Status        string    `json:"status"` // creating, running, stopped, error
	IP            string    `json:"ip,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// SearchQuery represents template search parameters.
type SearchQuery struct {
	Query     string   `form:"q"`
	Distro    string   `form:"distro"`
	Arch      string   `form:"arch"`
	Tags      []string `form:"tags"`
	MinRating float64  `form:"min_rating"`
	SortBy    string   `form:"sort_by"` // name, rating, downloads, created_at
	Page      int      `form:"page"`
	PageSize  int      `form:"page_size"`
}

// SearchResults represents paginated search results.
type SearchResults struct {
	Templates  []Template `json:"templates"`
	Total      int        `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// RatingRequest represents a user rating submission.
type RatingRequest struct {
	TemplateID string `json:"template_id" binding:"required"`
	Score      int    `json:"score" binding:"required,min=1,max=5"`
	Comment    string `json:"comment,omitempty"`
}

// RatingResponse represents the updated rating info.
type RatingResponse struct {
	TemplateID  string  `json:"template_id"`
	AverageRate float64 `json:"average_rate"`
	TotalRates  int     `json:"total_rates"`
}

// StatsResponse represents template statistics.
type StatsResponse struct {
	TotalTemplates int            `json:"total_templates"`
	TotalDownloads int64          `json:"totalDownloads"`
	TopDistro      map[string]int `json:"top_distro"`
	TopArch        map[string]int `json:"top_arch"`
}
