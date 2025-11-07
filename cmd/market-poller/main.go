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
	Instruments       []string
	UpstoxURL         string
	IngestMarketURL   string // Renamed
	IngestEventsURL   string // New
	Interval          time.Duration
	Loc               *time.Location
	StartTime         time.Time
	EndTime           time.Time
	HttpClient        *http.Client
}

// loadConfig loads and parses all config from env
func loadConfig() (*Config, error) {
	// Load simple strings
	instrumentsStr := os.Getenv("POLLER_INSTRUMENTS")
	upstoxURL := os.Getenv("UPSTOX_BASE_URL")
	ingestMarketURL := os.Getenv("POLLER_INGEST_MARKET_URL") // Renamed
	ingestEventsURL := os.Getenv("POLLER_INGEST_EVENTS_URL") // New
	intervalStr := os.Getenv("POLLER_INTERVAL")
	tzStr := os.Getenv("POLLER_TIMEZONE")
	startStr := os.Getenv("POLLER_START_TIME")
	endStr := os.Getenv("POLLER_END_TIME")

	// --- Check for missing env vars ---
	if ingestMarketURL == "" || ingestEventsURL == "" {
		return nil, fmt.Errorf("POLLER_INGEST_MARKET_URL and POLLER_INGEST_EVENTS_URL must be set in .env")
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLLER_INTERVAL: %w", err)
	}

	loc, err := time.LoadLocation(tzStr)
	if err != nil {
		return nil, fmt.Errorf("invalid POLLER_TIMEZONE: %w", err)
	}

	nowInLoc := time.Now().In(loc)
	startTime, err := time.ParseInLocation("15:04", startStr, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid POLLER_START_TIME: %w", err)
	}
	startTime = startTime.AddDate(nowInLoc.Year(), int(nowInLoc.Month())-1, nowInLoc.Day()-1)

	endTime, err := time.ParseInLocation("15:04", endStr, loc)
	if err != nil {
		return nil, fmt.Errorf("invalid POLLER_END_TIME: %w", err)
	}
	endTime = endTime.AddDate(nowInLoc.Year(), int(nowInLoc.Month())-1, nowInLoc.Day()-1)

	return &Config{
		Instruments:       strings.Split(instrumentsStr, ","),
		UpstoxURL:         upstoxURL,
		IngestMarketURL:   ingestMarketURL, // Renamed
		IngestEventsURL:   ingestEventsURL, // New
		Interval:          interval,
		Loc:               loc,
		StartTime:         startTime,
		EndTime:           endTime,
		HttpClient:        &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// isMarketOpen()... (UNCHANGED)
func (c *Config) isMarketOpen() bool {
	now := time.Now().In(c.Loc)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return false
	}
	start := c.StartTime.AddDate(now.Year()-c.StartTime.Year(), int(now.Month()-c.StartTime.Month()), now.Day()-c.StartTime.Day())
	end := c.EndTime.AddDate(now.Year()-c.EndTime.Year(), int(now.Month()-c.EndTime.Month()), now.Day()-c.EndTime.Day())
	return now.After(start) && now.Before(end)
}

// main()... (UNCHANGIT)
func main() {
	log.Println("--- Starting Market Poller Service ---")
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}
	log.Printf("Loaded %d instruments. Fetching every %s.", len(cfg.Instruments), cfg.Interval)
	log.Printf("Time window: %s-%s (%s).",
		cfg.StartTime.Format("15:04"), cfg.EndTime.Format("15:04"), cfg.Loc)
	ticker := time.NewTicker(cfg.Interval)
	defer ticker.Stop()
	runFetchCycle(cfg)
	for range ticker.C {
		runFetchCycle(cfg)
	}
}

// logEvent helper function (Now with nil-safe context)
func (c *Config) logEvent(level, message string, context map[string]string) {
	if context == nil {
		context = make(map[string]string)
	}

	event := models.Event{
		Timestamp: time.Now(),
		Level:     level,
		Source:    "market-poller",
		Message:   message,
		Context:   context,
	}

	payload, err := json.Marshal([]models.Event{event})
	if err != nil {
		log.Printf("WARN: Failed to marshal log event: %v", err)
		return
	}
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

// --- THIS IS THE UPDATED FETCH FUNCTION ---
func runFetchCycle(cfg *Config) {
	// Generate a unique ID for this fetch cycle
	requestID := fmt.Sprintf("poller-%d", time.Now().UnixNano())

	if !cfg.isMarketOpen() {
		log.Println("Market is closed. Sleeping.")
		cfg.logEvent("INFO", "Market is closed. Sleeping.", map[string]string{
			"request_id": requestID,
		})
		return
	}

	log.Println("Market is open. Fetching data for all instruments in one batch...")
	instrumentQuery := strings.Join(cfg.Instruments, ",")
	fetchURL := fmt.Sprintf("%s?i=%s&interval=1m", cfg.UpstoxURL, url.QueryEscape(instrumentQuery))

	log.Printf("[%s] Fetching URL: %s", requestID, fetchURL)
	cfg.logEvent("INFO", "Attempting to fetch data", map[string]string{
		"instrument_count": fmt.Sprintf("%d", len(cfg.Instruments)),
		"url":              fetchURL,
		"request_id":       requestID,
	})

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		log.Printf("ERROR: [%s] Failed to create batch request: %v", requestID, err)
		cfg.logEvent("ERROR", "Failed to create batch request", map[string]string{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	// --- THIS IS THE FIX ---
	// Add all the custom headers you requested
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36")
	req.Header.Set("X-Request-ID", requestID) // Use our unique, traceable ID
	// You will still need an Authorization header for a 200 OK
	// req.Header.Set("Authorization", "Bearer YOUR_ACCESS_TOKEN")
	// --- END OF FIX ---

	resp, err := cfg.HttpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: [%s] Failed to fetch batch data: %v", requestID, err)
		cfg.logEvent("ERROR", "Failed to fetch batch data", map[string]string{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: [%s] Failed to read batch response body: %v", requestID, err)
		cfg.logEvent("ERROR", "Failed to read batch response body", map[string]string{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("WARN: [%s] Got non-200 response for batch: %s (Body: %s)", requestID, resp.Status, string(body))
		cfg.logEvent("WARN", "Non-OK response from Upstox", map[string]string{
			"http_status": fmt.Sprintf("%d", resp.StatusCode),
			"url":         fetchURL,
			"body":        string(body),
			"request_id":  requestID,
		})
		return
	}

	// Ingest the data
	ingestResp, err := cfg.HttpClient.Post(cfg.IngestMarketURL, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("ERROR: [%s] Failed to ingest batch data: %v", requestID, err)
		cfg.logEvent("ERROR", "Failed to ingest batch data", map[string]string{
			"error":      err.Error(),
			"request_id": requestID,
		})
		return
	}
	defer ingestResp.Body.Close()

	if ingestResp.StatusCode != http.StatusAccepted {
		log.Printf("ERROR: [%s] Ingest service gave non-202 status: %s", requestID, ingestResp.Status)
		cfg.logEvent("ERROR", "Ingest service gave non-202 status", map[string]string{
			"http_status": fmt.Sprintf("%d", ingestResp.StatusCode),
			"request_id":  requestID,
		})
		return
	}

	log.Printf("[%s] Successfully fetched and ingested data for all instruments", requestID)
	cfg.logEvent("INFO", "Successfully ingested data", map[string]string{
		"instrument_count": fmt.Sprintf("%d", len(cfg.Instruments)),
		"request_id":       requestID,
	})
}