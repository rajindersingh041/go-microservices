package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"

	_ "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

// This service OWNS the 'events' table
// --- THIS IS THE UPDATED TABLE ---
// We add a 'Context' column of type Map(String, String)
const initEventsSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.events (
    Timestamp DateTime,
    Level     String,
    Source    String,
    Message   String,
    Context   Map(String, String)
) ENGINE = MergeTree()
ORDER BY (Source, Timestamp);
`
// --- END OF UPDATE ---
type App struct {
	db *sql.DB
}

func main() {
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Initialize *this service's* schema
	if _, err := conn.Exec(initEventsSQL); err != nil {
		log.Fatalf("Failed to init 'events' schema: %v", err)
	}
	log.Println("'events' table is ready.")

	app := &App{db: conn}
	http.HandleFunc("/ingest/events", app.handleIngest) // Define route
	port := ":8080"
	log.Printf("Starting event ingestion service on %s...", port)
	http.ListenAndServe(port, nil)
}

func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	// ... (your smart JSON array/object logic is unchanged)
	body, err := io.ReadAll(r.Body)
	if err != nil { /* ... */ }
	var events []models.Event // <-- This now expects the new struct
	err = json.Unmarshal(body, &events)
	if err != nil {
		var singleEvent models.Event
		err2 := json.Unmarshal(body, &singleEvent)
		if err2 != nil { /* ... */ }
		events = []models.Event{singleEvent}
	}
	if len(events) == 0 { /* ... */ }

	tx, _ := app.db.Begin()
    // --- UPDATE THE INSERT QUERY ---
	stmt, _ := tx.PrepareContext(context.Background(), 
        "INSERT INTO events (Timestamp, Level, Source, Message, Context)")
	
	for _, e := range events {
        // --- ADD THE NEW 'Context' FIELD ---
		if _, err := stmt.ExecContext(context.Background(), 
            e.Timestamp, e.Level, e.Source, e.Message, e.Context,
        ); err != nil {
			log.Printf("Error executing batch: %v", err)
			tx.Rollback()
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}
	if err := tx.Commit(); err != nil { /* ... */ }

	log.Printf("Ingested %d events", len(events))
	w.WriteHeader(http.StatusAccepted)
}