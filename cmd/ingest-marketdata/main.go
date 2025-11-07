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

	// Make sure this matches your go.mod
	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

// This service OWNS the 'market_data' table
const initMarketDataSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.market_data (
    RequestID         String,
    ResponseTime      DateTime64(3),
    TokenName         String,
    Timestamp         DateTime,
    LastTradeTime     DateTime,
    LastPrice         Float64,
    ClosePrice        Float64,
    LastQuantity      Int64,
    BuyQuantity       Float64,
    SellQuantity      Float64,
    Volume            Int64,
    AveragePrice      Float64,
    Oi                Float64,
    Poi               Float64,
    OiDayHigh         Float64,
    OiDayLow          Float64,
    NetChange         Float64,
    LowerCircuitLimit Float64,
    UpperCircuitLimit Float64,
    Yl                Float64,
    Yh                Float64,
    OhlcOpen          Float64,
    OhlcHigh          Float64,
    OhlcLow           Float64,
    OhlcClose         Float64,
    OhlcVolume        Int64
) ENGINE = MergeTree()
ORDER BY (TokenName, Timestamp);
`

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
	if _, err := conn.Exec(initMarketDataSQL); err != nil {
		log.Fatalf("Failed to init 'market_data' schema: %v", err)
	}
	log.Println("'market_data' table is ready.") // <-- You will see this log now

	app := &App{db: conn}
	http.HandleFunc("/ingest/marketdata", app.handleIngest) // Define route
	
	port := ":8081" // Run on a different port
	log.Printf("Starting market data ingestion service on %s...", port) // <-- And this one
	
	// --- THIS IS THE MISSING, CRITICAL LINE ---
	// It blocks forever and runs the server.
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	// --- END OF FIX ---
}
func (app *App) handleIngest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	var apiResp models.ApiResponse
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading body: %v", err)
		http.Error(w, "Failed to read body", http.StatusInternalServerError)
		return
	}
	
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("Failed to decode JSON: %v", err)
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		return
	}

	tx, err := app.db.Begin()
	if err != nil {
		log.Printf("Error beginning transaction: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	// --- UPDATED INSERT STATEMENT (Now has 26 columns) ---
	stmt, err := tx.PrepareContext(context.Background(), `
		INSERT INTO market_data (
			RequestID, ResponseTime, TokenName, Timestamp, LastTradeTime,
			LastPrice, ClosePrice, LastQuantity, BuyQuantity, SellQuantity,
			Volume, AveragePrice, Oi, Poi, OiDayHigh, OiDayLow,
			NetChange, LowerCircuitLimit, UpperCircuitLimit, Yl, Yh,
			OhlcOpen, OhlcHigh, OhlcLow, OhlcClose, OhlcVolume
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?,
			?, ?, ?, ?, ?
		)
	`)
	if err != nil {
		log.Printf("Error preparing statement: %v", err)
		tx.Rollback()
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	responseTime := time.UnixMilli(apiResp.Data.TimeInMillis)
	requestID := apiResp.Data.RequestID

	for tokenName, tokenInfo := range apiResp.Data.TokenData {
		ts, _ := time.Parse("2006-01-02 15:04:05", tokenInfo.Timestamp)
		
		lttStr := tokenInfo.LastTradeTime
		if lttStr == "1970-01-01 05:30:00" {
			lttStr = "1970-01-01 00:00:00"
		}
		ltt, _ := time.Parse("2006-01-02 15:04:05", lttStr)

		var ohlcOpen, ohlcHigh, ohlcLow, ohlcClose float64
		var ohlcVolume int64
		if tokenInfo.Ohlc != nil {
			ohlcOpen = tokenInfo.Ohlc.Open
			ohlcHigh = tokenInfo.Ohlc.High
			ohlcLow = tokenInfo.Ohlc.Low
			ohlcClose = tokenInfo.Ohlc.Close
			ohlcVolume = tokenInfo.Ohlc.Volume
		}

		// --- UPDATED ExecContext (Now has 26 parameters) ---
		if _, err := stmt.ExecContext(context.Background(),
			requestID, responseTime, tokenName, ts, ltt,
			tokenInfo.LastPrice, tokenInfo.ClosePrice, tokenInfo.LastQuantity,
			tokenInfo.BuyQuantity, tokenInfo.SellQuantity, tokenInfo.Volume,
			tokenInfo.AveragePrice, tokenInfo.Oi, tokenInfo.Poi,
			tokenInfo.OiDayHigh, tokenInfo.OiDayLow, tokenInfo.NetChange,
			tokenInfo.LowerCircuitLimit, tokenInfo.UpperCircuitLimit,
			tokenInfo.Yl, tokenInfo.Yh,
			ohlcOpen, ohlcHigh, ohlcLow, ohlcClose, ohlcVolume,
		); err != nil {
			log.Printf("Error executing batch: %v", err)
			tx.Rollback()
			http.Error(w, "Server error", http.StatusInternalServerError)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, "Server error", http.StatusInternalServerError)
		return
	}

	ingestedCount := len(apiResp.Data.TokenData)
	log.Printf("Successfully ingested batch of %d market tokens", ingestedCount)
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "accepted",
		"ingested": ingestedCount,
	})
}