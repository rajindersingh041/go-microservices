package main



import (import (

	"context"	"context"

	"database/sql"	"database/sql"

	"encoding/json"	"encoding/json"

	"io"	"io"

	"log"	"log"

	"net/http"	"net/http"

	"os"	"os"



	_ "github.com/ClickHouse/clickhouse-go/v2"	_ "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/rajindersingh041/go-microservices/internal/database"	"github.com/rajindersingh041/go-microservices/internal/database"

	"github.com/rajindersingh041/go-microservices/internal/models"	"github.com/rajindersingh041/go-microservices/internal/models"

))



/*/*

EVENTS INGESTION MICROSERVICEEVENTS INGESTION MICROSERVICE

==========================================================

Purpose: This service handles the ingestion (storage) of application events and logs.Purpose: This service handles the ingestion (storage) of application events and logs.



Microservice Principle: SINGLE RESPONSIBILITYMicroservice Principle: SINGLE RESPONSIBILITY

- This service ONLY handles event storage- This service ONLY handles event storage

- It owns the 'events' table (Database Per Service pattern)- It owns the 'events' table (Database Per Service pattern)

- Other services communicate via HTTP API, not direct database access- Other services communicate via HTTP API, not direct database access



Scaling Strategies:Scaling Strategies:

1. Horizontal: Run multiple instances behind a load balancer1. Horizontal: Run multiple instances behind a load balancer

2. Database: Use ClickHouse clustering for distributed storage2. Database: Use ClickHouse clustering for distributed storage

3. Caching: Add Redis for frequent event patterns3. Caching: Add Redis for frequent event patterns

4. Async: Use message queues (Kafka/RabbitMQ) for high-throughput scenarios4. Async: Use message queues (Kafka/RabbitMQ) for high-throughput scenarios

*/*/



// Database schema owned by this service// Database schema owned by this service

// Using ClickHouse MergeTree for high-performance analytics on time-series data// Using ClickHouse MergeTree for high-performance analytics on time-series data

// ORDER BY (Source, Timestamp) optimizes queries by service and time// ORDER BY (Source, Timestamp) optimizes queries by service and time

const initEventsSQL = `const initEventsSQL = `

CREATE TABLE IF NOT EXISTS mydatabase.events (CREATE TABLE IF NOT EXISTS mydatabase.events (

    Timestamp DateTime,    Timestamp DateTime,

    Level     String,    Level     String,

    Source    String,    Source    String,

    Message   String,    Message   String,

    Context   Map(String, String)  -- Key-value pairs for additional event data    Context   Map(String, String)  -- Key-value pairs for additional event data

) ENGINE = MergeTree()) ENGINE = MergeTree()

ORDER BY (Source, Timestamp);  -- Optimized for queries filtering by service and timeORDER BY (Source, Timestamp);  -- Optimized for queries filtering by service and time

``



// App holds database connection and implements HTTP handlers// App holds database connection and implements HTTP handlers

// In production, this would include: metrics, logging, circuit breakers, etc.// In production, this would include: metrics, logging, circuit breakers, etc.

type App struct {type App struct {

	db *sql.DB  // Database connection pool (thread-safe)	db *sql.DB  // Database connection pool (thread-safe)

}}



