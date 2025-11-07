package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

/*
EVENTS QUERY MICROSERVICE
=========================
Purpose: This service handles reading/querying of stored events.

Microservice Principles Demonstrated:
1. SINGLE RESPONSIBILITY: Only handles event retrieval (not storage)
2. SEPARATION OF CONCERNS: Read operations separate from write operations
3. STATELESS: No session state, each request is independent

Read/Write Separation Benefits:
- Independent scaling (queries often need more replicas than writes)
- Different performance optimizations (read replicas, caching)
- Different security policies (read-only vs read-write access)

Scaling Strategies:
1. Read Replicas: Multiple ClickHouse replicas for query distribution
2. Caching: Redis/Memcached for frequently accessed data
3. Load Balancing: Multiple service instances behind load balancer
4. Database Sharding: Partition data by date/source for better performance
*/

// App holds database connection for query operations
// In production, add: connection pooling, circuit breakers, rate limiting
type App struct {
	db *sql.DB // Read-only connection in production environments
}

func main() {
	// Database Connection: Connect to the same ClickHouse instance
	// In production, this could be a read replica for better performance
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Schema Management: Query services do NOT create/modify schema
	// They assume the schema exists (created by the corresponding ingest service)
	// This enforces clear ownership: ingest-events owns the events table
	log.Println("Connected to ClickHouse for event queries.")

	// Application Setup
	app := &App{db: conn}
	
	// Route Definition: RESTful pattern - GET for retrieving data
	http.HandleFunc("/query/events", app.handleQuery)
	
	// Port Assignment: Each service gets a unique port
	// Query services typically use ports in 809x range
	port := ":8090"
	log.Printf("Starting event query service on %s...", port)
	
	// Start Server: Handle incoming query requests
	http.ListenAndServe(port, nil)
}

/*
EVENT QUERY HANDLER
==================
This function handles HTTP requests to retrieve stored events.

Query Design Principles:
1. Default Limits: Prevent large result sets that could overwhelm clients
2. Ordering: Most recent events first (common use case)
3. Selective Fields: Only return necessary data

Scaling & Performance Enhancements (for production):
- Add pagination (offset/limit parameters)
- Add filtering (by level, source, date range)
- Add caching for frequent queries
- Add query result compression
- Implement GraphQL for flexible field selection
*/
func (app *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	// Method Validation: Only GET requests for data retrieval
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method allowed", http.StatusMethodNotAllowed)
		return
	}

	// Content Type: Set JSON response header
	w.Header().Set("Content-Type", "application/json")

	// Database Query: Retrieve recent events
	// ORDER BY Timestamp DESC: Most recent events first
	// LIMIT 10: Prevent overwhelming the client/network
	// In production, make limit configurable via query parameters
	query := "SELECT Timestamp, Level, Source, Message, Context FROM events ORDER BY Timestamp DESC LIMIT 10"
	
	rows, err := app.db.Query(query)
	if err != nil {
		log.Printf("Database query failed: %v", err)
		http.Error(w, "Query execution failed", http.StatusInternalServerError)
		return
	}
	defer rows.Close() // Always close database resources

	// Result Processing: Convert database rows to Go structs
	var events []models.Event
	for rows.Next() {
		var event models.Event
		
		// Scan database columns into struct fields
		// Must match the SELECT column order exactly
		if err := rows.Scan(
			&event.Timestamp, &event.Level, &event.Source, &event.Message, &event.Context,
		); err != nil {
			log.Printf("Row scanning failed: %v", err)
			http.Error(w, "Data processing error", http.StatusInternalServerError)
			return
		}
		events = append(events, event)
	}

	// Check for iteration errors
	if err = rows.Err(); err != nil {
		log.Printf("Row iteration error: %v", err)
		http.Error(w, "Data retrieval error", http.StatusInternalServerError)
		return
	}

	// JSON Response: Convert Go structs to JSON and send to client
	// This automatically sets proper HTTP status code (200 OK)
	if err := json.NewEncoder(w).Encode(events); err != nil {
		log.Printf("JSON encoding failed: %v", err)
		// Response already started, can't change status code
		return
	}

	log.Printf("Successfully returned %d events", len(events))
	
	// Future Enhancements:
	// - Add query parameter parsing for filtering
	// - Add pagination metadata in response headers
	// - Add response caching with proper cache headers
	// - Add request tracing for distributed debugging
}