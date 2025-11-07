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

	// A query service does NOT own schema
	log.Println("Connected to ClickHouse.")

	app := &App{db: conn}
	http.HandleFunc("/query/events", app.handleQuery) // Define route
	port := ":8090" // Run on a new port
	log.Printf("Starting event query service on %s...", port)
	http.ListenAndServe(port, nil)
}

func (app *App) handleQuery(w http.ResponseWriter, r *http.Request) {
    // ... (This is your 'events' query logic)
	query := "SELECT Timestamp, Level, Source, Message FROM events ORDER BY Timestamp DESC LIMIT 10"
	rows, err := app.db.Query(query)
	if err != nil { /* ... */ }
	defer rows.Close()

	var events []models.Event
	for rows.Next() {
		var event models.Event
		if err := rows.Scan(
			&event.Timestamp, &event.Level, &event.Source, &event.Message,
		); err != nil { /* ... */ }
		events = append(events, event)
	}
	
	json.NewEncoder(w).Encode(events)
}