package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/rajindersingh041/go-microservices/internal/models"
)

// Config holds all config from environment variables
type Config struct {
	UpstoxURL       string
	IngestMarketURL string
	IngestEventsURL string
	HttpClient      *http.Client
}

// This is the JSON body we expect from the user
type FetchRequest struct {
	Instruments []string `json:"instruments"`
}

// loadConfig loads *only* the URLs we need
func loadConfig() (*Config, error) {
	upstoxURL := os.Getenv("UPSTOX_BASE_URL")
	ingestMarketURL := os.Getenv("POLLER_INGEST_MARKET_URL")
	ingestEventsURL := os.Getenv("POLLER_INGEST_EVENTS_URL")

	if ingestMarketURL == "" || ingestEventsURL == "" {
		return nil, fmt.Errorf("POLLER_INGEST_MARKET_URL and POLLER_INGEST_EVENTS_URL must be set in .env")
	}

	return &Config{
		UpstoxURL:       upstoxURL,
		IngestMarketURL: ingestMarketURL,
		IngestEventsURL: ingestEventsURL,
		HttpClient:      &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// logEvent helper function (copied from poller)
func (c *Config) logEvent(level, message string, context map[string]string) {
	// ... (This function is identical to the one in market-poller)
	if context == nil {
		context = make(map[string]string)
	}
	event := models.Event{
		Timestamp: time.Now(),
		Level:     level,
		Source:    "on-demand-fetcher", // <-- New source
		Message:   message,
		Context:   context,
	}
	payload, _ := json.Marshal([]models.Event{event})
	resp, err := c.HttpClient.Post(c.IngestEventsURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("WARN: Failed to POST log to events service: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		log.Printf("WARN: Events service gave non-202 status for log: %s", resp.Status)
	}
}

// --- THIS IS THE MAIN HTTP HANDLER ---
func (app *App) handleFetch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

	// 1. Decode the user's request (e.g., from Postman)
	var reqBody FetchRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid JSON body. Expected {\"instruments\": [\"...\"]}", http.StatusBadRequest)
		return
	}

	if len(reqBody.Instruments) == 0 {
		http.Error(w, "No instruments provided in request", http.StatusBadRequest)
		return
	}

	// Generate a unique ID for this fetch
	requestID := fmt.Sprintf("manual-fetch-%d", time.Now().UnixNano())

	// 2. Run the Fetch-and-Ingest logic
	// (This is the same logic from the poller)
	instrumentQuery := strings.Join(reqBody.Instruments, ",")
	fetchURL := fmt.Sprintf("%s?i=%s&interval=1m", app.cfg.UpstoxURL, url.QueryEscape(instrumentQuery))

	log.Printf("[%s] Manual fetch requested for: %s", requestID, instrumentQuery)
	app.cfg.logEvent("INFO", "Manual fetch request received", map[string]string{
		"instrument_count": fmt.Sprintf("%d", len(reqBody.Instruments)),
		"url":              fetchURL,
		"request_id":       requestID,
	})

	fetchReq, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil { /* ... handle error & log ... */ }

	// Add all required headers
	fetchReq.Header.Set("Accept", "application/json, text/plain, */*")
	fetchReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	fetchReq.Header.Set("X-Request-ID", requestID)
	// fetchReq.Header.Set("Authorization", "Bearer YOUR_ACCESS_TOKEN")

	fetchResp, err := app.cfg.HttpClient.Do(fetchReq)
	if err != nil { /* ... handle error & log ... */ }
	defer fetchResp.Body.Close()

	body, err := io.ReadAll(fetchResp.Body)
	if err != nil { /* ... handle error & log ... */ }

	if fetchResp.StatusCode != http.StatusOK {
		log.Printf("WARN: [%s] Upstox API gave non-200: %s", requestID, fetchResp.Status)
		app.cfg.logEvent("WARN", "Non-OK response from Upstox", map[string]string{
			"http_status": fmt.Sprintf("%d", fetchResp.StatusCode),
			"body":        string(body),
			"request_id":  requestID,
		})
		http.Error(w, "Failed to fetch from Upstox API: "+string(body), fetchResp.StatusCode)
		return
	}

	// 3. Ingest the data
	ingestResp, err := app.cfg.HttpClient.Post(app.cfg.IngestMarketURL, "application/json", bytes.NewBuffer(body))
	if err != nil { /* ... handle error & log ... */ }
	defer ingestResp.Body.Close()

	if ingestResp.StatusCode != http.StatusAccepted {
		log.Printf("ERROR: [%s] Ingest service gave non-202 status: %s", requestID, ingestResp.Status)
		app.cfg.logEvent("ERROR", "Ingest service failed", map[string]string{
			"http_status": fmt.Sprintf("%d", ingestResp.StatusCode),
			"request_id":  requestID,
		})
		http.Error(w, "Failed to ingest data internally", http.StatusInternalServerError)
		return
	}

	// 4. Return success to the user
	app.cfg.logEvent("INFO", "Successfully executed manual fetch", map[string]string{
		"request_id": requestID,
	})
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":    true,
		"message":    "Successfully fetched and ingested data for " + instrumentQuery,
		"request_id": requestID,
	})
}

// App holds the config
type App struct {
	cfg *Config
}

func main() {
	log.Println("--- Starting On-Demand Fetcher Service ---")
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	
	app := &App{cfg: cfg}

	http.HandleFunc("/fetch/marketdata", app.handleFetch)
	
	port := ":8079"
	log.Printf("Listening on %s...", port)
	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}