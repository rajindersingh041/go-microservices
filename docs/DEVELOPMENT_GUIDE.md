# Development Setup Guide for Beginners

This guide is designed for developers who are new to microservices and Go. We'll walk through setting up the development environment, understanding the codebase, and making your first changes.

## 🎯 What You'll Learn

By the end of this guide, you'll understand:
- How microservices work and communicate
- Go project structure and conventions
- How to add new endpoints and services
- Testing and debugging microservices
- Best practices for microservice development

## 📋 Prerequisites

### Required Software
- **Docker Desktop**: [Download here](https://www.docker.com/products/docker-desktop)
- **Git**: [Download here](https://git-scm.com/downloads)  
- **VS Code** (recommended): [Download here](https://code.visualstudio.com/)
- **Go** (for local development): [Download here](https://golang.org/dl/)

### VS Code Extensions (Recommended)
```bash
# Install these extensions for better Go development
code --install-extension golang.go
code --install-extension ms-vscode.vscode-json
code --install-extension ms-azuretools.vscode-docker
code --install-extension humao.rest-client
```

### System Requirements
- **OS**: Windows 10+, macOS 10.14+, or Linux
- **RAM**: 8GB minimum (16GB recommended)
- **Storage**: 10GB free space
- **Network**: Internet connection for downloading dependencies

## 🚀 Getting Started

### Step 1: Clone the Repository

```bash
# Open terminal/command prompt and run:
git clone <repository-url>
cd go-microservices

# Verify you have all the files
ls -la  # On Windows: dir
```

You should see:
```
├── cmd/                    # All microservices
├── internal/              # Shared code
├── docker-compose.yml     # Service orchestration
├── Dockerfile            # Container build instructions
├── go.mod               # Go dependencies
├── README.md           # Project documentation
└── docs/              # Additional documentation
```

### Step 2: Understand the Architecture

**Think of it like a restaurant:**

```
Customer Order (HTTP Request)
        ↓
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Cashier       │    │   Kitchen       │    │   Database      │
│ (API Gateway)   │ ── │ (Microservice)  │ ── │  (ClickHouse)   │
└─────────────────┘    └─────────────────┘    └─────────────────┘
```

**In our system:**
- **Customer**: Your application making HTTP requests
- **Cashier**: Nginx (load balancer) or direct service access
- **Kitchen Stations**: Different microservices (events, market data)
- **Database**: ClickHouse storing all the data

### Step 3: Start the System

```bash
# Start all services (this will take a few minutes first time)
docker-compose up -d

# Check if services are running
docker-compose ps

# You should see something like:
#   ingest-events        Up      0.0.0.0:8080->8080/tcp
#   ingest-marketdata    Up      0.0.0.0:8081->8081/tcp
#   query-events         Up      0.0.0.0:8090->8090/tcp
#   query-marketdata     Up      0.0.0.0:8091->8091/tcp
#   clickhouse-server    Up      0.0.0.0:8123->8123/tcp
```

### Step 4: Test the System

Create a file called `test-requests.http` in VS Code:

```http
### Test Event Ingestion
POST http://localhost:8080/ingest/events
Content-Type: application/json

{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "source": "test-learning",
  "message": "My first microservice test!",
  "context": {
    "developer": "your-name",
    "environment": "learning"
  }
}

### Test Event Query
GET http://localhost:8090/query/events

### Test Market Data Ingestion
POST http://localhost:8081/ingest/marketdata
Content-Type: application/json

{
  "data": {
    "request_id": "learning-test-123",
    "time_in_millis": 1640995800000,
    "token_data": {
      "LEARNING_STOCK": {
        "timestamp": "1640995800",
        "lastPrice": 100.50,
        "volume": 1000
      }
    }
  },
  "success": true
}

### Test Market Data Query
GET http://localhost:8091/query/marketdata
```

Click the "Send Request" button above each request in VS Code.

## 🔍 Understanding the Code

### Microservice Structure

Each service follows the same pattern:

```go
// Every microservice has these components:

package main

// 1. IMPORTS: External libraries we use
import (
    "database/sql"      // Database connections
    "encoding/json"     // JSON parsing
    "net/http"         // HTTP server
    // ... more imports
)

// 2. CONFIGURATION: Database schema this service owns
const initTableSQL = `CREATE TABLE IF NOT EXISTS ...`

// 3. APPLICATION STRUCT: Holds shared resources
type App struct {
    db *sql.DB  // Database connection
}

// 4. MAIN FUNCTION: Sets up and starts the service
func main() {
    // Connect to database
    // Create tables
    // Set up HTTP routes
    // Start server
}

// 5. HTTP HANDLERS: Functions that handle incoming requests
func (app *App) handleRequest(w http.ResponseWriter, r *http.Request) {
    // Process the request
    // Talk to database
    // Send response
}
```

### Let's Trace a Request

When you send `POST http://localhost:8080/ingest/events`:

1. **HTTP Request arrives** at port 8080
2. **Go HTTP server** receives it in `cmd/ingest-events/main.go`
3. **Router** directs it to `handleIngest` function
4. **Handler function**:
   - Reads JSON from request body
   - Validates the data
   - Starts database transaction
   - Inserts events into ClickHouse
   - Commits transaction
   - Sends HTTP response

```go
// This is what happens inside handleIngest:
func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
    // 1. Read the request
    body, err := io.ReadAll(r.Body)
    
    // 2. Parse JSON
    var events []models.Event
    json.Unmarshal(body, &events)
    
    // 3. Save to database
    tx, _ := app.db.Begin()
    for _, event := range events {
        tx.Exec("INSERT INTO events ...", event.Timestamp, event.Level, ...)
    }
    tx.Commit()
    
    // 4. Send success response
    w.WriteHeader(http.StatusAccepted)
}
```

## 🛠️ Making Your First Change

Let's add a health check endpoint to the events service.

### Step 1: Add the Endpoint

Open `cmd/ingest-events/main.go` and find the `main()` function:

```go
func main() {
    // ... existing code ...
    
    app := &App{db: conn}
    http.HandleFunc("/ingest/events", app.handleIngest)
    
    // ADD THIS LINE:
    http.HandleFunc("/health", app.handleHealth)
    
    port := ":8080"
    // ... rest of main function
}
```

### Step 2: Implement the Handler

Add this function at the end of the file:

```go
/*
HEALTH CHECK ENDPOINT
====================
This endpoint allows monitoring systems to check if the service is working.
It's essential for:
- Load balancers (to know if they should send traffic here)
- Kubernetes (to restart unhealthy containers)
- Monitoring systems (to alert when services are down)

Best practices:
- Check database connectivity
- Check external dependencies
- Return appropriate HTTP status codes
- Include useful diagnostic information
*/
func (app *App) handleHealth(w http.ResponseWriter, r *http.Request) {
    // Only allow GET requests for health checks
    if r.Method != http.MethodGet {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Test database connection
    err := app.db.Ping()
    if err != nil {
        // Service is unhealthy - database is down
        w.WriteHeader(http.StatusServiceUnavailable)
        json.NewEncoder(w).Encode(map[string]interface{}{
            "status": "unhealthy",
            "error": "database connection failed",
            "timestamp": time.Now(),
        })
        return
    }

    // Service is healthy
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "healthy",
        "service": "ingest-events",
        "timestamp": time.Now(),
        "database": "connected",
    })
}
```

### Step 3: Add the Missing Import

At the top of the file, add `time` to the imports:

```go
import (
    "context"
    "database/sql"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "time"  // ADD THIS LINE

    _ "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/rajindersingh041/go-microservices/internal/database"
    "github.com/rajindersingh041/go-microservices/internal/models"
)
```

### Step 4: Test Your Change

```bash
# Rebuild and restart the service
docker-compose up -d --no-deps --build ingest-events

# Test the new endpoint
curl http://localhost:8080/health

# You should see:
# {
#   "status": "healthy",
#   "service": "ingest-events", 
#   "timestamp": "2024-01-15T10:30:00Z",
#   "database": "connected"
# }
```

**Congratulations!** You just:
1. ✅ Added a new HTTP endpoint
2. ✅ Implemented JSON responses
3. ✅ Added database health checking
4. ✅ Rebuilt and deployed the service

## 🧪 Testing Your Changes

### Unit Testing

Create `cmd/ingest-events/main_test.go`:

```go
package main

import (
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestHealthEndpoint(t *testing.T) {
    // Create a mock HTTP request
    req, err := http.NewRequest("GET", "/health", nil)
    if err != nil {
        t.Fatal(err)
    }

    // Create a response recorder
    rr := httptest.NewRecorder()
    
    // Create app with mock database (in real tests, use a test database)
    app := &App{db: nil} // For this simple test, we'll skip DB
    
    // Call the handler
    handler := http.HandlerFunc(app.handleHealth)
    handler.ServeHTTP(rr, req)

    // Check status code
    if status := rr.Code; status != http.StatusOK {
        t.Errorf("Wrong status code: got %v want %v", status, http.StatusOK)
    }
}
```

Run the test:
```bash
cd cmd/ingest-events
go test
```

### Integration Testing

Test the full service with a real database:

```bash
# Start services
docker-compose up -d

# Test complete flow
echo "Testing event ingestion..."
curl -X POST http://localhost:8080/ingest/events \
  -H "Content-Type: application/json" \
  -d '{
    "timestamp": "2024-01-15T10:30:00Z",
    "level": "INFO",
    "source": "test",
    "message": "Integration test",
    "context": {}
  }'

echo "Testing event retrieval..."
curl http://localhost:8090/query/events

echo "Testing health check..."
curl http://localhost:8080/health
```

## 🔧 Adding a New Service

Let's create a completely new service for user management.

### Step 1: Create Service Directory

```bash
mkdir cmd/user-service
```

### Step 2: Create the Service

Create `cmd/user-service/main.go`:

```go
package main

import (
    "context"
    "database/sql"
    "encoding/json"
    "io"
    "log"
    "net/http"
    "os"
    "time"

    _ "github.com/ClickHouse/clickhouse-go/v2"
    "github.com/rajindersingh041/go-microservices/internal/database"
)

/*
USER MANAGEMENT MICROSERVICE
============================
This service demonstrates how to create a new microservice from scratch.

Responsibilities:
- Store and retrieve user information
- Validate user data
- Provide user lookup APIs

This follows microservice principles:
- Single responsibility (only handles users)
- Own database schema (users table)
- Independent deployment
- API-first communication
*/

// User represents a user in our system
type User struct {
    ID       int       `json:"id"`
    Email    string    `json:"email"`
    Name     string    `json:"name"`
    Created  time.Time `json:"created"`
}

// Database schema owned by this service
const initUsersSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.users (
    id Int64,
    email String,
    name String,
    created DateTime
) ENGINE = MergeTree()
ORDER BY id;
`

type App struct {
    db *sql.DB
}

func main() {
    // Database connection
    host := os.Getenv("CLICKHOUSE_HOST")
    conn, err := database.Connect(host)
    if err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    defer conn.Close()

    // Initialize schema
    if _, err := conn.Exec(initUsersSQL); err != nil {
        log.Fatalf("Failed to init users schema: %v", err)
    }
    log.Println("Users table initialized.")

    app := &App{db: conn}
    
    // Define routes
    http.HandleFunc("/users", app.handleUsers)
    http.HandleFunc("/health", app.handleHealth)
    
    port := ":8100"  // New port for new service
    log.Printf("Starting user service on %s...", port)
    http.ListenAndServe(port, nil)
}

func (app *App) handleUsers(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case http.MethodPost:
        app.createUser(w, r)
    case http.MethodGet:
        app.getUsers(w, r)
    default:
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
    }
}

func (app *App) createUser(w http.ResponseWriter, r *http.Request) {
    var user User
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "Failed to read body", http.StatusBadRequest)
        return
    }

    if err := json.Unmarshal(body, &user); err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }

    // Simple validation
    if user.Email == "" || user.Name == "" {
        http.Error(w, "Email and name are required", http.StatusBadRequest)
        return
    }

    user.Created = time.Now()
    
    // Insert into database
    _, err = app.db.ExecContext(context.Background(),
        "INSERT INTO users (email, name, created) VALUES (?, ?, ?)",
        user.Email, user.Name, user.Created)
    
    if err != nil {
        log.Printf("Database error: %v", err)
        http.Error(w, "Failed to create user", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusCreated)
    json.NewEncoder(w).Encode(map[string]string{
        "status": "created",
        "message": "User created successfully",
    })
}

func (app *App) getUsers(w http.ResponseWriter, r *http.Request) {
    rows, err := app.db.Query("SELECT email, name, created FROM users ORDER BY created DESC LIMIT 10")
    if err != nil {
        http.Error(w, "Query failed", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.Email, &user.Name, &user.Created); err != nil {
            http.Error(w, "Scan failed", http.StatusInternalServerError)
            return
        }
        users = append(users, user)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(users)
}

func (app *App) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "status": "healthy",
        "service": "user-service",
        "timestamp": time.Now(),
    })
}
```

### Step 3: Update Build Configuration

Add to `Dockerfile`:

```dockerfile
# Add this line in the build stage
RUN CGO_ENABLED=0 go build -o /bin/user-service ./cmd/user-service/main.go

