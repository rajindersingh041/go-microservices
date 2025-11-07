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

	"github.com/rajindersingh041/go-microservices/internal/database"
	"github.com/rajindersingh041/go-microservices/internal/models"
)

/*
MARKET DATA INGESTION MICROSERVICE
=================================
Purpose: This service handles the ingestion of real-time market data from external APIs.

Domain Separation: This service is completely separate from events
- Different data model (financial vs log data)
- Different performance characteristics (high-frequency updates)
- Different scaling requirements (market hours vs 24/7)
- Independent development and deployment

Financial Data Considerations:
1. Time-series optimized storage (ClickHouse MergeTree)
2. High-frequency updates during market hours
3. Complex data structures (OHLC, volumes, open interest)
4. Data integrity critical for trading decisions

Scaling Strategies:
1. Market Hours Scaling: Auto-scale during trading hours
2. Symbol Partitioning: Separate services per market/exchange
3. Real-time Streaming: Apache Kafka for high-throughput ingestion
4. Data Deduplication: Handle duplicate market data from sources
*/

// Database Schema: Optimized for financial time-series data
// ORDER BY (TokenName, Timestamp): Enables efficient queries by symbol and time
// DateTime64(3): Millisecond precision for accurate trade timing
const initMarketDataSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.market_data (
    RequestID         String,           -- Unique identifier for API request
    ResponseTime      DateTime64(3),    -- When data was received (millisecond precision)
    TokenName         String,           -- Financial instrument identifier
    Timestamp         DateTime,         -- Market data timestamp
    LastTradeTime     DateTime,         -- When last trade occurred
    LastPrice         Float64,          -- Current market price
    ClosePrice        Float64,          -- Previous day's closing price
    LastQuantity      Int64,            -- Quantity of last trade
    BuyQuantity       Float64,          -- Total buy side quantity
    SellQuantity      Float64,          -- Total sell side quantity
    Volume            Int64,            -- Total volume traded
    AveragePrice      Float64,          -- Volume-weighted average price
    Oi                Float64,          -- Open Interest (derivatives)
    Poi               Float64,          -- Previous Open Interest
    OiDayHigh         Float64,          -- Day's highest open interest
    OiDayLow          Float64,          -- Day's lowest open interest
    NetChange         Float64,          -- Price change from previous close
    LowerCircuitLimit Float64,          -- Lower price limit (regulatory)
    UpperCircuitLimit Float64,          -- Upper price limit (regulatory)
    Yl                Float64,          -- Year Low
    Yh                Float64,          -- Year High
    OhlcOpen          Float64,          -- Day's opening price
    OhlcHigh          Float64,          -- Day's highest price
    OhlcLow           Float64,          -- Day's lowest price
    OhlcClose         Float64,          -- Current/last price
    OhlcVolume        Int64             -- Day's total volume
) ENGINE = MergeTree()
ORDER BY (TokenName, Timestamp);      -- Optimized for symbol-time queries
`

// App manages database connections and HTTP handlers for market data
type App struct {
	db *sql.DB // Connection pool for high-frequency market data inserts
}

func main() {
	// Database Connection: Connect to ClickHouse for market data storage
	// ClickHouse is ideal for financial data due to:
	// - Columnar storage (efficient for analytical queries)
	// - High compression (financial data has patterns)
	// - Fast aggregations (OHLC calculations, volume analysis)
	host := os.Getenv("CLICKHOUSE_HOST")
	conn, err := database.Connect(host)
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}
	defer conn.Close()

	// Schema Ownership: This service owns the market_data table
	// Database Per Service pattern: Each microservice manages its own data
	// Benefits: Independent scaling, schema evolution, fault isolation
	if _, err := conn.Exec(initMarketDataSQL); err != nil {
		log.Fatalf("Failed to initialize market_data schema: %v", err)
	}
	log.Println("Market data table initialized and ready for ingestion.")

	// Application Initialization
	app := &App{db: conn}
	
	// Route Setup: Define API endpoint for market data ingestion
	// RESTful convention: POST /ingest/marketdata
	http.HandleFunc("/ingest/marketdata", app.handleIngest)
	
	// Service Port: Port 8081 dedicated to market data ingestion
	// Port allocation strategy:
	// - 8080-8089: Ingestion services
	// - 8090-8099: Query services
	// - 8100+: Background/utility services
	port := ":8081"
	log.Printf("Starting market data ingestion service on %s...", port)
	
	// HTTP Server: Start listening for market data API requests
	// In production, add:
	// - Graceful shutdown handling
	// - Health check endpoints
	// - Metrics collection
	// - Request timeout configurations
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
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