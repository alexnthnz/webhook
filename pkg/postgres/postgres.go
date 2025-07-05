package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
)

// Config holds PostgreSQL connection configuration
type Config struct {
	Host     string
	Port     int
	Database string
	Username string
	Password string
	SSLMode  string
	MaxConns int32
	MinConns int32
	MaxLife  time.Duration
	MaxIdle  time.Duration
}

// Client wraps pgxpool.Pool with additional functionality
type Client struct {
	pool *pgxpool.Pool
	log  *logrus.Logger
}

// NewClient creates a new PostgreSQL client
func NewClient(cfg Config, logger *logrus.Logger) (*Client, error) {
	dsn := fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.Username,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.Database,
		cfg.SSLMode,
	)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Configure connection pool
	config.MaxConns = cfg.MaxConns
	config.MinConns = cfg.MinConns
	config.MaxConnLifetime = cfg.MaxLife
	config.MaxConnIdleTime = cfg.MaxIdle

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Successfully connected to PostgreSQL")

	return &Client{
		pool: pool,
		log:  logger,
	}, nil
}

// Close closes the connection pool
func (c *Client) Close() {
	c.pool.Close()
	c.log.Info("PostgreSQL connection pool closed")
}

// Pool returns the underlying connection pool
func (c *Client) Pool() *pgxpool.Pool {
	return c.pool
}

// Ping checks if the database is reachable
func (c *Client) Ping(ctx context.Context) error {
	return c.pool.Ping(ctx)
}

// BeginTx starts a new transaction
func (c *Client) BeginTx(ctx context.Context) (pgx.Tx, error) {
	return c.pool.Begin(ctx)
}

// WithTx executes a function within a transaction
func (c *Client) WithTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
		}
	}()

	err = fn(tx)
	return err
}

// HealthCheck performs a health check on the database
func (c *Client) HealthCheck(ctx context.Context) error {
	var result int
	err := c.pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	return nil
}

// GetStats returns connection pool statistics
func (c *Client) GetStats() *pgxpool.Stat {
	return c.pool.Stat()
}

// DefaultConfig returns a default PostgreSQL configuration
func DefaultConfig() Config {
	return Config{
		Host:     "localhost",
		Port:     5432,
		Database: "webhook_db",
		Username: "postgres",
		Password: "postgres123",
		SSLMode:  "disable",
		MaxConns: 30,
		MinConns: 5,
		MaxLife:  time.Hour,
		MaxIdle:  time.Minute * 30,
	}
}
