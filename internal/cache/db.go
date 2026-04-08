package cache

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// DB は SQLite データベース接続を管理する。
type DB struct {
	db *sql.DB
}

// Open は SQLite データベースを開き、PRAGMA を設定する。
// dsn が ":memory:" の場合はインメモリ DB を使用する。
func Open(dsn string) (*DB, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("cache: open: %w", err)
	}
	// PRAGMA 設定
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("cache: pragma: %w", err)
		}
	}
	return &DB{db: db}, nil
}

// Close はデータベース接続を閉じる。
func (d *DB) Close() error {
	return d.db.Close()
}

// SQLDB は内部の *sql.DB を返す。
// 上位パッケージ（repository 等）がクエリを実行する際に使用。
func (d *DB) SQLDB() *sql.DB {
	return d.db
}
