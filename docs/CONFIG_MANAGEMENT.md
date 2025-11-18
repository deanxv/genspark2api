# Configuration Management Guide

## Overview

The Genspark2API now includes a comprehensive configuration management system that allows runtime configuration changes without restarting the service. This feature provides:

- **Runtime Configuration Updates**: Modify configuration values without service restart
- **Configuration History**: Track all configuration changes with audit trail
- **Validation**: Built-in validation for configuration values
- **Rollback**: Reset to default configuration values
- **Admin Security**: Secure admin-only access to configuration endpoints

## 🚀 Quick Start

### 1. Enable Admin Access

Set the `ADMIN_KEY` environment variable:

```bash
# Single admin key
export ADMIN_KEY=your-secret-admin-key

# Multiple admin keys (comma-separated)
export ADMIN_KEY=admin-key-1,admin-key-2,admin-key-3
```

### 2. Access Configuration Endpoints

All configuration endpoints are available under `/admin/*` and require admin authentication.

## 🔧 Configuration Endpoints

### Get Current Configuration
```http
GET /admin/config
X-Admin-Key: your-admin-key
```

**Response:**
```json
{
  "status": "success",
  "config": {
    "rate_limit_rps": 60,
    "rate_limit_burst": 100,
    "max_request_size": 10485760,
    "request_timeout": 30,
    "cache_enabled": true,
    "cache_ttl": 300,
    "cache_max_size": 1000,
    "security_headers": true,
    "cors_origins": ["*"],
    "log_level": "info",
    "log_requests": true,
    "log_responses": false,
    "metrics_enabled": true,
    "validation_enabled": true,
    "debug_mode": false,
    "default_model": "gpt-4o",
    "max_tokens": 4096,
    "temperature": 0.7,
    "worker_pool_size": 10,
    "max_concurrent": 100,
    "queue_size": 1000,
    "health_check_interval": 30,
    "health_check_timeout": 5
  },
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### Update Configuration
```http
PUT /admin/config
X-Admin-Key: your-admin-key
Content-Type: application/json

{
  "key": "RateLimitRPS",
  "value": 120,
  "description": "Increased rate limit for high traffic"
}
```

**Available Configuration Keys:**
- `RateLimitRPS` - Rate limit requests per second (int, min: 1, max: 1000)
- `RateLimitBurst` - Rate limit burst size (int, min: 1, max: 2000)
- `MaxRequestSize` - Maximum request size in bytes (int, min: 1024, max: 100MB)
- `RequestTimeout` - Request timeout in seconds (int, min: 5, max: 300)
- `CacheEnabled` - Enable/disable caching (bool)
- `CacheTTL` - Cache TTL in seconds (int, min: 60, max: 86400)
- `CacheMaxSize` - Maximum cache entries (int, min: 100, max: 10000)
- `SecurityHeaders` - Enable security headers (bool)
- `CORSOrigins` - Allowed CORS origins (array of strings)
- `LogLevel` - Log level (string: "debug", "info", "warn", "error")
- `LogRequests` - Log incoming requests (bool)
- `LogResponses` - Log response data (bool)
- `MetricsEnabled` - Enable metrics collection (bool)
- `ValidationEnabled` - Enable request validation (bool)
- `DebugMode` - Enable debug mode (bool)
- `DefaultModel` - Default AI model (string)
- `MaxTokens` - Maximum tokens per request (int, min: 1, max: 32768)
- `Temperature` - Default temperature (float, min: 0.0, max: 2.0)
- `WorkerPoolSize` - Worker pool size (int, min: 1, max: 100)
- `MaxConcurrent` - Maximum concurrent requests (int, min: 1, max: 1000)
- `QueueSize` - Request queue size (int, min: 100, max: 10000)
- `HealthCheckInterval` - Health check interval in seconds (int, min: 10, max: 300)
- `HealthCheckTimeout` - Health check timeout in seconds (int, min: 1, max: 60)

### Get Configuration History
```http
GET /admin/config/history?limit=20
X-Admin-Key: your-admin-key
```

**Response:**
```json
{
  "status": "success",
  "history": [
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "user": "admin",
      "action": "update",
      "key": "RateLimitRPS",
      "old_value": 60,
      "new_value": 120,
      "description": "Increased rate limit for high traffic"
    },
    {
      "timestamp": "2024-01-15T09:15:00Z",
      "user": "admin",
      "action": "update",
      "key": "CacheTTL",
      "old_value": 300,
      "new_value": 600,
      "description": "Extended cache TTL"
    }
  ],
  "count": 2
}
```

### Reset Configuration to Defaults
```http
POST /admin/config/reset
X-Admin-Key: your-admin-key
Content-Type: application/json

