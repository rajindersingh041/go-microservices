package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	// We need our models package
	"github.com/rajindersingh041/go-microservices/internal/models"
)

// Config holds our settings
type Config struct {
	Interval   time.Duration
	SourceURL  string
	DestURL    string
	HttpClient *http.Client
}

// This is the struct we expect to GET from the query service
type SourceData struct {
	Token     string    `json:"token"`
	LastPrice float64   `json:"last_price"`
	Timestamp time.Time `json:"timestamp"`
}

func main() {
	log.Println("--- Starting Internal Transformer Service ---")

	// --- FIX: Add default value and error checking ---
	intervalStr := os.Getenv("TRANSFORMER_INTERVAL")
	if intervalStr == "" {
		intervalStr = "1m" // Provide a safe default
		log.Println("WARN: TRANSFORMER_INTERVAL not set, using default '1m'")
	}

	interval, err := time.ParseDuration(intervalStr)
	if err != nil {
		log.Fatalf("FATAL: Invalid TRANSFORMER_INTERVAL: %v", err)
	}
	// --- END OF FIX ---

	sourceURL := os.Getenv("TRANSFORMER_SOURCE_URL")
	destURL := os.Getenv("TRANSFORMER_DEST_URL")
	if sourceURL == "" || destURL == "" {
		log.Fatalf("FATAL: TRANSFORMER_SOURCE_URL and TRANSFORMER_DEST_URL must be set")
	}

	cfg := &Config{
		Interval:   interval,
		SourceURL:  sourceURL,
		DestURL:    destURL,
		HttpClient: &http.Client{Timeout: 10 * time.Second},
	}
	
	log.Printf("Starting transform cycle every %s", cfg.Interval) // This will no longer be 0s
	log.Printf("  Source: %s", cfg.SourceURL)
	log.Printf("  Destination: %s", cfg.DestURL)
	
	ticker := time.NewTicker(cfg.Interval) // This line will no longer panic
	defer ticker.Stop()

	// Run first cycle immediately
	runTransformCycle(cfg)
	
	for range ticker.C {
		runTransformCycle(cfg)
	}
}

func runTransformCycle(cfg *Config) {
	log.Println("Running transform cycle...")

	// 1. EXTRACT: Fetch data from our own query service
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	req, _ := http.NewRequestWithContext(ctx, "GET", cfg.SourceURL, nil)
	resp, err := cfg.HttpClient.Do(req)
	if err != nil {
		log.Printf("ERROR: Failed to fetch from query service: %v", err)
		return
	}
	
	var sourceData []SourceData
	if err := json.NewDecoder(resp.Body).Decode(&sourceData); err != nil {
		log.Printf("ERROR: Failed to decode JSON from query service: %v", err)
		resp.Body.Close()
		return
	}
	resp.Body.Close()

	if len(sourceData) == 0 {
		log.Println("No market data found. Skipping ingest.")
		return
	}

	// 2. TRANSFORM: Convert this data into a new "Event"
	// We will create one log event for the whole batch
	message := fmt.Sprintf("Successfully queried %d market data entries.", len(sourceData))
	
	// We can add the raw data to the context
	rawData, _ := json.Marshal(sourceData)
	
	event := models.Event{
		Timestamp: time.Now(),
		Level:     "INFO",
		Source:    "internal-transformer",
		Message:   message,
		Context: map[string]string{
			"source_url":   cfg.SourceURL,
			"items_found":  fmt.Sprintf("%d", len(sourceData)),
			"raw_data_ex": string(rawData), // Add the data as a string
		},
	}
	
	// 3. LOAD: Ingest this new event into the events service
	payload, _ := json.Marshal([]models.Event{event}) // Send as a batch of 1
	
	resp, err = cfg.HttpClient.Post(cfg.DestURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("ERROR: Failed to ingest event: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Printf("ERROR: Ingest-events service gave non-202 status: %s", resp.Status)
		return
	}

	log.Println("Successfully ran transform cycle and ingested new event.")
}