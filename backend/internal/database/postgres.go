package database

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPostgresPool creates and configures a new PostgreSQL connection pool.
func NewPostgresPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = 1 * time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = 1 * time.Minute

	// Fast reachability check to prevent deadlock in pgxpool createIdleResources when offline
	hostPort := net.JoinHostPort(config.ConnConfig.Host, strconv.Itoa(int(config.ConnConfig.Port)))
	d := net.Dialer{Timeout: 500 * time.Millisecond}
	conn, dialErr := d.DialContext(ctx, "tcp", hostPort)
	if dialErr != nil {
		return nil, fmt.Errorf("postgres host %s not reachable: %w", hostPort, dialErr)
	}
	_ = conn.Close()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping postgres database: %w", err)
	}

	return pool, nil
}
