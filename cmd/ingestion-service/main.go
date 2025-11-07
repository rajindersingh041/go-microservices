package main

import (
	"context"
	"encoding/json"
	"io"
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

func main() {
	// ... (main function is unchanged)
	conn, err := database.Connect()
	if err != nil {
		log.Fatalf("Failed to connect to Postgres: %v", err)
	}
	defer conn.Close()

	log.Println("Successfully connected to Postgres pool and schema is ready.")

	app := &App{db: conn}

	http.HandleFunc("/ingest", app.handleIngest)

	port := ":8080"
	log.Printf("Starting ingestion service on port %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// handleIngest is now "smart" and handles both single and batch events
func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// 1. Read the raw body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	var events []models.Event

	// --- THIS IS THE NEW "SMART" LOGIC ---
	// 2. Try to unmarshal as an array (batch) first
	err = json.Unmarshal(body, &events)
	if err != nil {
		// 3. If it's not an array, try to unmarshal as a single object
		var singleEvent models.Event
		err2 := json.Unmarshal(body, &singleEvent)
		if err2 != nil {
			// 4. If it's neither, the JSON is truly invalid
			log.Printf("Failed to decode JSON as array or object: %v", err)
			http.Error(w, "Failed to decode JSON: must be a single event object or an array of events", http.StatusBadRequest)
			return
		}
		
		// 5. It was a single object. Put it in the slice.
		events = []models.Event{singleEvent}
	}
	// --- END OF NEW LOGIC ---

	if len(events) == 0 {
		http.Error(w, "Received empty event batch", http.StatusBadRequest)
		return
	}

	// 6. The rest of the high-performance logic is UNCHANGED
	// It works perfectly with a slice of 1 or 1,000,000
	rows := make([][]interface{}, len(events))
	for i, e := range events {
		rows[i] = []interface{}{
			e.Timestamp,
			e.Level,
			e.Source,
			e.Message,
		}
	}

	tableName := pgx.Identifier{"events"}
	colNames := []string{"timestamp", "level", "source", "message"}

	copyCount, err := app.db.CopyFrom(
		context.Background(),
		tableName,
		colNames,
		pgx.CopyFromRows(rows),
	)

	if err != nil {
		log.Printf("Error during batch insert: %v", err)
		http.Error(w, "Server error during batch insert", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully ingested batch of %d events", copyCount)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "accepted",
		"ingested": copyCount,
	})
}