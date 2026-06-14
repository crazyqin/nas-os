package healthpredictor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewPredictor(t *testing.T) {
	config := DefaultConfig()
	p := NewPredictor(config)
	assert.NotNil(t, p)
}

func TestNewAnomalyDetector(t *testing.T) {
	config := DefaultConfig()
	d := NewAnomalyDetector(config)
	assert.NotNil(t, d)
}

func TestNewHealer(t *testing.T) {
	config := DefaultConfig()
	h := NewHealer(config)
	assert.NotNil(t, h)
}

func TestNewReporter(t *testing.T) {
	r := NewReporter(100)
	assert.NotNil(t, r)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.CollectInterval > 0)
	assert.True(t, config.MaxHistorySize > 0)
}
