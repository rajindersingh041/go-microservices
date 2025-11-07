# Microservices Architecture Guide

## 🎯 What Are Microservices?

**Microservices** are a way of building software applications as a collection of small, independent services that communicate over well-defined APIs. Think of it like a restaurant kitchen:

- **Monolith**: One chef does everything (cooking, plating, cleaning)
- **Microservices**: Specialized stations (grill chef, salad chef, dessert chef, dishwasher)

### Traditional Monolith vs Microservices

```
MONOLITH                          MICROSERVICES
┌─────────────────────┐          ┌──────┐ ┌──────┐ ┌──────┐
│                     │          │Service│ │Service│ │Service│
│  All Code in One    │   ──►    │   A   │ │   B   │ │   C   │
│  Application        │          │       │ │       │ │       │
│                     │          └──────┘ └──────┘ └──────┘
└─────────────────────┘               │       │       │
         │                           └───────┼───────┘
         │                                   │
    ┌─────────┐                         ┌─────────┐
    │Database │                         │Databases│
    └─────────┘                         └─────────┘
```

## 🏗️ Microservices Principles in This Project

### 1. Single Responsibility Principle
Each service has **one job**:

```
┌─────────────────┐    ┌─────────────────┐
│ ingest-events   │    │ query-events    │
│                 │    │                 │
│ Job: Store      │    │ Job: Retrieve   │
│ event data      │    │ event data      │
└─────────────────┘    └─────────────────┘
```

**Why it matters:**
- Easy to understand and maintain
- Teams can work independently  
- Bugs are isolated to specific functions

### 2. Database Per Service
Each service owns its data:

```
Service A ─── Database A
Service B ─── Database B  
Service C ─── Database C
```

**In our project:**
- `ingest-events` owns the `events` table
- `ingest-marketdata` owns the `market_data` table
- Services communicate via APIs, not shared database

**Benefits:**
- No database lock contention between services
- Each service can use the best database for its needs
- Schema changes don't break other services

### 3. API-First Communication
Services talk via HTTP APIs, never direct database access:

```
Client Request ───► Service A ───► Service B
                       │              │
                       ▼              ▼
                  Database A     Database B
```

### 4. Independent Deployment
Each service can be updated separately:

```bash
# Update only the events service
docker-compose up -d --no-deps ingest-events

# Update only the market data service  
docker-compose up -d --no-deps ingest-marketdata
```

## 🔄 Communication Patterns

### 1. Synchronous (HTTP/REST)
**When to use:** Real-time queries, immediate responses needed

```go
// Service A calls Service B synchronously
resp, err := http.Get("http://service-b:8090/query/events")
```

**Example in our project:**
- Market poller calls ingestion services
- Client queries call query services

### 2. Asynchronous (Message Queues)
**When to use:** High volume, fire-and-forget operations

```
Service A ───► Message Queue ───► Service B
              (Kafka/RabbitMQ)
```

**Not implemented yet, but useful for:**
- High-frequency market data updates
- Event sourcing patterns
- Handling traffic spikes

### 3. Event-Driven Architecture
Services publish events when something happens:

```
Order Service ───► "Order Created" Event ───► Inventory Service
                                         ───► Email Service  
                                         ───► Analytics Service
```

## 📊 Scaling Strategies

### 1. Horizontal Scaling (More Instances)
```yaml
# Scale query service to handle more reads
docker-compose up --scale query-events=3 -d
```

**Use when:**
- CPU/memory usage is high
- Response times are slow
- Traffic is increasing

### 2. Vertical Scaling (Bigger Instances)
```yaml
services:
  query-events:
    deploy:
      resources:
        limits:
          cpus: '2.0'    # More CPU
          memory: 4G     # More RAM
```

**Use when:**
- Single-threaded bottlenecks
- Memory-intensive operations
- Database connection limits

### 3. Database Scaling

#### Read Replicas
```yaml
services:
  clickhouse-master:
    image: clickhouse/clickhouse-server
  clickhouse-replica-1:
    image: clickhouse/clickhouse-server  
  clickhouse-replica-2:
    image: clickhouse/clickhouse-server
```

**Query services connect to replicas, ingest services to master**

#### Sharding (Partitioning)
```sql
-- Partition by date for time-series data
CREATE TABLE events_2024 AS events;
CREATE TABLE events_2025 AS events;
```

### 4. Caching Strategies

