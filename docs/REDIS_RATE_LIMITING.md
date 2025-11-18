# Redis-backed Rate Limiting Guide

## Overview

Genspark2API now includes advanced Redis-backed rate limiting that provides distributed rate limiting across multiple instances. This feature replaces the memory-based rate limiting with a scalable, Redis-based solution.

## 🚀 Features

- **Distributed Rate Limiting**: Works across multiple API instances
- **Sliding Window Algorithm**: Precise rate limiting with sliding windows
- **Multiple Rate Limit Strategies**: IP-based, API key-based, endpoint-based
- **Real-time Statistics**: Monitor rate limit usage and statistics
- **Dynamic Configuration**: Update rate limits without restart
- **Graceful Fallback**: Falls back to memory-based rate limiting if Redis is unavailable

## 🔧 Configuration

### Environment Variables

```bash
# Redis connection settings
REDIS_ADDR=localhost:6379      # Redis server address
REDIS_PASSWORD=your-password   # Redis password (optional)
REDIS_DB=0                     # Redis database number (default: 0)

# Rate limiting settings (these can be updated via config API)
REQUEST_RATE_LIMIT=60           # Requests per minute (default: 60)
```

### Redis Connection

The system will automatically detect if Redis is available:
- If `REDIS_ADDR` is set → Uses Redis-backed rate limiting
- If `REDIS_ADDR` is not set → Falls back to memory-based rate limiting

## 📊 Rate Limiting Strategies

### 1. IP-based Rate Limiting (Default)
Limits requests per IP address:
```
Key format: rate_limit:ip:192.168.1.1
Default: 60 requests per minute per IP
```

### 2. API Key-based Rate Limiting
Limits requests per API key (when using Authorization header):
```
Key format: rate_limit:api_key:abc123def456
```

### 3. Endpoint-based Rate Limiting
Different limits for different endpoints:
```
# Chat completions: Higher limit
rate_limit:endpoint:/v1/chat/completions:192.168.1.1

# Image generation: Lower limit  
rate_limit:endpoint:/v1/images/generations:192.168.1.1
```

## 🔍 API Endpoints

### Rate Limit Statistics
```http
GET /admin/rate-limit/stats
X-Admin-Key: your-admin-key
```

**Response:**
```json
{
  "status": "success",
  "stats": {
    "total_requests": 1523,
    "blocked_requests": 12,
    "current_rates": {
      "192.168.1.1": {
        "key": "rate_limit:ip:192.168.1.1",
        "current_count": 45,
        "limit": 60,
        "window": "1m",
        "reset_time": 1642345678
      }
    },
    "redis_connected": true,
    "last_update": "2024-01-15T10:30:00Z"
  }
}
```

### Clear Rate Limit
```http
POST /admin/rate-limit/clear?key=rate_limit:ip:192.168.1.1
X-Admin-Key: your-admin-key
```

**Response:**
```json
{
  "status": "success",
  "message": "Rate limit cleared",
  "key": "rate_limit:ip:192.168.1.1"
}
```

### Configure Rate Limit
```http
PUT /admin/rate-limit/config
X-Admin-Key: your-admin-key
Content-Type: application/json

{
  "endpoint": "/v1/chat/completions",
  "requests": 100,
  "window": 60
}
```

**Response:**
```json
{
  "status": "success",
  "message": "Rate limit configuration updated",
  "endpoint": "/v1/chat/completions",
  "requests": 100,
  "window_seconds": 60
}
```

### Redis Status
```http
GET /admin/redis/status
X-Admin-Key: your-admin-key
```

**Response:**
```json
{
  "status": "success",
  "redis": {
    "enabled": true,
    "connected": true,
    "addr": "localhost:6379",
    "db": 0,
    "pool_size": 100
  }
}
```

## 📈 Rate Limit Headers

Every response includes rate limit information headers:

```http
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 15
X-RateLimit-Reset: 1642345740
```

When rate limit is exceeded:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 45
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
```

## 🛠️ Redis Setup

### Docker Setup
```bash
# Run Redis with Docker
docker run -d --name redis \
  -p 6379:6379 \
  -e REDIS_PASSWORD=your-password \
  redis:7-alpine

# Or with docker-compose
version: '3.8'
services:
  redis:
    image: redis:7-alpine
    container_name: redis
    ports:
      - "6379:6379"
    environment:
      - REDIS_PASSWORD=your-password
    volumes:
      - redis-data:/data
volumes:
  redis-data:
```

### Production Redis Setup
```bash
# Install Redis on Ubuntu/Debian
sudo apt update
sudo apt install redis-server