func main() {func main() {

	// Configuration: Read database host from environment	// Configuration: Read database host from environment

	// This allows different environments (dev/staging/prod) to use different databases	// This allows different environments (dev/staging/prod) to use different databases

	host := os.Getenv("CLICKHOUSE_HOST")	host := os.Getenv("CLICKHOUSE_HOST")

		

	// Database Connection: Use shared connection logic from internal/database	// Database Connection: Use shared connection logic from internal/database

	// This connection pool is thread-safe and handles reconnections automatically	// This connection pool is thread-safe and handles reconnections automatically

	conn, err := database.Connect(host)	conn, err := database.Connect(host)

	if err != nil {	if err != nil {

		log.Fatalf("Failed to connect: %v", err)		log.Fatalf("Failed to connect: %v", err)

	}	}

	defer conn.Close()	defer conn.Close()



	// Schema Initialization: Each service owns its database schema	// Schema Initialization: Each service owns its database schema

	// This follows "Database Per Service" microservices pattern	// This follows "Database Per Service" microservices pattern

	// The service is responsible for creating and managing its own tables	// The service is responsible for creating and managing its own tables

	if _, err := conn.Exec(initEventsSQL); err != nil {	if _, err := conn.Exec(initEventsSQL); err != nil {

		log.Fatalf("Failed to init 'events' schema: %v", err)		log.Fatalf("Failed to init 'events' schema: %v", err)

	}	}

	log.Println("'events' table is ready.")	log.Println("'events' table is ready.")



	// Application Setup: Initialize the app with database connection	// Application Setup: Initialize the app with database connection

	app := &App{db: conn}	app := &App{db: conn}

		

	// Route Definition: Define HTTP endpoints this service provides	// Route Definition: Define HTTP endpoints this service provides

	// RESTful pattern: POST for creating/storing data	// RESTful pattern: POST for creating/storing data

	http.HandleFunc("/ingest/events", app.handleIngest)	http.HandleFunc("/ingest/events", app.handleIngest)

		

	// Service Discovery: Each service runs on a unique port	// Service Discovery: Each service runs on a unique port

	// Port 8080 is reserved for event ingestion	// Port 8080 is reserved for event ingestion

	// In production, use service mesh or API gateway for routing	// In production, use service mesh or API gateway for routing

	port := ":8080"	port := ":8080"

	log.Printf("Starting event ingestion service on %s...", port)	log.Printf("Starting event ingestion service on %s...", port)

		

	// Start HTTP Server: This blocks and handles incoming requests	// Start HTTP Server: This blocks and handles incoming requests

	// In production, add graceful shutdown handling	// In production, add graceful shutdown handling

	http.ListenAndServe(port, nil)	http.ListenAndServe(port, nil)

}}



/*/*

EVENT INGESTION HANDLEREVENT INGESTION HANDLER

============================================

This function processes incoming HTTP requests to store events.This function processes incoming HTTP requests to store events.



Input Flexibility: Accepts both single events and arrays of eventsInput Flexibility: Accepts both single events and arrays of events

- Single: {"timestamp": "...", "level": "INFO", ...}- Single: {"timestamp": "...", "level": "INFO", ...}

- Batch: [{"timestamp": "...", "level": "INFO", ...}, {...}]- Batch: [{"timestamp": "...", "level": "INFO", ...}, {...}]



Performance Optimizations:Performance Optimizations:

1. Batch Processing: Multiple events in one database transaction1. Batch Processing: Multiple events in one database transaction

2. Prepared Statements: SQL compiled once, executed many times2. Prepared Statements: SQL compiled once, executed many times

3. Database Transactions: Atomic operations for consistency3. Database Transactions: Atomic operations for consistency



Scaling Considerations:Scaling Considerations:

- Add rate limiting to prevent abuse- Add rate limiting to prevent abuse

- Implement async processing with message queues for high volume- Implement async processing with message queues for high volume

- Add request validation and sanitization- Add request validation and sanitization

- Include metrics collection (requests/sec, processing time)- Include metrics collection (requests/sec, processing time)

*/*/

