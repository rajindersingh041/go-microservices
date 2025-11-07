package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"

	// Make sure this matches your go.mod module name
	"github.com/rajindersingh041/go-microservices/internal/database"
)

type App struct {
	db *sql.DB
}

func main() {
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer conn.Close()

	// A query service is read-only, it does NOT create schema
	log.Println("Successfully connected to ClickHouse pool.")

	app := &App{db: conn}
	http.HandleFunc("/query/marketdata", app.handleQuery) // Define route

	port := ":8091" // Run on its own port
	log.Printf("Starting market data query service on %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

func (app *App) handleQuery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// Query the market_data table
	query := `
		SELECT TokenName, LastPrice, Timestamp
		FROM market_data
		ORDER BY Timestamp DESC
		LIMIT 10
	`
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	rows, err := app.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error executing query: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Define a simple struct for the response
	type QueryResponse struct {
			Token        string    `json:"token"`
			LastPrice    float64   `json:"last_price"`
			Timestamp    time.Time `json:"timestamp"`
			Volume       int64     `json:"volume"`
			AveragePrice float64   `json:"average_price"`
			NetChange    float64   `json:"net_change"`
		}

	var results []QueryResponse
		for rows.Next() {
			var res QueryResponse
			// --- UPDATED SCAN (Matches new query) ---
			if err := rows.Scan(
				&res.Token,
				&res.LastPrice,
				&res.Timestamp,
				&res.Volume,
				&res.AveragePrice,
				&res.NetChange,
			); err != nil {
				log.Printf("Error scanning row: %v", err)
				http.Error(w, "Server error", http.StatusInternalServerError)
				return
			}
			results = append(results, res)
		}
		if err := rows.Err(); err != nil { /* ... */ }

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(results)
}