package photos

import "time"

// PhotoMetadata contains metadata for a photo (used by conditional albums)
type PhotoMetadata struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	DateTaken *time.Time     `json:"date_taken,omitempty"`
	Location  *LocationInfo  `json:"location,omitempty"`
	Camera    *DeviceInfo    `json:"camera,omitempty"`
	Faces     []FaceInfo     `json:"faces,omitempty"`
	Objects   []ObjectInfo   `json:"objects,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	Size      int64          `json:"size"`
	Width     int            `json:"width"`
	Height    int            `json:"height"`
}

// ObjectInfo represents detected object
type ObjectInfo struct {
	Label    string  `json:"label"`
	Score    float64 `json:"score"`
	X        int     `json:"x"`
	Y        int     `json:"y"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
}

// AlbumStorage interface for album persistence
type AlbumStorage interface {
	SaveAlbum(album *ConditionalAlbum) error
	GetAllPhotos() ([]string, error)
	GetPhotoMetadata(id string) (*PhotoMetadata, error)
	DeleteAlbum(id string) error
}

// FaceDetector interface for face detection
type FaceDetector interface {
	DetectFaces(photoPath string) ([]FaceInfo, error)
	IdentifyPerson(faceID string) (string, error)
}