#### Application-Level Caching
```go
// Cache frequent queries in Redis
func (app *App) getCachedEvents() ([]Event, error) {
    // Check Redis first
    cached := redis.Get("recent_events")
    if cached != nil {
        return cached, nil
    }
    
    // Query database if cache miss  
    events, err := app.queryDatabase()
    redis.Set("recent_events", events, 5*time.Minute)
    return events, err
}
```

#### CDN/Edge Caching
```
Client ───► CDN ───► Load Balancer ───► Service
           Cache Hit      │
              │            ▼
              └──── Cache Miss ────┘
```

## 🛠️ Adding New Services

### Step 1: Identify the Domain
Ask yourself:
- **What business function does this serve?**
- **What data does it manage?**
- **Who are its clients?**

Examples:
- User Management Service (handles authentication, profiles)
- Notification Service (sends emails, SMS, push notifications)
- Analytics Service (processes and aggregates data)

### Step 2: Design the API
Define endpoints before writing code:

```yaml
# OpenAPI specification
paths:
  /users:
    get:
      summary: List users
      responses:
        200:
          description: List of users
    post:
      summary: Create user
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: '#/components/schemas/User'
```

### Step 3: Implement the Service
Follow the established patterns:

```go
package main

// 1. Define your domain models
type User struct {
    ID       int    `json:"id"`
    Email    string `json:"email"` 
    Name     string `json:"name"`
    Created  time.Time `json:"created"`
}

// 2. Define database schema
const initUsersSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.users (
    id Int64,
    email String,
    name String, 
    created DateTime
) ENGINE = MergeTree()
ORDER BY id;
`

// 3. Implement HTTP handlers
func (app *App) handleCreateUser(w http.ResponseWriter, r *http.Request) {
    // Validation, database operations, response
}

func (app *App) handleGetUsers(w http.ResponseWriter, r *http.Request) {
    // Query, serialization, response
}

// 4. Set up routing and server
func main() {
    // Database connection
    // Schema initialization  
    // Route registration
    // Server startup
}
```

### Step 4: Integration
Add to Docker Compose:

```yaml
user-service:
  build: .
  command: /bin/user-service
  ports: ["8100:8100"]
  env_file: [".env"]
  depends_on:
    clickhouse-server: { condition: service_healthy }
```

## 🧪 Testing Microservices

### 1. Unit Tests (Individual Service)
```go
func TestCreateUser(t *testing.T) {
    // Test individual service logic
    app := &App{db: mockDB}
    
    req := httptest.NewRequest("POST", "/users", body)
    resp := httptest.NewRecorder()
    
    app.handleCreateUser(resp, req)
    
    assert.Equal(t, 201, resp.Code)
}
```

### 2. Integration Tests (Service + Database)
```go
func TestUserServiceIntegration(t *testing.T) {
    // Test service with real database
    db := setupTestDatabase()
    defer cleanupTestDatabase(db)
    
    // Test full request/response cycle
}
```

### 3. Contract Tests (Service-to-Service)
```go
func TestEventServiceContract(t *testing.T) {
    // Ensure API contracts are maintained
    // When service A calls service B, B responds as expected
}
```

### 4. End-to-End Tests (Full System)
```bash
# Start all services
docker-compose up -d

# Test complete workflows
curl -X POST http://localhost:8080/ingest/events -d '{...}'
curl http://localhost:8090/query/events

# Verify data flow through multiple services
```

## 🚨 Common Pitfalls and Solutions

### 1. The Distributed Monolith
**Problem:** Services are too tightly coupled

```go
// BAD: Service A directly accesses Service B's database
func (a *ServiceA) getUserEmail(userID int) string {
    row := serviceBDatabase.QueryRow("SELECT email FROM users WHERE id = ?", userID)
    // ...
}

// GOOD: Service A calls Service B's API
func (a *ServiceA) getUserEmail(userID int) string {
    resp, err := http.Get(fmt.Sprintf("http://user-service/users/%d", userID))
    // ...
}
```

### 2. Chatty Interfaces
**Problem:** Too many service-to-service calls

```go
// BAD: Multiple round trips
user := userService.GetUser(id)
profile := profileService.GetProfile(user.ProfileID)  
permissions := authService.GetPermissions(user.ID)

