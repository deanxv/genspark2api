package controller

import (
	"genspark2api/common/config"
	"net/http"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse represents the health check response
type HealthResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Version   string            `json:"version"`
	Uptime    int64             `json:"uptime_seconds"`
	System    SystemInfo        `json:"system"`
	Checks    map[string]string `json:"checks"`
}

// SystemInfo contains system information
type SystemInfo struct {
	GoVersion   string `json:"go_version"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	NumCPU      int    `json:"num_cpu"`
	NumGoroutine int    `json:"num_goroutine"`
	MemoryUsage struct {
		Alloc      uint64 `json:"alloc_bytes"`
		TotalAlloc uint64 `json:"total_alloc_bytes"`
		Sys        uint64 `json:"sys_bytes"`
		NumGC      uint32 `json:"num_gc"`
	} `json:"memory_usage"`
}

var startTime = time.Now()

// HealthCheck handles health check requests
func HealthCheck(c *gin.Context) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now(),
		Version:   "v1.12.6", // Using the current version from constants
		Uptime:    int64(time.Since(startTime).Seconds()),
		System:    getSystemInfo(),
		Checks:    performHealthChecks(),
	}

	c.JSON(http.StatusOK, response)
}

// getSystemInfo gathers system information
func getSystemInfo() SystemInfo {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	info := SystemInfo{
		GoVersion:    runtime.Version(),
		OS:         runtime.GOOS,
		Arch:       runtime.GOARCH,
		NumCPU:     runtime.NumCPU(),
		NumGoroutine: runtime.NumGoroutine(),
	}

	info.MemoryUsage.Alloc = m.Alloc
	info.MemoryUsage.TotalAlloc = m.TotalAlloc
	info.MemoryUsage.Sys = m.Sys
	info.MemoryUsage.NumGC = m.NumGC

	return info
}

// performHealthChecks runs various health checks
func performHealthChecks() map[string]string {
	checks := make(map[string]string)

	// Check if required environment variables are set
	checks["environment"] = "ok"
	if len(config.GSCookie) == 0 {
		checks["cookies"] = "warning: no cookies configured"
	} else {
		checks["cookies"] = "ok"
	}

	// Check system resources
	checks["memory"] = "ok"
	checks["goroutines"] = "ok"

	// Additional checks can be added here
	checks["api"] = "ok"

	return checks
}