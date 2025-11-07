package database

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/ClickHouse/clickhouse-go/v2"
)

// Connect is the only function. It's shared by all services.
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
	db.SetMaxIdleConns(5)
	db.SetMaxOpenConns(50)
	db.SetConnMaxLifetime(time.Hour)
	for i := 0; i < 5; i++ {
		if err = db.Ping(); err == nil {
			return db, nil
		}
		fmt.Printf("Failed to ping clickhouse (attempt %d): %v\n", i+1, err)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("failed to ping clickhouse after retries: %w", err)
}