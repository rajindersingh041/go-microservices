package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

// This must match the model in your main project
type Event struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Source    string    `json:"source"`
	Message   string    `json:"message"`
}

var (
	// Counters
	ingestSuccess atomic.Uint64
	ingestFailure atomic.Uint64
	querySuccess  atomic.Uint64
	queryFailure  atomic.Uint64

	// Test parameters
	numIngestBatches = flag.Int("ingest-batches", 50, "Number of ingest batches to send")
	eventsPerBatch   = flag.Int("events-per-batch", 100, "Number of events per batch")
	numQueries       = flag.Int("queries", 100, "Number of concurrent query requests")
	ingestURL        = flag.String("ingest-url", "http://localhost:8080/ingest", "Ingestion service URL")
	queryURL         = flag.String("query-url", "http://localhost:8081/query", "Query service URL")

	// HTTP client with timeout
	client = &http.Client{
		Timeout: 30 * time.Second,
	}

	levels = []string{"INFO", "WARN", "ERROR", "DEBUG"}
	sources = []string{"payment-svc", "auth-svc", "cart-svc", "frontend"}
)

// generateBatch creates a slice of random events
func generateBatch(n int) []Event {
	events := make([]Event, n)
	for i := 0; i < n; i++ {
		events[i] = Event{
			Timestamp: time.Now(),
			Level:     levels[rand.Intn(len(levels))],
			Source:    sources[rand.Intn(len(sources))],
			Message:   fmt.Sprintf("This is a test event number %d", rand.Intn(100000)),
		}
	}
	return events
}

// runIngestWorker sends one batch of events
func runIngestWorker(wg *sync.WaitGroup) {
	defer wg.Done()

	// 1. Generate a batch of data
	batch := generateBatch(*eventsPerBatch)
	payload, err := json.Marshal(batch)
	if err != nil {
		log.Printf("Failed to marshal JSON: %v", err)
		ingestFailure.Add(uint64(*eventsPerBatch)) // Count all events as failed
		return
	}

	// 2. Send the POST request
	req, err := http.NewRequest("POST", *ingestURL, bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("Failed to create request: %v", err)
		ingestFailure.Add(uint64(*eventsPerBatch))
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Ingest request failed: %v", err)
		ingestFailure.Add(uint64(*eventsPerBatch))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		log.Printf("Ingest request got non-202 status: %s", resp.Status)
		ingestFailure.Add(uint64(*eventsPerBatch))
		return
	}

	// Success!
	ingestSuccess.Add(uint64(*eventsPerBatch))
}

// runQueryWorker sends one query request
func runQueryWorker(wg *sync.WaitGroup) {
	defer wg.Done()

	resp, err := client.Get(*queryURL)
	if err != nil {
		log.Printf("Query request failed: %v", err)
		queryFailure.Add(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Query request got non-200 status: %s", resp.Status)
		queryFailure.Add(1)
		return
	}

	// Success!
	querySuccess.Add(1)
}

func main() {
	flag.Parse()

	totalEvents := *numIngestBatches * *eventsPerBatch
	log.Printf("--- Starting Load Test ---")
	log.Printf("Query Service: %d concurrent requests", *numQueries)
	log.Printf("Ingest Service: %d batches @ %d events/batch (Total: %d events)", *numIngestBatches, *eventsPerBatch, totalEvents)
	log.Printf("----------------------------")

	startTime := time.Now()
	var wg sync.WaitGroup

	// --- Spawn all goroutines ---

	// Spawn query workers
	wg.Add(*numQueries)
	for i := 0; i < *numQueries; i++ {
		go runQueryWorker(&wg)
	}

	// Spawn ingest workers
	wg.Add(*numIngestBatches)
	for i := 0; i < *numIngestBatches; i++ {
		go runIngestWorker(&wg)
	}

	// --- Wait for all to finish ---
	log.Println("All workers spawned, waiting for completion...")
	wg.Wait()
	duration := time.Since(startTime)

	// --- Print Results ---
	log.Printf("--- Test Complete ---")
	log.Printf("Duration: %s", duration)
	log.Println("---")
	log.Printf("Query Service Results:")
	log.Printf("  Success: %d", querySuccess.Load())
	log.Printf("  Failure: %d", queryFailure.Load())
	log.Printf("  Rate:    %.2f req/s", float64(querySuccess.Load())/duration.Seconds())
	log.Println("---")
	log.Printf("Ingest Service Results:")
	log.Printf("  Success: %d events", ingestSuccess.Load())
	log.Printf("  Failure: %d events", ingestFailure.Load())
	log.Printf("  Rate:    %.2f events/s", float64(ingestSuccess.Load())/duration.Seconds())
}