package db

import (
	"time"
)

// Config holds database connection configuration.
type Config struct {
	// Connection string (e.g., "postgres://user:pass@host:port/db")
	ConnString string

	// Max number of connections in the pool.
	// Default: 4 * number of CPUs
	MaxConns int32

	// Min number of connections in the pool.
	// Default: 0
	MinConns int32

	// Max time a connection may be reused.
	// Default: 1 hour
	MaxConnLifetime time.Duration

	// Max time a connection may be idle.
	// Default: 30 minutes
	MaxConnIdleTime time.Duration

	// Max time to wait when acquiring a connection from pool.
	// Default: 30 seconds
	HealthCheckPeriod time.Duration
}

// DefaultConfig returns a config with sensible defaults.
// Apply only the non-zero fields from input cfg.
func DefaultConfig(cfg Config) Config {
	def := Config{
		MaxConns:           0, // let pgxpool decide (4 * CPUs)
		MinConns:           0,
		MaxConnLifetime:    time.Hour,
		MaxConnIdleTime:    30 * time.Minute,
		HealthCheckPeriod:  1 * time.Minute,
	}

	if cfg.MaxConns > 0 {
		def.MaxConns = cfg.MaxConns
	}
	if cfg.MinConns > 0 {
		def.MinConns = cfg.MinConns
	}
	if cfg.MaxConnLifetime > 0 {
		def.MaxConnLifetime = cfg.MaxConnLifetime
	}
	if cfg.MaxConnIdleTime > 0 {
		def.MaxConnIdleTime = cfg.MaxConnIdleTime
	}
	if cfg.HealthCheckPeriod > 0 {
		def.HealthCheckPeriod = cfg.HealthCheckPeriod
	}

	return def
}
