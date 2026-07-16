package web

import (
	"context"

	"nas-os/internal/arch"
)

// CoreHealthReport is the public health probe payload, aggregating Core module Health().
type CoreHealthReport struct {
	Code    int                  `json:"code"`
	Message string               `json:"message"`
	Data    CoreHealthReportData `json:"data"`
}

// CoreHealthReportData holds overall status and per-module Core health.
type CoreHealthReportData struct {
	Status  string             `json:"status"` // healthy | unhealthy
	Modules []CoreModuleHealth `json:"modules"`
}

// CoreModuleHealth is one Core module's health snapshot.
type CoreModuleHealth struct {
	Name    string `json:"name"`
	Tier    string `json:"tier"`
	Healthy bool   `json:"healthy"`
	Error   string `json:"error,omitempty"`
}

// AggregateCoreHealth evaluates Core-tier modules only and returns a report suitable
// for GET /api/v1/system/health. Non-core modules in the slice are ignored.
func AggregateCoreHealth(ctx context.Context, modules []arch.Module) CoreHealthReport {
	report := CoreHealthReport{
		Code:    0,
		Message: "healthy",
		Data: CoreHealthReportData{
			Status:  "healthy",
			Modules: make([]CoreModuleHealth, 0, len(modules)),
		},
	}
	if ctx == nil {
		ctx = context.Background()
	}

	unhealthy := 0
	for _, mod := range modules {
		if mod == nil {
			continue
		}
		if mod.Tier() != arch.ModuleTierCore {
			continue
		}
		entry := CoreModuleHealth{
			Name:    mod.Name(),
			Tier:    string(mod.Tier()),
			Healthy: true,
		}
		if err := mod.Health(ctx); err != nil {
			entry.Healthy = false
			entry.Error = err.Error()
			unhealthy++
		}
		report.Data.Modules = append(report.Data.Modules, entry)
	}

	if unhealthy > 0 {
		report.Code = 1
		report.Message = "unhealthy"
		report.Data.Status = "unhealthy"
	}
	return report
}
