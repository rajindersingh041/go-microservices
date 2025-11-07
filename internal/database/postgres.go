package database

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// The SQL to create our table, embedded in the Go code.
const initSQL = `
CREATE TABLE IF NOT EXISTS events (
    id        SERIAL PRIMARY KEY,
    Timestamp TIMESTAMPTZ,
    Level     VARCHAR(50),
    Source    VARCHAR(100),
    Message   TEXT
);
`

// Connect establishes a pool, pings, and runs init SQL.
func Connect() (*pgxpool.Pool, error) {
	// Build the connection string (DSN)
	host := os.Getenv("POSTGRES_HOST")
	if host == "" {
		host = "localhost"
	}

	// Correct DSN with fixed user variable and sslmode disabled
	dsn := fmt.Sprintf("postgres://%s:%s@%s:5432/%s?sslmode=disable",
		os.Getenv("POSTGRES_USER"), // Fixed typo
		os.Getenv("POSTGRES_PASSWORD"),
		host,
		os.Getenv("POSTGRES_DB"),
	)

	// --- THIS IS THE UPGRADED CONFIG ---
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pgxpool config: %w", err)
	}

	// Set pool settings for high concurrency
	config.MaxConns = 50 // Max connections for high load
	config.MinConns = 5  // Keep some connections warm
	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConnLifetime = 30 * time.Minute
	// Set a timeout for acquiring a connection from the pool
	config.HealthCheckPeriod = 1 * time.Minute
	// We need to set the connect timeout on the underlying config
	config.ConnConfig.ConnectTimeout = 10 * time.Second

	// 2. Try to connect to the pool (with retry)
	var pool *pgxpool.Pool
	for i := 0; i < 5; i++ {
		// Use NewWithConfig instead of New
		pool, err = pgxpool.NewWithConfig(context.Background(), config)
		if err == nil {
			break // Success
		}
		fmt.Printf("Failed to connect to postgres pool (attempt %d): %v\n", i+1, err)
		time.Sleep(3 * time.Second)
	}
	// --- END UPGRADED CONFIG ---

	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres pool after retries: %w", err)
	}

	// 3. Run the init SQL on the pool
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err = pool.Exec(ctx, initSQL)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to run init sql: %w", err)
	}

	return pool, nil
}