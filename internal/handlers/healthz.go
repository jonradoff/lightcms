package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/jonradoff/lightcms/v7/internal/build"
	"github.com/jonradoff/lightcms/v7/internal/services"
)

// healthStatus mirrors the vibectl VibeCtl Health Check Protocol status values.
type healthStatus string

const (
	healthStatusHealthy   healthStatus = "healthy"
	healthStatusDegraded  healthStatus = "degraded"
	healthStatusUnhealthy healthStatus = "unhealthy"
)

type healthDependency struct {
	Name    string       `json:"name"`
	Status  healthStatus `json:"status"`
	Message string       `json:"message,omitempty"`
}

type healthKPI struct {
	Name  string  `json:"name"`
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

// healthResponse matches the vibectl VibeCtl Health Check Protocol schema,
// with an additional "name" field to identify the software.
type healthResponse struct {
	Status       healthStatus       `json:"status"`
	Name         string             `json:"name"`
	Version      string             `json:"version,omitempty"`
	Uptime       int                `json:"uptime"`
	Dependencies []healthDependency `json:"dependencies"`
	KPIs         []healthKPI        `json:"kpis"`
}

var processStart = time.Now()

// Healthz serves GET /healthz in the vibectl VibeCtl Health Check Protocol format.
// The endpoint is unauthenticated — it is safe to expose publicly.
func (h *Handler) Healthz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp := healthResponse{
		Status:       healthStatusHealthy,
		Name:         "LightCMS",
		Version:      build.GetVersion(),
		Uptime:       int(time.Since(processStart).Seconds()),
		Dependencies: []healthDependency{},
		KPIs:         []healthKPI{},
	}

	// Check dependencies concurrently
	var mu sync.Mutex
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		dep := healthDependency{Name: "mongodb", Status: healthStatusHealthy}
		if err := h.db.Collection("content").Database().Client().Ping(ctx, nil); err != nil {
			dep.Status = healthStatusUnhealthy
			dep.Message = err.Error()
		}
		mu.Lock()
		resp.Dependencies = append(resp.Dependencies, dep)
		mu.Unlock()
	}()

	wg.Wait()

	for _, d := range resp.Dependencies {
		if d.Status == healthStatusUnhealthy {
			resp.Status = healthStatusUnhealthy
			break
		}
	}

	// Collect KPIs if analytics service is available
	if h.analyticsService != nil {
		resp.KPIs = []healthKPI{
			{Name: "dau", Value: float64(h.analyticsService.GetDAU(ctx)), Unit: "count"},
			{Name: "mau", Value: float64(h.analyticsService.GetMAU(ctx)), Unit: "count"},
			{Name: "content_created_today", Value: float64(h.analyticsService.GetContentCreatedToday(ctx)), Unit: "count"},
		}
	}

	code := http.StatusOK
	if resp.Status == healthStatusUnhealthy {
		code = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

// analyticsService getter to avoid import cycle — set via SetAnalyticsService.
func (h *Handler) SetAnalyticsService(a *services.AnalyticsService) {
	h.analyticsService = a
}
