package controller

import (
	"encoding/json"
	"genspark2api/common"
	"genspark2api/common/config"
	logger "genspark2api/common/loggger"
	"net/http"
	"runtime"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsCollector holds all metrics data
type MetricsCollector struct {
	mu sync.RWMutex

	// Request metrics
	TotalRequests    int64                     `json:"total_requests"`
	SuccessRequests  int64                     `json:"success_requests"`
	ErrorRequests    int64                     `json:"error_requests"`
	RequestCounts    map[string]int64          `json:"request_counts"`    // by endpoint
	ModelUsage       map[string]int64          `json:"model_usage"`       // by model name
	ResponseTimes    map[string][]float64      `json:"response_times"`    // by endpoint (ms)
	StatusCodeCounts map[int]int64              `json:"status_code_counts"`

	// Time-based metrics
	RequestsPerMinute []int64                   `json:"requests_per_minute"`
	LastResetTime     time.Time                 `json:"last_reset_time"`

	// System metrics
	MemorySnapshots []MemorySnapshot            `json:"memory_snapshots"`
	PeakMemoryUsage uint64                    `json:"peak_memory_usage"`
}

// MemorySnapshot represents memory usage at a point in time
type MemorySnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	Alloc       uint64    `json:"alloc_bytes"`
	TotalAlloc  uint64    `json:"total_alloc_bytes"`
	Sys         uint64    `json:"sys_bytes"`
	NumGC       uint32    `json:"num_gc"`
}

// MetricsResponse represents the API response
type MetricsResponse struct {
	Status          string                    `json:"status"`
	Timestamp       time.Time                 `json:"timestamp"`
	Version         string                    `json:"version"`
	UptimeSeconds   int64                     `json:"uptime_seconds"`
	Metrics         MetricsData               `json:"metrics"`
	TopModels       []ModelUsage              `json:"top_models"`
	RecentRequests  []RequestSnapshot         `json:"recent_requests"`
}

// MetricsData contains the core metrics
type MetricsData struct {
	TotalRequests    int64                     `json:"total_requests"`
	SuccessRate      float64                   `json:"success_rate"`
	AverageResponseTime float64                `json:"average_response_time_ms"`
	RequestsPerMinute int64                   `json:"requests_per_minute"`
	ActiveModels     int                       `json:"active_models"`
	PeakMemoryUsage  uint64                    `json:"peak_memory_usage_mb"`
}

// ModelUsage represents model usage statistics
type ModelUsage struct {
	Model     string  `json:"model"`
	Count     int64   `json:"count"`
	Percentage float64 `json:"percentage"`
}

// RequestSnapshot represents a recent request
type RequestSnapshot struct {
	Timestamp   time.Time `json:"timestamp"`
	Endpoint    string    `json:"endpoint"`
	Model       string    `json:"model,omitempty"`
	StatusCode  int       `json:"status_code"`
	ResponseTime float64  `json:"response_time_ms"`
	Success     bool      `json:"success"`
}

// Global metrics collector
var GlobalMetrics = NewMetricsCollector()

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		RequestCounts:     make(map[string]int64),
		ModelUsage:        make(map[string]int64),
		ResponseTimes:   make(map[string][]float64),
		StatusCodeCounts:  make(map[int]int64),
		RequestsPerMinute: make([]int64, 0),
		MemorySnapshots: make([]MemorySnapshot, 0),
		LastResetTime:     time.Now(),
	}
}

// RecordRequest records a new request
func (m *MetricsCollector) RecordRequest(endpoint, model string, statusCode int, responseTime float64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests++
	if success {
		m.SuccessRequests++
	} else {
		m.ErrorRequests++
	}

	// Record endpoint count
	m.RequestCounts[endpoint]++

	// Record model usage
	if model != "" {
		m.ModelUsage[model]++
	}

	// Record status code
	m.StatusCodeCounts[statusCode]++

	// Record response time
	m.ResponseTimes[endpoint] = append(m.ResponseTimes[endpoint], responseTime)

	// Keep only last 100 response times per endpoint
	if len(m.ResponseTimes[endpoint]) > 100 {
		m.ResponseTimes[endpoint] = m.ResponseTimes[endpoint][len(m.ResponseTimes[endpoint])-100:]
	}

	// Record memory snapshot every 100 requests
	if m.TotalRequests%100 == 0 {
		m.recordMemorySnapshot()
	}
}

