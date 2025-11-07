# Go Microservices - Market Data & Events Platform

[![Go Version](https://img.shields.io/badge/Go-1.25.3-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

A **distributed microservices architecture** built in Go for real-time market data processing and event management. This project demonstrates modern microservices patterns including service separation, database per service, and event-driven architecture.

## 🏗️ Architecture Overview

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Market Data   │    │     Events      │    │   ClickHouse    │
│   Ingestion     │    │   Ingestion     │    │    Database     │
│   Service       │    │   Service       │    │                 │
│   (Port 8081)   │    │   (Port 8080)   │    │  (Ports 8123,   │
└─────────────────┘    └─────────────────┘    │      9000)      │
         │                       │             └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
    ┌─────────────────┐         │         ┌─────────────────┐
    │   Market Data   │         │         │     Events      │
    │     Query       │         │         │     Query       │
    │   Service       │         │         │   Service       │
    │   (Port 8091)   │         │         │   (Port 8090)   │
    └─────────────────┘         │         └─────────────────┘
                                 │
              ┌─────────────────┐
              │  Market Poller  │
              │   Background    │
              │   Service       │
              └─────────────────┘
```

### 🎯 What is a Microservice?
A **microservice** is a small, independent service that:
- Handles **one specific business function** (like "storing events" or "querying market data")
- Can be **deployed independently** 
- Communicates with other services via **HTTP APIs**
- Has its **own database/data storage**

### 🔑 Key Microservices Principles Demonstrated

1. **Single Responsibility**: Each service handles one domain (events OR market data)
2. **Database Per Service**: Each service owns its database tables
3. **API-First Communication**: Services talk via REST APIs
4. **Independent Deployment**: Each service can be updated separately
5. **Fault Isolation**: If one service fails, others continue working

## 📋 Services Overview

| Service | Port | Responsibility | Endpoints |
|---------|------|----------------|-----------|
| **ingest-events** | 8080 | Store event logs | `POST /ingest/events` |
| **ingest-marketdata** | 8081 | Store market data | `POST /ingest/marketdata` |
| **query-events** | 8090 | Retrieve events | `GET /query/events` |
| **query-marketdata** | 8091 | Retrieve market data | `GET /query/marketdata` |
| **market-poller** | - | Background data polling | No HTTP endpoints |
| **internal-transformer** | - | Data transformation | No HTTP endpoints |

## 🚀 Quick Start

### Prerequisites
- Docker & Docker Compose
- Go 1.25+ (for local development)

### 1. Clone and Setup
```bash
git clone <repository-url>
cd go-microservices

# Create environment file
cp .env.example .env  # Edit with your settings
```

### 2. Start All Services
```bash
# Start the entire microservices stack
docker-compose up -d

# View logs from all services
docker-compose logs -f

# Check service health
docker-compose ps
```

### 3. Test the Services
```bash
# Test event ingestion
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '[{
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO", 
    "source": "trading-engine",
    "message": "Order executed successfully",
    "context": {"orderId": "12345", "symbol": "AAPL"}
  }]'

# Query stored events  
curl http://localhost:8090/query/events

# Test market data ingestion
curl -X POST http://localhost:8081/ingest/marketdata \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "request_id": "test-123",
      "time_in_millis": 1640995800000,
      "token_data": {
        "AAPL": {
          "timestamp": "1640995800",
          "lastPrice": 150.25,
          "volume": 1000000
        }
      }
    },
    "success": true
  }'

# Query market data
curl http://localhost:8091/query/marketdata
```

## 📚 API Documentation

### Events Service

#### POST /ingest/events
**Purpose**: Store application events and logs
**Port**: 8080

```bash
# Single event
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "ERROR",
    "source": "payment-service", 
    "message": "Payment processing failed",
    "context": {"userId": "123", "amount": "99.99"}
  }'

# Multiple events (batch)
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '[
    {
      "timestamp": "2024-01-15T10:30:00Z",
      "level": "INFO",
      "source": "user-service",
      "message": "User logged in", 
      "context": {"userId": "456"}
    },
    {
      "timestamp": "2024-01-15T10:31:00Z",
      "level": "WARN",
      "source": "inventory-service",
      "message": "Low stock alert",
      "context": {"productId": "ABC123", "remaining": "5"}
    }
  ]'
