package dam

import "time"

// AssetType 资产类型
type AssetType string

const (
	AssetImage    AssetType = "image"
	AssetVideo    AssetType = "video"
	AssetAudio    AssetType = "audio"
	AssetDocument AssetType = "document"
	AssetArchive  AssetType = "archive"
	Asset3D       AssetType = "3d"
	AssetFont     AssetType = "font"
	AssetCode     AssetType = "code"
	AssetOther    AssetType = "other"
)

// AssetStatus 资产状态
type AssetStatus string

const (
	StatusDraft     AssetStatus = "draft"
	StatusReview    AssetStatus = "review"
	StatusApproved  AssetStatus = "approved"
	StatusPublished AssetStatus = "published"
	StatusArchived  AssetStatus = "archived"
	StatusDeleted   AssetStatus = "deleted"
)

// AccessLevel 访问级别
type AccessLevel string

const (
	AccessPublic  AccessLevel = "public"
	AccessPrivate AccessLevel = "private"
	AccessTeam    AccessLevel = "team"
	AccessShared  AccessLevel = "shared"
)

// DigitalAsset 数字资产
type DigitalAsset struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Description  string      `json:"description"`
	Type         AssetType   `json:"type"`
	Status       AssetStatus `json:"status"`
	Access       AccessLevel `json:"access"`
	FilePath     string      `json:"file_path"`
	ThumbnailPath string    `json:"thumbnail_path,omitempty"`
	PreviewPath  string      `json:"preview_path,omitempty"`
	MimeType     string      `json:"mime_type"`
	FileSize     int64       `json:"file_size"`
	Checksum     string      `json:"checksum"`
	Width        int         `json:"width,omitempty"`
	Height       int         `json:"height,omitempty"`
	Duration     int         `json:"duration,omitempty"`
	Bitrate      int         `json:"bitrate,omitempty"`
	Format       string      `json:"format,omitempty"`
	ColorSpace   string      `json:"color_space,omitempty"`
	Tags         []string    `json:"tags"`
	Categories   []string    `json:"categories"`
	Labels       []string    `json:"labels"`
	Collections  []string    `json:"collections"`
	Folder       string      `json:"folder"`
	OwnerID      string      `json:"owner_id"`
	OwnerName    string      `json:"owner_name"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	UploadedAt   time.Time   `json:"uploaded_at"`
	LastAccessed *time.Time  `json:"last_accessed,omitempty"`
	ViewCount    int         `json:"view_count"`
	DownloadCount int        `json:"download_count"`
	Rating       float64     `json:"rating"`
	IsFavorite   bool        `json:"is_favorite"`
	License      string      `json:"license,omitempty"`
	Copyright    string      `json:"copyright,omitempty"`
	Source       string      `json:"source,omitempty"`
	Author       string      `json:"author,omitempty"`
	CopyrightHolder string   `json:"copyright_holder,omitempty"`
	ExpiryDate   *time.Time  `json:"expiry_date,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	Versions     []AssetVersion `json:"versions,omitempty"`
	Relations    []AssetRelation `json:"relations,omitempty"`
	CustomFields map[string]interface{} `json:"custom_fields,omitempty"`
}

// AssetVersion 资产版本
type AssetVersion struct {
	ID          string    `json:"id"`
	Version     string    `json:"version"`
	FilePath    string    `json:"file_path"`
	FileSize    int64     `json:"file_size"`
	Checksum    string    `json:"checksum"`
	Comment     string    `json:"comment"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// AssetRelation 资产关系
type AssetRelation struct {
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
	Type       string `json:"type"`
}

// Collection 资产集合
type Collection struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	CoverImage  string    `json:"cover_image"`
	AssetCount  int       `json:"asset_count"`
	TotalSize   int64     `json:"total_size"`
	OwnerID     string    `json:"owner_id"`
	IsPublic    bool      `json:"is_public"`
	Tags        []string  `json:"tags"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Folder 文件夹
type Folder struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	ParentPath  string    `json:"parent_path"`
	AssetCount  int       `json:"asset_count"`
	SubFolders  int       `json:"sub_folders"`
	TotalSize   int64     `json:"total_size"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AITag AI 标签
type AITag struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Source     string  `json:"source"`
}

// AIAnalysis AI 分析结果
type AIAnalysis struct {
	AssetID     string   `json:"asset_id"`
	Tags        []AITag  `json:"tags"`
	Objects     []AIObject `json:"objects"`
	Faces       []AIFace `json:"faces"`
	Colors      []AIColor `json:"colors"`
	OCRText     string   `json:"ocr_text,omitempty"`
	Transcript  string   `json:"transcript,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Sentiment   string   `json:"sentiment,omitempty"`
	Language    string   `json:"language,omitempty"`
	NSFWScore   float64  `json:"nsfw_score"`
	QualityScore float64 `json:"quality_score"`
	BlurScore   float64  `json:"blur_score"`
	CreatedAt   time.Time `json:"created_at"`
}

// AIObject AI 检测对象
type AIObject struct {
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
	BBox       []int   `json:"bbox"`
}