# Add this line in the final stage
COPY --from=builder /bin/user-service /bin/user-service
```

### Step 4: Update Docker Compose

Add to `docker-compose.yml`:

```yaml
user-service:
  build: .
  command: /bin/user-service
  ports: ["8100:8100"]
  env_file: [".env"]
  depends_on:
    clickhouse-server: { condition: service_healthy }
  healthcheck:
    test: ["CMD", "curl", "http://localhost:8100/health"]
    interval: 10s
    timeout: 5s
    retries: 3
    start_period: 5s
```

### Step 5: Test the New Service

```bash
# Build and start the new service
docker-compose up -d --build

# Test creating a user
curl -X POST http://localhost:8100/users \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "name": "Test User"
  }'

# Test getting users
curl http://localhost:8100/users

# Test health check
curl http://localhost:8100/health
```

## 🐛 Debugging Microservices

### Viewing Logs

```bash
# View logs from all services
docker-compose logs -f

# View logs from specific service
docker-compose logs -f ingest-events

# View logs with timestamps
docker-compose logs -f -t ingest-events

# View last 100 lines
docker-compose logs --tail=100 ingest-events
```

### Common Issues and Solutions

#### Issue: Service won't start
```bash
# Check if port is already in use
netstat -tulpn | grep :8080

# Check service logs
docker-compose logs ingest-events

