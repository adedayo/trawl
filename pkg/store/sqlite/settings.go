package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

func (s *SQLiteStore) GetSetting(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, "SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil // Return empty string if not found, not an error
		}
		return "", err
	}
	return value, nil
}

func (s *SQLiteStore) SaveSetting(ctx context.Context, key string, value string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO settings (key, value, updated_at) 
		VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET 
			value = excluded.value, 
			updated_at = excluded.updated_at
	`, key, value, time.Now())
	return err
}