// recordMemorySnapshot records current memory usage
func (m *MetricsCollector) recordMemorySnapshot() {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	snapshot := MemorySnapshot{
		Timestamp:  time.Now(),
		Alloc:      memStats.Alloc,
		TotalAlloc: memStats.TotalAlloc,
		Sys:        memStats.Sys,
		NumGC:      memStats.NumGC,
	}

	m.MemorySnapshots = append(m.MemorySnapshots, snapshot)
	
	// Update peak memory usage
	if memStats.Alloc > m.PeakMemoryUsage {
		m.PeakMemoryUsage = memStats.Alloc
	}

	// Keep only last 50 memory snapshots
	if len(m.MemorySnapshots) > 50 {
		m.MemorySnapshots = m.MemorySnapshots[len(m.MemorySnapshots)-50:]
	}
}

// GetSuccessRate calculates the success rate
func (m *MetricsCollector) GetSuccessRate() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.TotalRequests == 0 {
		return 0.0
	}
	return float64(m.SuccessRequests) / float64(m.TotalRequests) * 100
}

// GetAverageResponseTime calculates average response time
func (m *MetricsCollector) GetAverageResponseTime() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalResponseTime := 0.0
	count := 0

	for _, times := range m.ResponseTimes {
		for _, time := range times {
			totalResponseTime += time
			count++
		}
	}

	if count == 0 {
		return 0.0
	}
	return totalResponseTime / float64(count)
}

// GetTopModels returns top models by usage
func (m *MetricsCollector) GetTopModels(limit int) []ModelUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	type modelCount struct {
		model string
		count int64
	}

	var counts []modelCount
	totalModelUsage := int64(0)

	for model, count := range m.ModelUsage {
		counts = append(counts, modelCount{model, count})
		totalModelUsage += count
	}

	// Sort by count descending
	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Take top N
	if len(counts) > limit {
		counts = counts[:limit]
	}

	var result []ModelUsage
	for _, mc := range counts {
		percentage := 0.0
		if totalModelUsage > 0 {
			percentage = float64(mc.count) / float64(totalModelUsage) * 100
		}
		result = append(result, ModelUsage{
			Model:      mc.model,
			Count:      mc.count,
			Percentage: percentage,
		})
	}

	return result
}

// GetMetrics returns current metrics
func (m *MetricsCollector) GetMetrics() MetricsData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	peakMemoryMB := m.PeakMemoryUsage / (1024 * 1024)

	return MetricsData{
		TotalRequests:       m.TotalRequests,
		SuccessRate:         m.GetSuccessRate(),
		AverageResponseTime: m.GetAverageResponseTime(),
		RequestsPerMinute:   m.TotalRequests / int64(time.Since(m.LastResetTime).Minutes()),
		ActiveModels:        len(m.ModelUsage),
		PeakMemoryUsage:     peakMemoryMB,
	}
}

// ResetMetrics resets all metrics
func (m *MetricsCollector) ResetMetrics() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalRequests = 0
	m.SuccessRequests = 0
	m.ErrorRequests = 0
	m.RequestCounts = make(map[string]int64)
	m.ModelUsage = make(map[string]int64)
	m.ResponseTimes = make(map[string][]float64)
	m.StatusCodeCounts = make(map[int]int64)
	m.RequestsPerMinute = make([]int64, 0)
	m.MemorySnapshots = make([]MemorySnapshot, 0)
	m.PeakMemoryUsage = 0
	m.LastResetTime = time.Now()

	logger.SysLog("Metrics have been reset")
}

// MetricsHandler returns current metrics
func MetricsHandler(c *gin.Context) {
	metrics := GlobalMetrics.GetMetrics()
	topModels := GlobalMetrics.GetTopModels(10)

	response := MetricsResponse{
		Status:        "success",
		Timestamp:     time.Now(),
		Version:       "v1.12.6",
		UptimeSeconds: int64(time.Since(GlobalMetrics.LastResetTime).Seconds()),
		Metrics:       metrics,
		TopModels:     topModels,
		RecentRequests: getRecentRequests(), // This would need to be implemented
	}

	c.JSON(http.StatusOK, response)
}

// ResetMetricsHandler resets all metrics
func ResetMetricsHandler(c *gin.Context) {
	GlobalMetrics.ResetMetrics()
	c.JSON(http.StatusOK, gin.H{
		"status":    "success",
		"message":   "Metrics have been reset",
		"timestamp": time.Now(),
	})
}

// getRecentRequests would need to be implemented with a circular buffer
func getRecentRequests() []RequestSnapshot {
	// This is a placeholder - in a real implementation, you'd maintain a circular buffer
	// of recent requests for monitoring purposes
	return []RequestSnapshot{}
}

// init initializes the metrics system
func init() {
	logger.SysLog("Metrics system initialized")
}