package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// DB is a global variable that holds the connection pool to the PostgreSQL database.
var DB *sql.DB

func InitPostgres() error {
	var err error
	DB, err = sql.Open("postgres", os.Getenv("POSTGRES_URL"))
	if err != nil {
		return fmt.Errorf("failed to connect to the database: %w", err)
	}

	// Configure connection pool settings for production
	DB.SetMaxOpenConns(25)                 // Maximum number of open connections
	DB.SetMaxIdleConns(5)                  // Maximum number of idle connections
	DB.SetConnMaxLifetime(5 * time.Minute) // Maximum lifetime of a connection
	DB.SetConnMaxIdleTime(2 * time.Minute) // Maximum idle time before connection is closed

	if err := DB.Ping(); err != nil {
		return fmt.Errorf("failed to ping the database: %w", err)
	}

	return nil
}

func ClosePostgres() error {
	if DB != nil {
		return DB.Close()
	}
	return nil
}

func GetQueries() *Queries {
	return New(DB)
}

// QueryTimeout is the default timeout for database queries
const QueryTimeout = 10 * time.Second

// GetQueryContext returns a context with a timeout for database queries
// This prevents long-running queries from blocking the application
func GetQueryContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), QueryTimeout)
}
