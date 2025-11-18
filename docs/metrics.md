# Metrics and Monitoring System

## Overview

The Genspark2API now includes a comprehensive metrics collection and monitoring system that provides detailed insights into service performance, request patterns, and system health.

## Features

### Real-time Metrics Collection
- **Request Tracking**: Total requests, success/error rates, endpoint usage
- **Performance Monitoring**: Response times, request counts per minute
- **Model Usage Analytics**: Track which AI models are being used most frequently
- **System Resource Monitoring**: Memory usage, peak memory consumption
- **Response Time Analysis**: Average response times per endpoint

### New API Endpoints

#### 1. Health Check Endpoint
```
GET /health
```
Returns comprehensive service health status including:
- Service status (healthy/unhealthy)
- System information (Go version, OS, architecture)
- Memory usage statistics
- Uptime tracking
- Component health checks

#### 2. Metrics Endpoint
```
GET /metrics
```
Returns detailed performance metrics:
- Total request counts and success rates
- Average response times
- Requests per minute
- Top 10 most used models
- Peak memory usage
- Active model count

#### 3. Reset Metrics Endpoint
```
POST /metrics/reset
```
Resets all collected metrics to zero. Useful for:
- Performance testing
- Starting fresh monitoring periods
- Troubleshooting

## Response Examples

### Health Check Response
```json
{
  "status": "healthy",
  "timestamp": "2025-11-18T12:00:00Z",
  "version": "v1.12.6",
  "uptime_seconds": 3600,
  "system": {
    "go_version": "go1.23.2",
    "os": "linux",
    "arch": "amd64",
    "num_cpu": 4,
    "num_goroutine": 12,
    "memory_usage": {
      "alloc_bytes": 5242880,
      "total_alloc_bytes": 10485760,
      "sys_bytes": 8388608,
      "num_gc": 5
    }
  },
  "checks": {
    "environment": "ok",
    "cookies": "ok",
    "memory": "ok",
    "goroutines": "ok",
    "api": "ok"
  }
}
```

### Metrics Response
```json
{
  "status": "success",
  "timestamp": "2025-11-18T12:00:00Z",
  "version": "v1.12.6",
  "uptime_seconds": 3600,
  "metrics": {
    "total_requests": 15420,
    "success_rate": 98.5,
    "average_response_time_ms": 245.3,
    "requests_per_minute": 257,
    "active_models": 15,
    "peak_memory_usage_mb": 128
  },
  "top_models": [
    {
      "model": "gpt-4o",
      "count": 5234,
      "percentage": 33.9
    },
    {
      "model": "claude-3-7-sonnet",
      "count": 4123,
      "percentage": 26.7
    }
  ]
}
```

## Usage Examples

### cURL Commands

#### Check Health Status
```bash
curl -X GET http://localhost:7055/health
```

#### Get Metrics Data
```bash
curl -X GET http://localhost:7055/metrics
```

#### Reset Metrics
```bash
curl -X POST http://localhost:7055/metrics/reset
```

### Python Integration
```python
import requests
import json

# Check service health
response = requests.get('http://localhost:7055/health')
health_data = response.json()
print(f"Service Status: {health_data['status']}")
print(f"Uptime: {health_data['uptime_seconds']} seconds")

# Get performance metrics
response = requests.get('http://localhost:7055/metrics')
metrics_data = response.json()
print(f"Total Requests: {metrics_data['metrics']['total_requests']}")
print(f"Success Rate: {metrics_data['metrics']['success_rate']}%")
```

### JavaScript Integration
```javascript
// Check health status
fetch('http://localhost:7055/health')
  .then(response => response.json())
  .then(data => {
    console.log('Service status:', data.status);
    console.log('Uptime:', data.uptime_seconds, 'seconds');
  });

// Get metrics
fetch('http://localhost:7055/metrics')
  .then(response => response.json())
  .then(data => {
    console.log('Total requests:', data.metrics.total_requests);
    console.log('Success rate:', data.metrics.success_rate + '%');
  });
```

## Benefits

### For Operations Teams
- **Proactive Monitoring**: Early detection of performance issues
- **Resource Planning**: Memory usage tracking for capacity planning
- **Performance Analysis**: Response time trends and bottleneck identification
- **Service Reliability**: Health checks for service availability monitoring

### For Developers
- **Debugging Support**: Detailed request logging and error tracking
- **Performance Optimization**: Response time analysis per endpoint
- **Usage Analytics**: Model popularity and usage patterns
- **System Health**: Real-time monitoring of system resources

### For Business Users
- **Usage Insights**: Most popular AI models and usage trends
- **Performance Metrics**: Service reliability and response times
- **Resource Utilization**: System efficiency and optimization opportunities

## Configuration

The metrics system is automatically enabled and requires no additional configuration. The following middleware components are automatically applied:

- **MetricsMiddleware**: Collects request metrics and response times
- **RequestLoggingMiddleware**: Provides detailed request logging

Both middleware components are applied to all API routes and provide:
- Request/response logging with timing information
- X-Response-Time header for debugging
- Automatic model detection from request parameters
- Memory usage monitoring every 100 requests

## Integration with Monitoring Tools

The metrics endpoints are designed to integrate with popular monitoring tools:

### Prometheus Integration
The JSON responses can be easily converted to Prometheus metrics format for use with:
- Prometheus + Grafana dashboards
- Datadog, New Relic, or other APM tools
- Custom monitoring solutions

### Health Check Integration
The health endpoint can be used with:
- Load balancers for health checks
- Container orchestration platforms (Kubernetes, Docker Swarm)
- Uptime monitoring services
- Alert systems for service availability

## Performance Impact

The metrics collection system is designed for minimal performance impact:
- Efficient memory usage with circular buffers
- Configurable sampling rates
- Non-blocking metric collection
- Automatic cleanup of old data
- Optimized data structures for fast access

## Future Enhancements

Potential future improvements to the metrics system:
- Custom metric collection intervals
- Metric export to external systems
- Advanced alerting based on thresholds
- Historical data persistence
- Custom dashboard integration
- Performance benchmarking tools