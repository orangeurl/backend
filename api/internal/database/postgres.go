package database

import (
	"database/sql"
	"fmt"
	"os"

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
