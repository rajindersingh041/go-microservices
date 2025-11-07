package main

import (
	"context"
	"database/sql" // <-- Use standard *sql.DB
	"encoding/json"
	"log"
	"net/http"
	"os"

	// This is the blank import to register the driver
	_ "github.com/ClickHouse/clickhouse-go/v2"

	// Update this to your go.mod module name
	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

type App struct {
	db *sql.DB // <-- Use the concurrent-safe pool
}

func main() {
    // ... (This is all correct) ...
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer conn.Close()
	log.Printf("Successfully connected to ClickHouse pool on %s", host)
	app := &App{db: conn}
	http.HandleFunc("/query", app.handleQuery)
	port := ":8081"
	log.Printf("Starting query service on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (app *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// --- THIS IS THE FIX ---
	// Change all column names to match the case in the CREATE TABLE command
	query := "SELECT Timestamp, Level, Source, Message FROM events ORDER BY Timestamp DESC LIMIT 10"
	// --- END OF FIX ---

	rows, err := app.db.QueryContext(context.Background(), query)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		// The Scan order must match the SELECT
		if err := rows.Scan(
			&event.Timestamp,
			&event.Level,
			&event.Source,
			&event.Message,
		); err != nil {
			log.Printf("Error scanning row: %v", err)
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		log.Printf("Error after row iteration: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(events)
}