# Health Check Feature

## Overview
Added a new health check endpoint to monitor the service status and system health.

## Usage

### Health Check Endpoint
```
GET /health
```

### Response Format
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

### Features
- **Service Status**: Shows if the service is healthy
- **Uptime Tracking**: Displays how long the service has been running
- **System Information**: Provides Go runtime details and memory usage
- **Health Checks**: Monitors various system components
- **Version Information**: Shows the current API version

### Benefits
1. **Monitoring**: Easy integration with monitoring tools
2. **Debugging**: Quick system status overview
3. **Uptime Tracking**: Monitor service availability
4. **Resource Monitoring**: Track memory and goroutine usage
5. **Cookie Status**: Monitor configured cookies status

### Integration Examples

#### cURL
```bash
curl -X GET http://localhost:7055/health
```

#### Python
```python
import requests
response = requests.get('http://localhost:7055/health')
health_data = response.json()
print(f"Status: {health_data['status']}")
print(f"Uptime: {health_data['uptime_seconds']} seconds")
```

#### JavaScript
```javascript
fetch('http://localhost:7055/health')
  .then(response => response.json())
  .then(data => {
    console.log('Service status:', data.status);
    console.log('Uptime:', data.uptime_seconds, 'seconds');
  });
```