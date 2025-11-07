package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	// We need to import the models to create test data
	"github.com/rajindersingh041/go-microservices/internal/models"
)

var (
	// Base URLs for all services
	ingestEventsURL    = "http://localhost:8080/ingest/events"
	queryEventsURL     = "http://localhost:8090/query/events"
	ingestMarketURL    = "http://localhost:8081/ingest/marketdata"
	queryMarketURL     = "http://localhost:8091/query/marketdata"
	
	client = &http.Client{Timeout: 10 * time.Second}
	
	// A unique ID for this test run
	testRunID = fmt.Sprintf("test-run-%d", time.Now().UnixNano())
)

// Main test runner
func main() {
	log.Println("--- Starting System Smoke Test ---")

	if err := testEventPipeline(); err != nil {
		log.Fatalf("--- TEST FAILED: Event Pipeline --- \n%v", err)
	}

	if err := testMarketDataPipeline(); err != nil {
		log.Fatalf("--- TEST FAILED: Market Data Pipeline --- \n%v", err)
	}

	if err := testPollerLogs(); err != nil {
		log.Fatalf("--- TEST FAILED: Poller Logging --- \n%v", err)
	}

	log.Println("--- ALL TESTS PASSED ---")
}

// Test 1: Can we ingest and query a single event?
func testEventPipeline() error {
	log.Println("Running Test 1: Event Pipeline...")
	
	// --- 1. INGEST ---
	log.Println("  Sub-test: Ingesting single event...")
	testEvent := models.Event{
		Timestamp: time.Now(),
		Level:     "TEST",
		Source:    "system-test",
		Message:   testRunID, // Use the unique ID
		Context:   map[string]string{"test": "true"},
	}
	
	payload, _ := json.Marshal(testEvent) // Use single object, 'ingest-events' is smart
	resp, err := client.Post(ingestEventsURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to POST event: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ingest-events returned non-202 status: %s", resp.Status)
	}
	log.Println("  Sub-test: Ingest... PASS")

	// --- 2. QUERY ---
	log.Println("  Sub-test: Querying for event...")
	// We might need to wait a moment for ClickHouse to process
	time.Sleep(1 * time.Second)
	
	resp, err = client.Get(queryEventsURL)
	if err != nil {
		return fmt.Errorf("failed to GET events: %v", err)
	}
	
	var events []models.Event
	if err := json.NewDecoder(resp.Body).Decode(&events); err != nil {
		return fmt.Errorf("failed to decode events JSON: %v", err)
	}
	
	if len(events) == 0 {
		return fmt.Errorf("no events returned from query")
	}

	// Check if the top result is our test event
	if events[0].Message != testRunID {
		return fmt.Errorf("failed to find test event: expected message %s, got %s", testRunID, events[0].Message)
	}
	log.Println("  Sub-test: Query... PASS")
	log.Println("Test 1: Event Pipeline... PASS")
	return nil
}

// Test 2: Can we ingest and query complex market data (with nil ohlc)?
func testMarketDataPipeline() error {
	log.Println("Running Test 2: Market Data Pipeline...")

	// --- 1. INGEST ---
	log.Println("  Sub-test: Ingesting market data...")
	// Use the exact JSON that has one nil 'ohlc'
	marketDataJSON := `{
		"data": {
			"request_id": "WUPW-test-run", "time_in_millis": 1762521758822,
			"token_data": {
				"TEST|Nifty 50": {
					"timestamp": "2025-11-07 06:52:38", "lastTradeTime": "2025-11-07 04:00:00",
					"lastPrice": 12345.6, "closePrice": 25509.7, "netChange": -17.4,
					"ohlc": { "open": 25433.8, "high": 25551.25, "low": 25318.45, "close": 25509.7, "volume": 0 }
				},
				"TEST|RELIANCE": {
					"timestamp": "2025-11-07 07:33:06", "lastTradeTime": "1970-01-01 05:30:00",
					"lastPrice": 789.0, "closePrice": 0.0, "netChange": 0.0
				}
			}
		},
		"success": true
	}`
	
	resp, err := client.Post(ingestMarketURL, "application/json", strings.NewReader(marketDataJSON))
	if err != nil {
		return fmt.Errorf("failed to POST market data: %v", err)
	}
	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("ingest-marketdata returned non-202 status: %s", resp.Status)
	}
	log.Println("  Sub-test: Ingest... PASS")

	// --- 2. QUERY ---
	log.Println("  Sub-test: Querying for market data...")
	time.Sleep(1 * time.Second)
	
	resp, err = client.Get(queryMarketURL)
	if err != nil {
		return fmt.Errorf("failed to GET market data: %v", err)
	}
	
	// Define the expected response struct from our query
	type QueryResponse struct {
		Token     string    `json:"token"`
		LastPrice float64   `json:"last_price"`
		Timestamp time.Time `json:"timestamp"`
	}
	var results []QueryResponse
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return fmt.Errorf("failed to decode market data JSON: %v", err)
	}
	
	if len(results) < 2 {
		return fmt.Errorf("expected at least 2 market data results, got %d", len(results))
	}
	
	// Find our test data (order might vary)
	found := 0
	for _, res := range results {
		if res.Token == "TEST|RELIANCE" && res.LastPrice == 789.0 {
			found++
		}
		if res.Token == "TEST|Nifty 50" && res.LastPrice == 12345.6 {
			found++
		}
	}
	
	if found != 2 {
		return fmt.Errorf("failed to find both test market data entries")
	}
	log.Println("  Sub-test: Query... PASS")
	log.Println("Test 2: Market Data Pipeline... PASS")
	return nil
}

// Test 3: Can we see logs from the poller?
func testPollerLogs() error {
	log.Println("Running Test 3: Poller Logging...")
	log.Println("  (This test assumes the poller has run at least once)")
	
	resp, err := client.Get(queryEventsURL)
	if err != nil {
		return fmt.Errorf("failed to GET events: %v", err)
	}
	
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("market-poller")) {
		return fmt.Errorf("failed to find any logs from 'market-poller' in the event table")
	}

	log.Println("Test 3: Poller Logging... PASS")
	return nil
}