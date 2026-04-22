package usage

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/golang-migrate/migrate/v4"
	mclick "github.com/golang-migrate/migrate/v4/database/clickhouse"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

type Store struct {
	conn driver.Conn
}

func NewStore(dsn string) (*Store, error) {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse clickhouse dsn: %w", err)
	}

	conn, err := clickhouse.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("open clickhouse: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := conn.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping clickhouse: %w", err)
	}

	if err := runMigrations(dsn); err != nil {
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	return &Store{conn: conn}, nil
}

func runMigrations(dsn string) error {
	sub, err := fs.Sub(migrationsFS, "migrations")
	if err != nil {
		return err
	}

	src, err := iofs.New(sub, ".")
	if err != nil {
		return err
	}

	db := clickhouse.OpenDB(mustParseOptions(dsn))
	drv, err := mclick.WithInstance(db, &mclick.Config{})
	if err != nil {
		return err
	}

	m, err := migrate.NewWithInstance("iofs", src, "clickhouse", drv)
	if err != nil {
		return err
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func mustParseOptions(dsn string) *clickhouse.Options {
	opts, err := clickhouse.ParseDSN(dsn)
	if err != nil {
		panic(err)
	}
	return opts
}

func (s *Store) Record(ctx context.Context, email, model string, u TokenUsage) error {
	cost := ComputeCost(model, u)
	return s.conn.Exec(ctx,
		"INSERT INTO usage (email, model, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd) VALUES (?, ?, ?, ?, ?, ?, ?)",
		email, model, uint32(u.InputTokens), uint32(u.OutputTokens), uint32(u.CacheCreationTokens), uint32(u.CacheReadTokens), cost,
	)
}

type UserSummary struct {
	Email                string  `json:"email"`
	Model                string  `json:"model"`
	RequestCount         uint64  `json:"request_count"`
	InputTokens          uint64  `json:"input_tokens"`
	OutputTokens         uint64  `json:"output_tokens"`
	CacheCreationTokens  uint64  `json:"cache_creation_tokens"`
	CacheReadTokens      uint64  `json:"cache_read_tokens"`
	TotalCostUSD         float64 `json:"total_cost_usd"`
}

func (s *Store) QuerySummary(ctx context.Context, since, until time.Time) ([]UserSummary, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT email, model, count() AS request_count,
		       sum(input_tokens) AS input_tokens,
		       sum(output_tokens) AS output_tokens,
		       sum(cache_creation_tokens) AS cache_creation_tokens,
		       sum(cache_read_tokens) AS cache_read_tokens,
		       sum(cost_usd) AS total_cost_usd
		FROM usage
		WHERE created_at >= ? AND created_at < ?
		GROUP BY email, model
		ORDER BY total_cost_usd DESC
	`, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UserSummary
	for rows.Next() {
		var s UserSummary
		if err := rows.Scan(&s.Email, &s.Model, &s.RequestCount, &s.InputTokens, &s.OutputTokens, &s.CacheCreationTokens, &s.CacheReadTokens, &s.TotalCostUSD); err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, rows.Err()
}

type UsageRow struct {
	Model               string    `json:"model"`
	InputTokens         uint32    `json:"input_tokens"`
	OutputTokens        uint32    `json:"output_tokens"`
	CacheCreationTokens uint32    `json:"cache_creation_tokens"`
	CacheReadTokens     uint32    `json:"cache_read_tokens"`
	CostUSD             float64   `json:"cost_usd"`
	CreatedAt           time.Time `json:"created_at"`
}

func (s *Store) QueryByEmail(ctx context.Context, email string, since, until time.Time) ([]UsageRow, error) {
	rows, err := s.conn.Query(ctx, `
		SELECT model, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost_usd, created_at
		FROM usage
		WHERE email = ? AND created_at >= ? AND created_at < ?
		ORDER BY created_at DESC
	`, email, since, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []UsageRow
	for rows.Next() {
		var r UsageRow
		if err := rows.Scan(&r.Model, &r.InputTokens, &r.OutputTokens, &r.CacheCreationTokens, &r.CacheReadTokens, &r.CostUSD, &r.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

type SessionRow struct {
	Token     string
	Email     string
	ExpiresAt time.Time
}

func (s *Store) SaveSession(ctx context.Context, token, email string, expiresAt time.Time) error {
	return s.conn.Exec(ctx,
		"INSERT INTO sessions (token, email, expires_at) VALUES (?, ?, ?)",
		token, email, expiresAt,
	)
}

func (s *Store) LoadSessions(ctx context.Context) ([]SessionRow, error) {
	rows, err := s.conn.Query(ctx,
		"SELECT token, email, expires_at FROM sessions WHERE expires_at > now()",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SessionRow
	for rows.Next() {
		var r SessionRow
		if err := rows.Scan(&r.Token, &r.Email, &r.ExpiresAt); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	return s.conn.Exec(ctx, "ALTER TABLE sessions DELETE WHERE token = ?", token)
}

func (s *Store) Close() error {
	return s.conn.Close()
}
