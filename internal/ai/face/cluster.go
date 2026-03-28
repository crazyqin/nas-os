package face

import (
	"context"
	"sync"
)

// Cluster manages face clustering for person identification
type Cluster struct {
	persons    map[string]*Person
	faceQueue  chan *Face
	threshold  float64
	processing bool
	mu         sync.RWMutex
}

// ClusterConfig for face clustering
type ClusterConfig struct {
	Threshold   float64
	QueueSize   int
	AutoCluster bool
}

// NewCluster creates a new face cluster manager
func NewCluster(cfg ClusterConfig) *Cluster {
	queueSize := cfg.QueueSize
	if queueSize <= 0 {
		queueSize = 1000
	}
	
	c := &Cluster{
		persons:   make(map[string]*Person),
		faceQueue: make(chan *Face, queueSize),
		threshold: cfg.Threshold,
	}
	
	if cfg.AutoCluster {
		c.StartProcessing()
	}
	
	return c
}

// StartProcessing starts background clustering
func (c *Cluster) StartProcessing() {
	c.mu.Lock()
	if c.processing {
		c.mu.Unlock()
		return
	}
	c.processing = true
	c.mu.Unlock()
	
	go c.processQueue()
}

// StopProcessing stops background clustering
func (c *Cluster) StopProcessing() {
	c.mu.Lock()
	c.processing = false
	c.mu.Unlock()
}

// processQueue processes incoming faces
func (c *Cluster) processQueue() {
	for {
		c.mu.RLock()
		running := c.processing
		c.mu.RUnlock()
		
		if !running {
			return
		}
		
		select {
		case face := <-c.faceQueue:
			c.clusterFace(face)
		default:
			// No faces in queue, wait
			continue
		}
	}
}

// AddFace adds a face to clustering queue
func (c *Cluster) AddFace(face *Face) error {
	select {
	case c.faceQueue <- face:
		return nil
	default:
		return ErrQueueFull
	}
}

// clusterFace clusters a single face
func (c *Cluster) clusterFace(face *Face) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if len(face.Embedding) == 0 {
		// Cannot cluster without embedding
		return
	}
	
	// Find best matching person
	bestMatch := ""
	bestScore := c.threshold
	
	for personID, person := range c.persons {
		if len(person.AvgEmbedding) == 0 {
			continue
		}
		
		score := cosineSimilarity(face.Embedding, person.AvgEmbedding)
		if score > bestScore {
			bestScore = score
			bestMatch = personID
		}
	}
	
	if bestMatch != "" {
		// Add to existing person
		c.persons[bestMatch].FaceCount++
		c.persons[bestMatch].Photos = append(c.persons[bestMatch].Photos, face.ImagePath)
		face.PersonID = bestMatch
		
		// Update average embedding
		c.updatePersonEmbedding(bestMatch, face.Embedding)
	} else {
		// Create new person
		personID := generatePersonID()
		c.persons[personID] = &Person{
			ID:           personID,
			Name:         "Unknown",
			FaceCount:    1,
			AvgEmbedding: face.Embedding,
			Photos:       []string{face.ImagePath},
		}
		face.PersonID = personID
	}
}

// updatePersonEmbedding updates person's average embedding
func (c *Cluster) updatePersonEmbedding(personID string, newEmbedding []float64) {
	person := c.persons[personID]
	if len(person.AvgEmbedding) != len(newEmbedding) {
		return
	}
	
	// Weighted average (weight old embedding more for stability)
	n := float64(person.FaceCount)
	for i := range person.AvgEmbedding {
		person.AvgEmbedding[i] = (person.AvgEmbedding[i] * (n - 1) + newEmbedding[i]) / n
	}
}

// GetPerson retrieves a person by ID
func (c *Cluster) GetPerson(id string) (*Person, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	person, exists := c.persons[id]
	if !exists {
		return nil, ErrPersonNotFound
	}
	return person, nil
}

// GetAllPersons returns all identified persons
func (c *Cluster) GetAllPersons() []*Person {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	result := make([]*Person, 0, len(c.persons))
	for _, person := range c.persons {
		result = append(result, person)
	}
	return result
}

// RenamePerson renames a person
func (c *Cluster) RenamePerson(id string, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	person, exists := c.persons[id]
	if !exists {
		return ErrPersonNotFound
	}
	person.Name = name
	return nil
}

// MergePersons merges two persons (source into target)
func (c *Cluster) MergePersons(sourceID, targetID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	source, exists := c.persons[sourceID]
	if !exists {
		return ErrPersonNotFound
	}
	
	target, exists := c.persons[targetID]
	if !exists {
		return ErrPersonNotFound
	}
	
	// Merge data
	target.FaceCount += source.FaceCount
	target.Photos = append(target.Photos, source.Photos...)
	
	// Merge embeddings (weighted average)
	if len(target.AvgEmbedding) > 0 && len(source.AvgEmbedding) > 0 {
		totalFaces := target.FaceCount + source.FaceCount
		sourceWeight := float64(source.FaceCount) / float64(totalFaces)
		targetWeight := float64(target.FaceCount) / float64(totalFaces)
		
		for i := range target.AvgEmbedding {
			target.AvgEmbedding[i] = target.AvgEmbedding[i] * targetWeight + 
			                          source.AvgEmbedding[i] * sourceWeight
		}
	}
	
	// Remove source person
	delete(c.persons, sourceID)
	
	return nil
}

// DeletePerson removes a person (faces become unassigned)
func (c *Cluster) DeletePerson(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	if _, exists := c.persons[id]; !exists {
		return ErrPersonNotFound
	}
	
	delete(c.persons, id)
	return nil
}

// SetThreshold adjusts clustering threshold
func (c *Cluster) SetThreshold(threshold float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.threshold = threshold
}

// GetStats returns clustering statistics
func (c *Cluster) GetStats() ClusterStats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	totalFaces := 0
	for _, person := range c.persons {
		totalFaces += person.FaceCount
	}
	
	return ClusterStats{
		TotalPersons: len(c.persons),
		TotalFaces:   totalFaces,
		QueueSize:    len(c.faceQueue),
		Threshold:    c.threshold,
	}
}

// ClusterStats holds clustering statistics
type ClusterStats struct {
	TotalPersons int     `json:"total_persons"`
	TotalFaces   int     `json:"total_faces"`
	QueueSize    int     `json:"queue_size"`
	Threshold    float64 `json:"threshold"`
}

// Errors
var (
	ErrQueueFull     = errors.New("face queue is full")
	ErrPersonNotFound = errors.New("person not found")
)

import "errors"