// GOOD: Batch operations or composite APIs
userDetails := userService.GetUserDetails(id) // Returns user + profile + permissions
```

### 3. Data Consistency Issues
**Problem:** Data spread across services can become inconsistent

**Solutions:**
- **Eventual Consistency:** Accept that data will be consistent "eventually"
- **Saga Pattern:** Coordinate transactions across services
- **Event Sourcing:** Store events instead of current state

### 4. Service Discovery
**Problem:** How do services find each other?

**Solutions:**
- **DNS-based:** Use service names (like in Docker Compose)
- **Service Registry:** Consul, etcd, Kubernetes services
- **API Gateway:** Single entry point that routes to services

## 📚 Advanced Patterns

### 1. Circuit Breaker
Prevent cascading failures:

```go
type CircuitBreaker struct {
    failures int
    lastFailure time.Time
    state string // "closed", "open", "half-open"
}

func (cb *CircuitBreaker) Call(fn func() error) error {
    if cb.state == "open" {
        if time.Since(cb.lastFailure) > 30*time.Second {
            cb.state = "half-open"
        } else {
            return errors.New("circuit breaker open")
        }
    }
    
    err := fn()
    if err != nil {
        cb.failures++
        cb.lastFailure = time.Now()
        if cb.failures > 5 {
            cb.state = "open"
        }
    } else {
        cb.failures = 0
        cb.state = "closed"
    }
    
    return err
}
```

### 2. Bulkhead Pattern
Isolate critical resources:

```go
// Separate connection pools for different operations
type App struct {
    readDB  *sql.DB   // Pool for read operations
    writeDB *sql.DB   // Pool for write operations  
    adminDB *sql.DB   // Pool for admin operations
}
```

### 3. Saga Pattern
Manage distributed transactions:

```go
type OrderSaga struct {
    steps []SagaStep
}

type SagaStep struct {
    Execute func() error
    Compensate func() error  // Rollback action
}

func (s *OrderSaga) Run() error {
    for i, step := range s.steps {
        if err := step.Execute(); err != nil {
            // Rollback previous steps
            for j := i - 1; j >= 0; j-- {
                s.steps[j].Compensate()
            }
            return err
        }
    }
    return nil
}
```

## 🎯 When to Use Microservices

### ✅ Good Fit For:
- **Large teams** (Conway's Law: software mirrors org structure)
- **Different scaling requirements** per feature
- **Technology diversity** needs (some services need Python ML libraries, others need Go performance)
- **Independent release cycles**
- **High availability** requirements

### ❌ Poor Fit For:
- **Small teams** (< 10 developers)
- **Simple applications** with basic CRUD operations
- **Tight coupling** between features
- **Limited infrastructure** expertise
- **Startup/early stage** projects (premature optimization)

## 🚀 Migration Strategy

### 1. Strangler Fig Pattern
Gradually replace monolith:

```
Step 1: Monolith handles everything
Step 2: New features as microservices  
Step 3: Extract existing features to microservices
Step 4: Retire monolith
```

### 2. Database Decomposition
```
Step 1: Shared database with service boundaries
Step 2: Separate schemas per service
Step 3: Separate database instances
Step 4: Service-owned data stores
```

## 📊 Monitoring & Observability

### 1. The Three Pillars
- **Metrics:** What is happening? (Prometheus + Grafana)
- **Logs:** Why is it happening? (ELK Stack)
- **Traces:** Where is it happening? (Jaeger/Zipkin)

### 2. Health Checks
```go
func healthCheck(w http.ResponseWriter, r *http.Request) {
    checks := map[string]string{
        "database": checkDatabase(),
        "external_api": checkExternalAPI(),
        "disk_space": checkDiskSpace(),
    }
    
    allHealthy := true
    for _, status := range checks {
        if status != "healthy" {
            allHealthy = false
            break
        }
    }
    
    if allHealthy {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    
    json.NewEncoder(w).Encode(checks)
}
```

### 3. Distributed Tracing
Track requests across services:

```go
import "go.opentelemetry.io/otel/trace"

func (app *App) handleRequest(w http.ResponseWriter, r *http.Request) {
    ctx, span := trace.SpanFromContext(r.Context()).Tracer().Start(r.Context(), "handle-request")
    defer span.End()
    
    // Call other services with context
    data, err := app.callOtherService(ctx)
    // ...
}
```

This guide covers the essential concepts and patterns for building and scaling microservices. The key is to start simple and evolve your architecture as your needs grow.