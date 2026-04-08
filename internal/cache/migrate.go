package cache

import (
	"fmt"
	"strconv"
)

// Migrate はスキーマバージョンを確認し、必要に応じて DDL を適用する。
// 冪等に設計: 複数回呼んでも安全。
func Migrate(db *DB) error {
	// まず cache_meta テーブルを作成（これがないとバージョン確認できない）
	if _, err := db.db.Exec(ddlCacheMeta); err != nil {
		return fmt.Errorf("cache: migrate: create cache_meta: %w", err)
	}

	// 現在のバージョンを取得
	current := 0
	var val string
	err := db.db.QueryRow("SELECT value FROM cache_meta WHERE key = 'db_schema_version'").Scan(&val)
	if err == nil {
		current, _ = strconv.Atoi(val)
	}

	if current >= schemaVersion {
		return nil // 既に最新
	}

	// マイグレーション適用
	migrations := []struct {
		version int
		ddl     string
	}{
		{1, ddlResourceCache + ddlSyncState},
	}

	for _, m := range migrations {
		if m.version <= current {
			continue
		}
		if _, err := db.db.Exec(m.ddl); err != nil {
			return fmt.Errorf("cache: migrate v%d: %w", m.version, err)
		}
	}

	// バージョン更新
	_, err = db.db.Exec(
		"INSERT OR REPLACE INTO cache_meta (key, value) VALUES ('db_schema_version', ?)",
		strconv.Itoa(schemaVersion),
	)
	if err != nil {
		return fmt.Errorf("cache: migrate: update version: %w", err)
	}

	return nil
}
