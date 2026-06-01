package storageml

import (
	"time"
)

// NewStorageMLManager creates a new StorageML manager
func NewStorageMLManager() *StorageML {
	ml := NewStorageML()
	return ml
}

// Start starts the collector
func (ml *StorageML) Start() {
	collector := NewCollector(ml, 5*time.Minute)
	collector.Start()
}

// Stop stops the collector
func (ml *StorageML) Stop() {
	// Collector stop is handled externally
}

// GetPredictor returns the predictor
func (ml *StorageML) GetPredictor() *Predictor {
	return NewPredictor(ml)
}

// GetCollector returns the collector
func (ml *StorageML) GetCollector() *Collector {
	return NewCollector(ml, 5*time.Minute)
}

// GetAnalyzer returns the analyzer
func (ml *StorageML) GetAnalyzer() *Analyzer {
	return NewAnalyzer(ml)
}

// CleanupOldData removes data points older than retention period
func (ml *StorageML) CleanupOldData() {
	ml.mu.Lock()
	defer ml.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -ml.retentionDays)
	for poolID, points := range ml.dataPoints {
		var filtered []DataPoint
		for _, dp := range points {
			if dp.Timestamp.After(cutoff) {
				filtered = append(filtered, dp)
			}
		}
		ml.dataPoints[poolID] = filtered
	}
}

// UnregisterPool removes a pool from monitoring
func (ml *StorageML) UnregisterPool(poolID string) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	delete(ml.poolConfigs, poolID)
	delete(ml.dataPoints, poolID)
}
