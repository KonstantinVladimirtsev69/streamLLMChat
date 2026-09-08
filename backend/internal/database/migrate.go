package database

import (
	"errors"
	"fmt"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"backend/migrations"
)

// normalizeURL converts postgres:// URL to pgx5:// required by golang-migrate pgx/v5 driver.
func normalizeURL(databaseURL string) string {
	if strings.HasPrefix(databaseURL, "postgres://") {
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")
	}
	if strings.HasPrefix(databaseURL, "postgresql://") {
		return "pgx5://" + strings.TrimPrefix(databaseURL, "postgresql://")
	}
	return databaseURL
}

// Up applies all pending database migrations against databaseURL.
func Up(databaseURL string) error {
	driver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	targetURL := normalizeURL(databaseURL)
	m, err := migrate.NewWithSourceInstance("iofs", driver, targetURL)
	if err != nil {
		return fmt.Errorf("failed to init migrate instance: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	return nil
}

// Down rolls back the most recent database migration.
func Down(databaseURL string) error {
	driver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to create iofs driver: %w", err)
	}

	targetURL := normalizeURL(databaseURL)
	m, err := migrate.NewWithSourceInstance("iofs", driver, targetURL)
	if err != nil {
		return fmt.Errorf("failed to init migrate instance: %w", err)
	}
	defer func() {
		_, _ = m.Close()
	}()

	if err := m.Steps(-1); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to roll back migration: %w", err)
	}

	return nil
}