```

**Response**: `202 Accepted` on success

#### GET /query/events
**Purpose**: Retrieve stored events
**Port**: 8090

```bash
curl http://localhost:8090/query/events
```

**Response**: 
```json
[
  {
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO",
    "source": "trading-engine", 
    "message": "Order executed",
    "context": {"orderId": "12345"}
  }
]
```

### Market Data Service

#### POST /ingest/marketdata  
**Purpose**: Store real-time market data from external APIs
**Port**: 8081

```bash
curl -X POST http://localhost:8081/ingest/marketdata \
  -H "Content-Type: application/json" \
  -d '{
    "data": {
      "request_id": "upstox-req-789",
      "time_in_millis": 1640995800000,
      "token_data": {
        "AAPL": {
          "timestamp": "1640995800",
          "lastTradeTime": "1640995800", 
          "lastPrice": 150.25,
          "closePrice": 149.50,
          "volume": 1000000,
          "netChange": 0.75,
          "ohlc": {
            "open": 149.00,
            "high": 151.00, 
            "low": 148.50,
            "close": 150.25,
            "volume": 1000000
          }
        }
      }
    },
    "success": true
  }'
```

#### GET /query/marketdata
**Purpose**: Retrieve market data
**Port**: 8091

```bash
curl http://localhost:8091/query/marketdata
```

## 🛠️ Development Guide

### Adding New Endpoints

Want to add a new endpoint? Here's how microservices make it easy:

#### 1. Choose the Right Service
- **Events-related?** → Add to `cmd/query-events/main.go` or `cmd/ingest-events/main.go`
- **Market data-related?** → Add to `cmd/query-marketdata/main.go` or `cmd/ingest-marketdata/main.go`  
- **New domain?** → Create a new service (see "Adding New Services")

#### 2. Add Route Handler
```go
// In the appropriate service's main.go
func main() {
    // Existing code...
    
    // Add your new endpoint
    http.HandleFunc("/your/new/endpoint", app.handleYourNewFeature)
    
    // Existing code...
}

// Add handler function
func (app *App) handleYourNewFeature(w http.ResponseWriter, r *http.Request) {
    // Your endpoint logic here
    // - Parse request
    // - Query/update database  
    // - Return response
}
```

#### 3. Update Documentation
- Add endpoint to this README
- Update docker-compose.yml if needed
- Test the new endpoint

### Adding New Services

Need a completely new service? Follow the microservices pattern:

#### 1. Create Service Structure
```bash
mkdir cmd/your-new-service
touch cmd/your-new-service/main.go
```

#### 2. Implement Service
```go
// cmd/your-new-service/main.go
package main

import (
    "database/sql"
    "log"
    "net/http"
    "os"
    
    "github.com/rajindersingh041/go-microservices/internal/database"
)

// Define your service's database schema
const initYourTableSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.your_table (
    id Int64,
    name String,
    created_at DateTime
) ENGINE = MergeTree()
ORDER BY id;
`

type App struct {
    db *sql.DB
}

func main() {
    // Connect to database
    host := os.Getenv("CLICKHOUSE_HOST")
    conn, err := database.Connect(host)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Initialize YOUR service's schema (database-per-service pattern)
    if _, err := conn.Exec(initYourTableSQL); err != nil {
        log.Fatalf("Failed to init schema: %v", err)
    }

    app := &App{db: conn}
    
    // Define your endpoints
    http.HandleFunc("/your/endpoint", app.handleYourEndpoint)
    
    // Use unique port (increment from existing services)
    port := ":8092"  
    log.Printf("Starting your service on %s...", port)
    http.ListenAndServe(port, nil)
}

func (app *App) handleYourEndpoint(w http.ResponseWriter, r *http.Request) {
    // Your business logic
}
```

#### 3. Add to Build Pipeline
```dockerfile
# Add to Dockerfile
RUN CGO_ENABLED=0 go build -o /bin/your-new-service ./cmd/your-new-service/main.go

# Add to final stage  
COPY --from=builder /bin/your-new-service /bin/your-new-service
```

#### 4. Add to Docker Compose
```yaml
# Add to docker-compose.yml
your-new-service:
  build: .
  command: /bin/your-new-service
  ports: ["8092:8092"]
  env_file: [".env"]
  depends_on:
    clickhouse-server: { condition: service_healthy }
```

### Scaling Patterns

#### Horizontal Scaling
```yaml
# Scale a service to multiple instances
docker-compose up --scale query-events=3 -d

# Use load balancer (nginx, traefik) to distribute traffic
```

#### Database Scaling
- **Read Replicas**: Add read-only ClickHouse replicas for query services
- **Sharding**: Partition data across multiple ClickHouse instances
- **Caching**: Add Redis for frequently accessed data

#### Service Mesh
For production, consider:
- **Istio** or **Linkerd** for service-to-service communication
- **API Gateway** (Kong, Ambassador) for external traffic
- **Service Discovery** (Consul, etcd) for dynamic service registration

## 🏛️ Project Structure

