// Package media provides media library management functionality
package media

import "sync"

// Cache provides in-memory caching for media files and metadata
type Cache struct {
	files    map[string]*VideoFile
	metadata map[string]interface{}
	mu       sync.RWMutex
}

// NewCache creates a new cache instance
func NewCache() *Cache {
	return &Cache{
		files:    make(map[string]*VideoFile),
		metadata: make(map[string]interface{}),
	}
}

// GetFile retrieves a cached video file by path
func (c *Cache) GetFile(path string) (*VideoFile, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	file, ok := c.files[path]
	return file, ok
}

// SetFile stores a video file in the cache
func (c *Cache) SetFile(path string, file *VideoFile) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files[path] = file
}

// RemoveFile removes a video file from the cache
func (c *Cache) RemoveFile(path string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.files, path)
}

// GetAllFiles returns all cached video files
func (c *Cache) GetAllFiles() []*VideoFile {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	files := make([]*VideoFile, 0, len(c.files))
	for _, f := range c.files {
		files = append(files, f)
	}
	return files
}

// GetFileByID retrieves a cached video file by its ID
func (c *Cache) GetFileByID(id string) (*VideoFile, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	for _, f := range c.files {
		if f.ID == id {
			return f, true
		}
	}
	return nil, false
}

// GetMetadata retrieves cached metadata by key
func (c *Cache) GetMetadata(key string) (interface{}, bool) {
	if c == nil {
		return nil, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	meta, ok := c.metadata[key]
	return meta, ok
}

// SetMetadata stores metadata in the cache
func (c *Cache) SetMetadata(key string, meta interface{}) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metadata[key] = meta
}

// RemoveMetadata removes metadata from the cache
func (c *Cache) RemoveMetadata(key string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.metadata, key)
}

// Clear clears all cached data
func (c *Cache) Clear() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.files = make(map[string]*VideoFile)
	c.metadata = make(map[string]interface{})
}

// Size returns the number of cached files and metadata entries
func (c *Cache) Size() (files int, metadata int) {
	if c == nil {
		return 0, 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.files), len(c.metadata)
}
