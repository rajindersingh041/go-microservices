package main

import (
	"context"
	"database/sql" // <-- Use standard *sql.DB
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	// This is the blank import to register the driver
	_ "github.com/ClickHouse/clickhouse-go/v2"

	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

type App struct {
	db *sql.DB // <-- Use the concurrent-safe pool
}

func main() {
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer conn.Close()
	log.Printf("Successfully connected to ClickHouse pool on %s", host)

	if err := database.InitializeSchema(conn); err != nil {
		log.Fatalf("Failed to initialize schema: %v", err)
	}

	app := &App{db: conn}
	http.HandleFunc("/ingest", app.handleIngest)

	port := ":8080"
	log.Printf("Starting ingestion service on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	// ... "Smart" JSON logic is unchanged ...
	body, err := io.ReadAll(r.Body)
	if err != nil { /* ... error handling ... */ }
	var events []models.Event
	err = json.Unmarshal(body, &events)
	if err != nil {
		var singleEvent models.Event
		err2 := json.Unmarshal(body, &singleEvent)
		if err2 != nil { /* ... error handling ... */ }
		events = []models.Event{singleEvent}
	}
	if len(events) == 0 { /* ... error handling ... */ }

	// --- NEW: ClickHouse database/sql Batch Insert ---
	// We must use a transaction for batching with the sql.DB driver
	tx, err := app.db.Begin()
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// Prepare the statement within the transaction
	stmt, err := tx.PrepareContext(context.Background(), "INSERT INTO events (Timestamp, Level, Source, Message)")
	if err != nil {
		log.Printf("Error preparing statement: %v", err)
		tx.Rollback()
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	// Loop and add each event to the batch
	for _, e := range events {
		if _, err := stmt.ExecContext(context.Background(), e.Timestamp, e.Level, e.Source, e.Message); err != nil {
			log.Printf("Error executing batch: %v", err)
			tx.Rollback()
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	// --- END NEW BATCH LOGIC ---

	log.Printf("Successfully ingested batch of %d events", len(events))
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "accepted",
		"ingested": len(events),
	})
}