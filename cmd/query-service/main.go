package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	// Update this to your go.mod module name
	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

// App holds the concurrent-safe connection pool
type App struct {
	db *pgxpool.Pool
}

// Includes the fix: func main()
func main() {
	// Connect to Postgres pool (also runs init sql)
	conn, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer conn.Close() // Closes the pool on shutdown

	log.Println("Successfully connected to Postgres pool and schema is ready.")

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

	query := "SELECT Timestamp, Level, Source, Message FROM events ORDER BY Timestamp DESC LIMIT 10"

	// app.db.Query() is concurrency-safe
	rows, err := app.db.Query(context.Background(), query)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// working now, earlier wasnot working
	events, err := pgx.CollectRows[models.Event](rows, pgx.RowToStructByName[models.Event])
		
	if err != nil {
		log.Printf("Error scanning rows: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(events)
}