# Configure Redis
sudo nano /etc/redis/redis.conf
# Set: requirepass your-password
# Set: maxmemory 256mb
# Set: maxmemory-policy allkeys-lru

# Restart Redis
sudo systemctl restart redis-server
sudo systemctl enable redis-server
```

## 🔒 Security Considerations

### Redis Security
- Use strong passwords for Redis authentication
- Enable SSL/TLS for Redis connections in production
- Restrict Redis access to internal networks
- Monitor Redis access logs

### Rate Limit Security
- Different rate limits for authenticated vs unauthenticated requests
- Stricter limits for expensive operations (image/video generation)
- Monitor for rate limit bypass attempts
- Log all rate limit violations

## 📊 Monitoring and Alerting

### Key Metrics to Monitor
```
rate_limit_requests_total        # Total requests
rate_limit_blocked_total         # Blocked requests
rate_limit_redis_errors_total    # Redis connection errors
rate_limit_config_updates_total  # Configuration changes
```

### Prometheus Metrics
The system exposes these metrics for monitoring:
```
# HELP genspark2api_rate_limit_requests_total Total rate limit requests
# TYPE genspark2api_rate_limit_requests_total counter
genspark2api_rate_limit_requests_total{strategy="ip",status="allowed"} 1523
genspark2api_rate_limit_requests_total{strategy="ip",status="blocked"} 12

# HELP genspark2api_rate_limit_redis_connected Redis connection status
# TYPE genspark2api_rate_limit_redis_connected gauge
genspark2api_rate_limit_redis_connected 1
```

## 🚨 Troubleshooting

### Redis Connection Issues
```bash
# Check Redis connectivity
redis-cli -h localhost -p 6379 ping

# Check Redis logs
docker logs redis
# or
tail -f /var/log/redis/redis-server.log
```

### Rate Limit Not Working
1. Check if Redis is enabled: `GET /admin/redis/status`
2. Verify rate limit configuration: `GET /admin/config`
3. Check Redis connection logs
4. Ensure rate limiting middleware is enabled in router

### Performance Issues
- Monitor Redis memory usage
- Check for Redis connection pool exhaustion
- Review rate limit key expiration settings
- Consider Redis clustering for high-load scenarios

## 🔄 Migration from Memory-based Rate Limiting

### Automatic Migration
The system automatically detects Redis availability:
- If `REDIS_ADDR` is set → Uses Redis rate limiting
- If `REDIS_ADDR` is not set → Uses memory-based rate limiting

### Gradual Migration
1. Deploy Redis infrastructure
2. Set `REDIS_ADDR` environment variable
3. Restart Genspark2API service
4. Monitor rate limiting behavior
5. No code changes required!

## 🎯 Best Practices

### Rate Limit Configuration
```bash
# Conservative limits for public endpoints
REQUEST_RATE_LIMIT=30  # 30 requests per minute

# Higher limits for internal endpoints
# Configure via API: 100 requests per minute for chat completions

# Strict limits for expensive operations
# Configure via API: 10 requests per minute for video generation
```

### Redis Optimization
```bash
# Set appropriate memory limits
redis-cli config set maxmemory 256mb
redis-cli config set maxmemory-policy allkeys-lru

# Monitor Redis performance
redis-cli --latency
redis-cli info stats
```

### High Availability
- Use Redis Sentinel for failover
- Consider Redis Cluster for scaling
- Implement circuit breakers for Redis failures
- Monitor Redis replication lag

## 📚 Configuration Reference

### Environment Variables
```bash
REDIS_ADDR=localhost:6379        # Redis server address
REDIS_PASSWORD=your-password   # Redis password
REDIS_DB=0                       # Redis database number
REQUEST_RATE_LIMIT=60          # Default rate limit per minute
```

### API Configuration
```json
{
  "endpoint": "/v1/chat/completions",
  "requests": 100,
  "window": 60
}
```

### Rate Limit Keys
```
rate_limit:ip:192.168.1.1                    # IP-based
rate_limit:api_key:abc123def456                # API key-based
rate_limit:endpoint:/v1/chat/completions:ip   # Endpoint-based
```

## 🎉 Summary

The Redis-backed rate limiting system provides:
- ✅ **Distributed Rate Limiting** across multiple instances
- ✅ **Sliding Window Algorithm** for precise control
- ✅ **Multiple Rate Limit Strategies** (IP, API key, endpoint)
- ✅ **Real-time Statistics** and monitoring
- ✅ **Dynamic Configuration** without restart
- ✅ **Graceful Fallback** to memory-based limiting
- ✅ **Production-ready** with security and monitoring

This implementation significantly improves the scalability and reliability of rate limiting in distributed deployments while maintaining backward compatibility with the existing memory-based system.