// AIFace AI 检测人脸
type AIFace struct {
	PersonID   string  `json:"person_id,omitempty"`
	PersonName string  `json:"person_name,omitempty"`
	Confidence float64 `json:"confidence"`
	BBox       []int   `json:"bbox"`
	Age        int     `json:"age,omitempty"`
	Gender     string  `json:"gender,omitempty"`
	Emotion    string  `json:"emotion,omitempty"`
}

// AIColor AI 检测颜色
type AIColor struct {
	Hex        string  `json:"hex"`
	Name       string  `json:"name"`
	Percentage float64 `json:"percentage"`
}

// SearchFilter 搜索过滤器
type SearchFilter struct {
	Query      string    `json:"query,omitempty"`
	Type       AssetType `json:"type,omitempty"`
	Status     AssetStatus `json:"status,omitempty"`
	Access     AccessLevel `json:"access,omitempty"`
	Tags       []string  `json:"tags,omitempty"`
	Categories []string  `json:"categories,omitempty"`
	Collections []string `json:"collections,omitempty"`
	Folder     string    `json:"folder,omitempty"`
	OwnerID    string    `json:"owner_id,omitempty"`
	DateFrom   *time.Time `json:"date_from,omitempty"`
	DateTo     *time.Time `json:"date_to,omitempty"`
	SizeMin    int64     `json:"size_min,omitempty"`
	SizeMax    int64     `json:"size_max,omitempty"`
	RatingMin  float64   `json:"rating_min,omitempty"`
	SortBy     string    `json:"sort_by,omitempty"`
	SortOrder  string    `json:"sort_order,omitempty"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
}

// SearchResult 搜索结果
type SearchResult struct {
	Assets     []DigitalAsset `json:"assets"`
	Total      int            `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"page_size"`
	TotalPages int            `json:"total_pages"`
	Facets     SearchFacets   `json:"facets"`
}

// SearchFacets 搜索分面
type SearchFacets struct {
	Types       []FacetItem `json:"types"`
	Tags        []FacetItem `json:"tags"`
	Categories  []FacetItem `json:"categories"`
	Collections []FacetItem `json:"collections"`
	Owners      []FacetItem `json:"owners"`
}

// FacetItem 分面项
type FacetItem struct {
	Value string `json:"value"`
	Count int    `json:"count"`
}

// ShareLink 分享链接
type ShareLink struct {
	ID          string    `json:"id"`
	AssetID     string    `json:"asset_id"`
	Token       string    `json:"token"`
	Password    string    `json:"password,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	MaxDownloads int      `json:"max_downloads,omitempty"`
	DownloadCount int     `json:"download_count"`
	AllowDownload bool    `json:"allow_download"`
	AllowPreview bool    `json:"allow_preview"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
}

// Workflow 审批流程
type Workflow struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Steps       []WorkflowStep `json:"steps"`
	Enabled     bool           `json:"enabled"`
	CreatedAt   time.Time      `json:"created_at"`
}

// WorkflowStep 流程步骤
type WorkflowStep struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Assignees []string `json:"assignees"`
	Actions  []string `json:"actions"`
	Order    int      `json:"order"`
}

// DAMStats 资产统计
type DAMStats struct {
	TotalAssets    int64   `json:"total_assets"`
	TotalSize      int64   `json:"total_size"`
	ImageCount     int64   `json:"image_count"`
	VideoCount     int64   `json:"video_count"`
	AudioCount     int64   `json:"audio_count"`
	DocumentCount  int64   `json:"document_count"`
	OtherCount     int64   `json:"other_count"`
	TotalCollections int   `json:"total_collections"`
	TotalTags      int     `json:"total_tags"`
	StorageUsed    int64   `json:"storage_used"`
	StorageLimit   int64   `json:"storage_limit"`
}

// WatermarkConfig 水印配置
type WatermarkConfig struct {
	Enabled    bool   `json:"enabled"`
	Text       string `json:"text,omitempty"`
	ImagePath  string `json:"image_path,omitempty"`
	Position   string `json:"position"`
	Opacity    float64 `json:"opacity"`
	Scale      float64 `json:"scale"`
}

// TransformConfig 转换配置
type TransformConfig struct {
	Resize     *ResizeConfig  `json:"resize,omitempty"`
	Crop       *CropConfig    `json:"crop,omitempty"`
	Rotate     int            `json:"rotate,omitempty"`
	Flip       string         `json:"flip,omitempty"`
	Format     string         `json:"format,omitempty"`
	Quality    int            `json:"quality,omitempty"`
	Watermark  *WatermarkConfig `json:"watermark,omitempty"`
	Filters    []string       `json:"filters,omitempty"`
}

// ResizeConfig 调整大小配置
type ResizeConfig struct {
	Width   int  `json:"width"`
	Height  int  `json:"height"`
	Fit     string `json:"fit"`
	WithoutEnlargement bool `json:"without_enlargement"`
}

// CropConfig 裁剪配置
type CropConfig struct {
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
	Gravity string `json:"gravity,omitempty"`
}