# Restart specific service
docker-compose restart ingest-events
```

#### Issue: Database connection failed
```bash
# Check if ClickHouse is running
docker-compose ps clickhouse-server

# Test database connection
docker exec -it clickhouse-server clickhouse-client -u myuser --password mypassword

# Check environment variables
docker-compose exec ingest-events env | grep CLICKHOUSE
```

#### Issue: Services can't communicate
```bash
# Test service-to-service communication
docker-compose exec ingest-events curl http://query-events:8090/query/events

# Check network connectivity
docker network ls
docker network inspect go-microservices_default
```

### Debug Mode

Add debug logging to your services:

```go
// Add to main.go
import "os"

func main() {
    // Enable debug logging
    if os.Getenv("DEBUG") == "true" {
        log.SetFlags(log.LstdFlags | log.Lshortfile)
        log.Println("Debug mode enabled")
    }
    
    // ... rest of main function
}

// Add debug logs in handlers
func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
    if os.Getenv("DEBUG") == "true" {
        log.Printf("Received request: %s %s", r.Method, r.URL.Path)
    }
    
    // ... rest of handler
}
```

Enable debug mode:
```bash
# Add to docker-compose.yml environment
services:
  ingest-events:
    environment:
      - DEBUG=true
```

## 📚 Next Steps

### Learn More About Go
- [Go Tour](https://tour.golang.org/) - Interactive Go tutorial
- [Effective Go](https://golang.org/doc/effective_go.html) - Go best practices
- [Go by Example](https://gobyexample.com/) - Practical Go examples

### Learn More About Microservices
- Read `docs/MICROSERVICES_GUIDE.md` for advanced patterns
- Read `docs/API_DOCUMENTATION.md` for complete API reference
- Experiment with the Circuit Breaker pattern
- Try implementing event-driven communication

### Practice Exercises

1. **Add Validation**: Add email validation to the user service
2. **Add Pagination**: Modify query endpoints to support pagination
3. **Add Metrics**: Implement Prometheus metrics in services
4. **Add Caching**: Implement Redis caching for frequent queries
5. **Add Authentication**: Create a JWT authentication service

### Production Readiness

Before deploying to production, implement:
- [ ] Comprehensive error handling
- [ ] Input validation and sanitization
- [ ] Rate limiting
- [ ] Authentication and authorization
- [ ] Monitoring and alerting
- [ ] Database migrations
- [ ] Graceful shutdown handling
- [ ] Circuit breakers for external dependencies

## 🤝 Getting Help

### Resources
- **Documentation**: Check the `docs/` folder
- **Code Examples**: Look at existing services for patterns
- **Community**: Join Go and microservices forums

### Troubleshooting Checklist
- [ ] All services are running (`docker-compose ps`)
- [ ] Database is healthy (`curl http://localhost:8123`)
- [ ] Environment variables are set correctly
- [ ] Ports are not conflicting
- [ ] JSON payloads are valid
- [ ] Log files don't show errors

**Remember**: Microservices are about building small, focused services that do one thing well. Start simple, test thoroughly, and iterate based on real needs.

Happy coding! 🚀