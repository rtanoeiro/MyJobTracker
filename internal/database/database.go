package database

import (
	"context"
	"fmt"
	"job-applications/internal/db"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DB struct {
	*db.Queries
	conn *pgxpool.Pool
}

func (d *DB) Close() {
	d.conn.Close()
}

func Connect(ctx context.Context, dbString string) (*DB, error) {
	config, err := pgxpool.ParseConfig(dbString)
	if err != nil {
		return nil, fmt.Errorf("parsing database config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 5 * time.Minute
	config.MaxConnIdleTime = 1 * time.Minute

	conn, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("creating connection pool: %w", err)
	}

	if err := conn.Ping(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	slog.Info("Successfully connected to database")
	return &DB{
		Queries: db.New(conn),
		conn:    conn,
	}, nil
}
