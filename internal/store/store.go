// Package store persists authorization lists and key/value settings in SQL.
// It supports both SQLite (modernc.org/sqlite, pure Go) and PostgreSQL (pgx).
package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib" // registers "pgx"
	_ "modernc.org/sqlite"             // registers "sqlite"
)

// Store wraps a *sql.DB and adapts SQL placeholders per driver.
type Store struct {
	db       *sql.DB
	postgres bool
}

// Open opens a database. driver is "sqlite" or "postgres".
func Open(driver, dsn string) (*Store, error) {
	var (
		sqlDriver string
		postgres  bool
	)
	switch strings.ToLower(driver) {
	case "sqlite", "sqlite3":
		sqlDriver = "sqlite"
		if err := ensureSQLiteDir(dsn); err != nil {
			return nil, err
		}
	case "postgres", "postgresql", "pgx":
		sqlDriver = "pgx"
		postgres = true
	default:
		return nil, fmt.Errorf("unsupported db driver %q", driver)
	}

	db, err := sql.Open(sqlDriver, dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}
	// SQLite is a single file; serialize writers to avoid "database is locked".
	if !postgres {
		db.SetMaxOpenConns(1)
	}
	return &Store{db: db, postgres: postgres}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// Migrate creates the schema if absent.
func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS authorized_users (user_id BIGINT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS authorized_chats (chat_id BIGINT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS settings (skey TEXT PRIMARY KEY, svalue TEXT NOT NULL)`,
	}
	for _, q := range stmts {
		if _, err := s.db.ExecContext(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
	}
	return nil
}

// rebind converts "?" placeholders to "$1, $2, ..." for Postgres.
func (s *Store) rebind(query string) string {
	if !s.postgres {
		return query
	}
	var b strings.Builder
	n := 0
	for _, r := range query {
		if r == '?' {
			n++
			b.WriteString(fmt.Sprintf("$%d", n))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// --- Authorized users ---

// AddAuthorizedUser authorizes a user id (idempotent).
func (s *Store) AddAuthorizedUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`INSERT INTO authorized_users (user_id) VALUES (?) ON CONFLICT DO NOTHING`), userID)
	return err
}

// RemoveAuthorizedUser de-authorizes a user id.
func (s *Store) RemoveAuthorizedUser(ctx context.Context, userID int64) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`DELETE FROM authorized_users WHERE user_id = ?`), userID)
	return err
}

// IsUserAuthorized reports whether a user id is authorized.
func (s *Store) IsUserAuthorized(ctx context.Context, userID int64) (bool, error) {
	return s.exists(ctx, `SELECT 1 FROM authorized_users WHERE user_id = ?`, userID)
}

// ListAuthorizedUsers returns all authorized user ids.
func (s *Store) ListAuthorizedUsers(ctx context.Context) ([]int64, error) {
	return s.listIDs(ctx, `SELECT user_id FROM authorized_users`)
}

// --- Authorized chats ---

// AddAuthorizedChat authorizes a chat id (idempotent).
func (s *Store) AddAuthorizedChat(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`INSERT INTO authorized_chats (chat_id) VALUES (?) ON CONFLICT DO NOTHING`), chatID)
	return err
}

// RemoveAuthorizedChat de-authorizes a chat id.
func (s *Store) RemoveAuthorizedChat(ctx context.Context, chatID int64) error {
	_, err := s.db.ExecContext(ctx,
		s.rebind(`DELETE FROM authorized_chats WHERE chat_id = ?`), chatID)
	return err
}

// IsChatAuthorized reports whether a chat id is authorized.
func (s *Store) IsChatAuthorized(ctx context.Context, chatID int64) (bool, error) {
	return s.exists(ctx, `SELECT 1 FROM authorized_chats WHERE chat_id = ?`, chatID)
}

// ListAuthorizedChats returns all authorized chat ids.
func (s *Store) ListAuthorizedChats(ctx context.Context) ([]int64, error) {
	return s.listIDs(ctx, `SELECT chat_id FROM authorized_chats`)
}

// --- Settings ---

// GetSetting returns the value for key, or ("", nil) if absent.
func (s *Store) GetSetting(ctx context.Context, key string) (string, error) {
	var v string
	err := s.db.QueryRowContext(ctx, s.rebind(`SELECT svalue FROM settings WHERE skey = ?`), key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// SetSetting upserts a key/value setting.
func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, s.rebind(
		`INSERT INTO settings (skey, svalue) VALUES (?, ?) ON CONFLICT (skey) DO UPDATE SET svalue = excluded.svalue`),
		key, value)
	return err
}

// --- helpers ---

func (s *Store) exists(ctx context.Context, query string, arg int64) (bool, error) {
	var one int
	err := s.db.QueryRowContext(ctx, s.rebind(query), arg).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) listIDs(ctx context.Context, query string) ([]int64, error) {
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ensureSQLiteDir creates the directory for a file: DSN so Open does not fail
// on a missing parent directory.
func ensureSQLiteDir(dsn string) error {
	path := dsn
	path = strings.TrimPrefix(path, "file:")
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}
	if path == "" || path == ":memory:" || strings.Contains(dsn, "mode=memory") {
		return nil
	}
	dir := filepath.Dir(path)
	if dir == "" || dir == "." {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}