func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {

	// Method Validation: Only accept POST requests	// Method Validation: Only accept POST requests

	// RESTful principle: POST for creating resources	// RESTful principle: POST for creating resources

	if r.Method != http.MethodPost {	if r.Method != http.MethodPost {

		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)		http.Error(w, "Only POST method allowed", http.StatusMethodNotAllowed)

		return		return

	}	}



	// Request Body Reading: Extract JSON payload	// Request Body Reading: Extract JSON payload

	// In production, add content-length limits to prevent DoS attacks	// In production, add content-length limits to prevent DoS attacks

	body, err := io.ReadAll(r.Body)	body, err := io.ReadAll(r.Body)

	if err != nil {	if err != nil {

		log.Printf("Error reading request body: %v", err)		log.Printf("Error reading request body: %v", err)

		http.Error(w, "Failed to read request body", http.StatusBadRequest)		http.Error(w, "Failed to read request body", http.StatusBadRequest)

		return		return

	}	}



	// Flexible JSON Parsing: Handle both single events and arrays	// Flexible JSON Parsing: Handle both single events and arrays

	// This improves API usability - clients can send one or many events	// This improves API usability - clients can send one or many events

	var events []models.Event	var events []models.Event

		

	// Try parsing as array first	// Try parsing as array first

	err = json.Unmarshal(body, &events)	err = json.Unmarshal(body, &events)

	if err != nil {	if err != nil {

		// If array parsing fails, try single event		// If array parsing fails, try single event

		var singleEvent models.Event		var singleEvent models.Event

		err2 := json.Unmarshal(body, &singleEvent)		err2 := json.Unmarshal(body, &singleEvent)

		if err2 != nil {		if err2 != nil {

			log.Printf("JSON parsing failed: %v", err2)			log.Printf("JSON parsing failed: %v", err2)

			http.Error(w, "Invalid JSON format", http.StatusBadRequest)			http.Error(w, "Invalid JSON format", http.StatusBadRequest)

			return			return

		}		}

		// Convert single event to array for uniform processing		// Convert single event to array for uniform processing

		events = []models.Event{singleEvent}		events = []models.Event{singleEvent}

	}	}



	// Validation: Ensure we have events to process	// Validation: Ensure we have events to process

	if len(events) == 0 {	if len(events) == 0 {

		http.Error(w, "No events provided", http.StatusBadRequest)		http.Error(w, "No events provided", http.StatusBadRequest)

		return		return

	}	}



	// Database Transaction: Ensure all events are stored atomically	// Database Transaction: Ensure all events are stored atomically

	// If any event fails, none are stored (consistency)	// If any event fails, none are stored (consistency)

	tx, err := app.db.Begin()	tx, err := app.db.Begin()

	if err != nil {	if err != nil {

		log.Printf("Failed to begin transaction: %v", err)		log.Printf("Failed to begin transaction: %v", err)

		http.Error(w, "Database error", http.StatusInternalServerError)		http.Error(w, "Database error", http.StatusInternalServerError)

		return		return

	}	}

		

	// Prepared Statement: Compile SQL once for performance	// Prepared Statement: Compile SQL once for performance

	// ClickHouse VALUES format for bulk inserts	// ClickHouse VALUES format for bulk inserts

	stmt, err := tx.PrepareContext(context.Background(), 	stmt, err := tx.PrepareContext(context.Background(), 

		"INSERT INTO events (Timestamp, Level, Source, Message, Context) VALUES (?, ?, ?, ?, ?)")		"INSERT INTO events (Timestamp, Level, Source, Message, Context) VALUES (?, ?, ?, ?, ?)")

	if err != nil {	if err != nil {

		log.Printf("Failed to prepare statement: %v", err)		log.Printf("Failed to prepare statement: %v", err)

		tx.Rollback()		tx.Rollback()

		http.Error(w, "Database error", http.StatusInternalServerError)		http.Error(w, "Database error", http.StatusInternalServerError)

		return		return

	}	}

		

	// Batch Processing: Insert all events in the same transaction	// Batch Processing: Insert all events in the same transaction

	// This is much faster than individual INSERT statements	// This is much faster than individual INSERT statements

	for _, e := range events {	for _, e := range events {

		if _, err := stmt.ExecContext(context.Background(), 		if _, err := stmt.ExecContext(context.Background(), 

			e.Timestamp, e.Level, e.Source, e.Message, e.Context,			e.Timestamp, e.Level, e.Source, e.Message, e.Context,

		); err != nil {		); err != nil {

			log.Printf("Error executing batch insert: %v", err)			log.Printf("Error executing batch insert: %v", err)

			tx.Rollback() // Rollback on any failure			tx.Rollback() // Rollback on any failure

			http.Error(w, "Database insert failed", http.StatusInternalServerError)			http.Error(w, "Database insert failed", http.StatusInternalServerError)

			return			return

		}		}

	}	}

		

	// Commit Transaction: Make all changes permanent	// Commit Transaction: Make all changes permanent

	if err := tx.Commit(); err != nil {	if err := tx.Commit(); err != nil {

		log.Printf("Failed to commit transaction: %v", err)		log.Printf("Failed to commit transaction: %v", err)

		http.Error(w, "Failed to save events", http.StatusInternalServerError)		http.Error(w, "Failed to save events", http.StatusInternalServerError)

		return		return

	}	}



	// Success Response: 202 Accepted indicates async processing completed	// Success Response: 202 Accepted indicates async processing completed

	log.Printf("Successfully ingested %d events", len(events))	log.Printf("Successfully ingested %d events", len(events))

	w.WriteHeader(http.StatusAccepted)	w.WriteHeader(http.StatusAccepted)

		

	// Optional: Return summary in production	// Optional: Return summary in production

	// json.NewEncoder(w).Encode(map[string]interface{}{	// json.NewEncoder(w).Encode(map[string]interface{}{

	//     "status": "accepted",	//     "status": "accepted",

	//     "events_processed": len(events),	//     "events_processed": len(events),

	//     "timestamp": time.Now(),	//     "timestamp": time.Now(),

	// })	// })

}}