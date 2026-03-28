package photos

// PhotoMetadata contains metadata for a photo
type PhotoMetadata struct {
	ID        string        `json:"id"`
	Path      string        `json:"path"`
	DateTaken *time.Time    `json:"date_taken,omitempty"`
	Location  *Location     `json:"location,omitempty"`
	Camera    *CameraInfo   `json:"camera,omitempty"`
	Faces     []FaceInfo    `json:"faces,omitempty"`
	Objects   []ObjectInfo  `json:"objects,omitempty"`
	Tags      []string      `json:"tags,omitempty"`
	Size      int64         `json:"size"`
	Width     int           `json:"width"`
	Height    int           `json:"height"`
}

// Location represents photo location info
type Location struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

// CameraInfo represents camera information
type CameraInfo struct {
	Model    string `json:"model"`
	Make     string `json:"make"`
	Lens     string `json:"lens"`
	ISO      int    `json:"iso"`
	FNumber  string `json:"f_number"`
}

// FaceInfo represents detected face
type FaceInfo struct {
	ID       string `json:"id"`
	PersonID string `json:"person_id"`
	X        int    `json:"x"`
	Y        int    `json:"y"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
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

import "time"