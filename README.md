# Custom Layer 7 API Gateway & Load Balancer

## Project Overview

A high-performance and lightweight Layer 7 API Gateway built natively in Go. The gateway handles dynamic routing, intelligent load balancing, active health checking, and critical edge features like rate limiting, JWT authentication, and CORS management.

## Architecture

The gateway processes incoming requests through a strict middleware pipeline before routing them to healthy backend servers.

```mermaid
graph TD
    Client([Client]) --> Gateway
    
    subgraph Gateway [API Gateway Core]
        direction TB
        L[Logger & Metrics] --> C[CORS]
        C --> J[JWT Auth]
        J --> RL[Rate Limiter]
        RL --> RT[Router]
        
        RT --> |Longest-Prefix Match| P[Reverse Proxy]
        P --> |Retry Loop| CB[Circuit Breaker]
        CB --> B[Load Balancer]
    end
    
    B -->|NextServer| Backend1[(Backend 1)]
    B -->|NextServer| Backend2[(Backend 2)]
    B -->|NextServer| Backend3[(Backend 3)]
    
    HC[Background Health Checker] -.->|Poll interval| Backend1
    HC -.->|Poll interval| Backend2
    HC -.->|Poll interval| Backend3
    
    W[Config Watcher] -.->|fsnotify| Config[(config.json)]
    W -.->|Hot Reload| RT
```

## Quickstart

### Prerequisites
- [Go 1.22+](https://golang.org/doc/install)

### Running Locally

1. **Install dependencies:**
   ```bash
   go mod download
   ```
2. **Start the gateway:**
   ```bash
   go run cmd/gateway/main.go
   ```
   The gateway runs on `localhost:8080` (or the port specified in `config.json`).
3. **Hot-Reloading:**
   Modify `config.json` while the server is running to see changes automatically applied with zero downtime!

### Load Testing
A pre-configured `k6` script (`test.js`) is included to test the gateway's performance.
```bash
k6 run test.js
```

## Configuration Reference

The gateway is entirely configured via a `config.json` file. Changes to this file are automatically detected via `fsnotify` and reloaded with zero downtime.

```json
{
  "server": {
    "port": 8080,
    "read_timeout": "15s",
    "write_timeout": "15s",
    "tls": {
      "enabled": false,
      "cert_file": "",
      "key_file": ""
    }
  },
  "middleware": {
    "cors": {
      "allowed_origins": ["*"],
      "allowed_methods": ["GET", "POST", "PUT", "DELETE", "OPTIONS"],
      "allowed_headers": ["Content-Type", "Authorization"]
    },
    "jwt": {
      "secret": ""
    },
    "rate_limit": {
      "requests_per_second": 2,
      "burst": 5,
      "cleanup_interval": "1m",
      "ttl": "5m"
    }
  },
  "routes": [
    {
      "path_prefix": "/users",
      "strip_prefix": true,
      "balancer": "round_robin",
      "backends": [
        { "url": "https://jsonplaceholder.typicode.com", "weight": 1 },
        { "url": "https://dummyjson.com", "weight": 2 }
      ]
    },
    {
      "path_prefix": "/products",
      "strip_prefix": false,
      "balancer": "least_conn",
      "backends": [
        { "url": "https://api.restful-api.dev", "weight": 1 }
      ]
    }
  ],
  "health_check_path": "/health",
  "retry_max_attempts": 3,
  "retry_base_delay": "100ms",
  "retry_max_delay": "2s",
  "cb_failure_threshold": 3,
  "cb_success_threshold": 2,
  "cb_timeout": "5s"
}
```

## Benchmark Results

The gateway was benchmarked using [k6](https://k6.io/) simulating 100 concurrent virtual users over 10 seconds against an unthrottled local backend configuration to measure absolute maximum throughput.

| Metric | Result |
|--------|--------|
| **Throughput (RPS)** | `4,103 req/sec` |
| **Average Latency** | `24.3 ms` |
| **Success Rate (HTTP 200)** | `100%` |
| **Total Requests** | `41,103` |

*Note: In a separate saturation test with 500 concurrent virtual users, the gateway gracefully queued traffic while maintaining a 100% success rate (zero dropped connections), heavily proving its resilience under pressure.*

## Extending the Gateway

### Adding a New Balancer Strategy

1. **Implement the `Balancer` interface:**
   Create a new file in `internal/balancer/` and implement the required methods.
   ```go
   type Balancer interface {
       NextServer() (*health.Backend, error)
   }
   ```

2. **Register it:**
   Open `cmd/gateway/main.go` and add your algorithm to the `buildRoutesAndPoller` function:
   ```go
   if routeCfg.Balancer == "least_conn" {
       b = balancer.NewLeastConnections(routeBackends)
   } else if routeCfg.Balancer == "ip_hash" {
       b = balancer.NewIPHash(routeBackends) // <-- Your custom strategy
   } else {
       b = balancer.NewWeightedRoundRobin(routeBackends)
   }
   ```

3. **Update config:**
   Set `"balancer": "ip_hash"` in your `config.json`.
