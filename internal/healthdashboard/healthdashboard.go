// Package healthdashboard provides storage health monitoring and alerting.
package healthdashboard

import (
	"github.com/gin-gonic/gin"
)

// Module represents the health dashboard module.
type Module struct {
	collector *Collector
	handlers  *Handlers
}

// NewModule creates a new health dashboard module.
func NewModule() *Module {
	collector := NewCollector()
	handlers := NewHandlers(collector)

	return &Module{
		collector: collector,
		handlers:  handlers,
	}
}

// RegisterRoutes registers all health dashboard routes.
func (m *Module) RegisterRoutes(r *gin.RouterGroup) {
	m.handlers.RegisterRoutes(r)
}

// GetCollector returns the module's collector for external data updates.
func (m *Module) GetCollector() *Collector {
	return m.collector
}

// GetHandlers returns the module's handlers.
func (m *Module) GetHandlers() *Handlers {
	return m.handlers
}
