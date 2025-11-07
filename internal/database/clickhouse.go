package database

import (
	"context"
	"database/sql" // <-- Use standard database/sql
	"fmt"
	"log"
	"os"
	"time"
	// We still need this
)

const initSQL = `
CREATE TABLE IF NOT EXISTS mydatabase.events (
    Timestamp DateTime,
    Level     String,
    Source    String,
    Message   String
) ENGINE = MergeTree()
ORDER BY Timestamp;
`

// Connect now returns a standard *sql.DB pool
func Connect(host string) (*sql.DB, error) {
	if host == "" {
		host = "localhost"
	}

	dsn := fmt.Sprintf("clickhouse://%s:%s@%s:9000/%s?dial_timeout=%s",
		os.Getenv("CLICKHOUSE_USER"),
		os.Getenv("CLICKHOUSE_PASSWORD"),
		host,
		os.Getenv("CLICKHOUSE_DB"),
		"10s",
	)

	db, err := sql.Open("clickhouse", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open clickhouse db: %w", err)
	}

	// Configure the pool
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(time.Hour)

	// Ping the database to ensure connectivity
	for i := 0; i < 5; i++ {
		if err = db.Ping(); err == nil {
			return db, nil
		}
		fmt.Printf("Failed to ping clickhouse (attempt %d): %v\n", i+1, err)
		time.Sleep(3 * time.Second)
	}

	return nil, fmt.Errorf("failed to ping clickhouse after retries: %w", err)
}

// InitializeSchema now uses *sql.DB
func InitializeSchema(db *sql.DB) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := db.ExecContext(ctx, initSQL)
	if err != nil {
		return fmt.Errorf("failed to run init sql: %w", err)
	}
	log.Println("Database schema is initialized.")
	return nil
}