```
go-microservices/
├── cmd/                          # Microservices (one per directory)
│   ├── ingest-events/           # Event ingestion service
│   ├── ingest-marketdata/       # Market data ingestion service  
│   ├── query-events/            # Event query service
│   ├── query-marketdata/        # Market data query service
│   ├── market-poller/           # Background polling service
│   ├── internal-transformer/    # Data transformation service
│   └── on-demand-fetcher/       # On-demand data fetcher
├── internal/                    # Shared libraries
│   ├── database/               # Database connection logic
│   └── models/                 # Data models
├── docker-compose.yml          # Service orchestration
├── Dockerfile                  # Multi-service container build
└── README.md                   # This documentation
```

### Why This Structure?
- **`cmd/`**: Each subdirectory = one microservice (industry standard)
- **`internal/`**: Shared code between services (not exported outside this module)
- **Single Dockerfile**: Builds all services (simpler for development, can be split later)
- **Docker Compose**: Orchestrates all services and dependencies

## 🔧 Configuration

### Environment Variables (.env file)
```env
# Database Configuration
CLICKHOUSE_HOST=clickhouse-server
CLICKHOUSE_USER=myuser
CLICKHOUSE_PASSWORD=mypassword  
CLICKHOUSE_DB=mydatabase

# Market Poller Configuration
POLLER_INSTRUMENTS=NSE_EQ|INE009A01021,BSE_EQ|500325
UPSTOX_BASE_URL=https://api.upstox.com
POLLER_INGEST_MARKET_URL=http://ingest-marketdata:8081/ingest/marketdata
POLLER_INGEST_EVENTS_URL=http://ingest-events:8080/ingest/events
POLLER_INTERVAL=30s
POLLER_TIMEZONE=Asia/Kolkata
POLLER_START_TIME=09:15
POLLER_END_TIME=15:30
```

### Service Configuration
Each service:
- **Reads environment variables** for configuration
- **Connects to ClickHouse** using shared database package
- **Exposes HTTP endpoints** on unique ports
- **Handles one specific domain** (events OR market data)

## 🧪 Testing

### Manual Testing
```bash
# Test all services are running
curl -f http://localhost:8080/health || echo "Events ingest down"
curl -f http://localhost:8081/health || echo "Market ingest down"  
curl -f http://localhost:8090/health || echo "Events query down"
curl -f http://localhost:8091/health || echo "Market query down"

# Test data flow
curl -X POST http://localhost:8080/ingest/events -d '{"timestamp":"2024-01-15T10:30:00Z","level":"INFO","source":"test","message":"hello","context":{}}'
curl http://localhost:8090/query/events
```

### Load Testing
```bash
# Install Apache Bench
apt-get install apache2-utils

# Test event ingestion performance
ab -n 1000 -c 10 -p event.json -T application/json http://localhost:8080/ingest/events

# Test query performance  
ab -n 1000 -c 10 http://localhost:8090/query/events
```

## 🚨 Troubleshooting

### Common Issues

#### Services Won't Start
```bash
# Check logs
docker-compose logs ingest-events
docker-compose logs clickhouse-server

# Check if ports are available
netstat -tlnp | grep :8080
```

#### Database Connection Issues
```bash
# Test ClickHouse connection
docker exec -it clickhouse-server clickhouse-client -u myuser --password mypassword

# Check database exists
docker exec -it clickhouse-server clickhouse-client -u myuser --password mypassword -q "SHOW DATABASES"
```

#### Service Communication Issues
```bash
# Test service-to-service communication
docker exec -it go-microservices_ingest-events_1 curl http://query-events:8090/query/events
```

## 🎓 Learning Resources

### Microservices Concepts
- [Microservices.io](https://microservices.io/) - Patterns and best practices
- [Martin Fowler - Microservices](https://martinfowler.com/articles/microservices.html)

### Go Development  
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Web Examples](https://gowebexamples.com/)

### ClickHouse
- [ClickHouse Documentation](https://clickhouse.com/docs/)
- [ClickHouse Go Driver](https://github.com/ClickHouse/clickhouse-go)

## 📈 Production Considerations

### Monitoring & Observability  
- **Prometheus** + **Grafana** for metrics
- **Jaeger** or **Zipkin** for distributed tracing  
- **ELK Stack** for centralized logging

### Security
- **API Authentication** (JWT tokens)
- **TLS/SSL** for service communication
- **Network policies** to restrict service access
- **Secrets management** (Vault, K8s secrets)

### High Availability
- **Multiple replicas** of each service
- **Circuit breakers** for fault tolerance
- **Health checks** and **graceful shutdowns**
- **Database clustering** and **backups**

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Follow the microservices patterns shown above
4. Add tests for new functionality
5. Update documentation
6. Create a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

**Built with ❤️ using Go, ClickHouse, and Docker**