{
  "description": "Reset to defaults after performance testing"
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Configuration reset to defaults",
  "description": "Reset to defaults after performance testing"
}
```

## 🔒 Security

### Admin Authentication
- All admin endpoints require the `X-Admin-Key` header
- Admin keys can be configured via the `ADMIN_KEY` environment variable
- Multiple admin keys are supported (comma-separated)
- If `ADMIN_KEY` is not set, admin access is disabled by default

### Best Practices
1. **Use Strong Admin Keys**: Generate cryptographically secure random keys
2. **Rotate Keys Regularly**: Change admin keys periodically
3. **Limit Access**: Only expose admin endpoints to internal networks
4. **Monitor Usage**: Log all configuration changes for audit purposes
5. **Backup Configurations**: Export configurations before major changes

## 📝 Environment Variables

### Required
```bash
ADMIN_KEY=your-secret-admin-key  # Enable admin access
```

### Optional
```bash
# Multiple admin keys (comma-separated)
ADMIN_KEY=admin-key-1,admin-key-2,admin-key-3

# Disable admin authentication (not recommended for production)
# ADMIN_KEY=  # Leave empty to disable admin auth
```

## 🛠️ Common Use Cases

### 1. Performance Tuning
```bash
# Increase rate limits for high-traffic periods
curl -X PUT "http://localhost:7055/admin/config" \
  -H "X-Admin-Key: your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "RateLimitRPS",
    "value": 200,
    "description": "Increased for marketing campaign"
  }'
```

### 2. Debug Mode
```bash
# Enable debug mode for troubleshooting
curl -X PUT "http://localhost:7055/admin/config" \
  -H "X-Admin-Key: your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "DebugMode",
    "value": true,
    "description": "Enable debug mode for issue investigation"
  }'
```

### 3. Cache Management
```bash
# Adjust cache settings for optimal performance
curl -X PUT "http://localhost:7055/admin/config" \
  -H "X-Admin-Key: your-admin-key" \
  -H "Content-Type: application/json" \
  -d '{
    "key": "CacheTTL",
    "value": 1800,
    "description": "Extended cache TTL for stable responses"
  }'
```

## 🔍 Monitoring

### Configuration Change Alerts
Monitor the logs for configuration changes:
```bash
tail -f /var/log/genspark2api.log | grep "Configuration"
```

### Health Check with Configuration Status
```http
GET /health
```

The health endpoint now includes configuration status in the response.

## 🚨 Troubleshooting

### 401 Unauthorized
- **Cause**: Invalid or missing admin key
- **Solution**: Check `X-Admin-Key` header and `ADMIN_KEY` environment variable

### 400 Bad Request
- **Cause**: Invalid configuration value
- **Solution**: Check the configuration key name and value type/range

### Configuration Not Applied
- **Cause**: Configuration validation failed
- **Solution**: Check logs for validation errors

### Admin Endpoints Not Accessible
- **Cause**: `ADMIN_KEY` not set
- **Solution**: Set `ADMIN_KEY` environment variable and restart service

## 📚 API Reference

All configuration endpoints follow RESTful conventions:

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/admin/config` | Get current configuration |
| PUT | `/admin/config` | Update configuration value |
| GET | `/admin/config/history` | Get configuration history |
| POST | `/admin/config/reset` | Reset to default values |

For more information, see the main API documentation.