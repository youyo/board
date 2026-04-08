package cache

// schemaVersion is the current schema version.
const schemaVersion = 1

// DDL for resource_cache, sync_state, cache_meta tables.
// Conforms to spec §14-16.

const ddlResourceCache = `
CREATE TABLE IF NOT EXISTS resource_cache (
  profile_name TEXT NOT NULL,
  resource_name TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  updated_at TEXT,
  fetched_at TEXT NOT NULL,
  PRIMARY KEY (profile_name, resource_name, entity_id)
);

CREATE INDEX IF NOT EXISTS idx_resource_cache_resource
  ON resource_cache(profile_name, resource_name);

CREATE INDEX IF NOT EXISTS idx_resource_cache_updated
  ON resource_cache(profile_name, resource_name, updated_at);
`

const ddlSyncState = `
CREATE TABLE IF NOT EXISTS sync_state (
  profile_name TEXT NOT NULL,
  resource_name TEXT NOT NULL,
  last_synced_at TEXT,
  cursor_updated_at TEXT,
  last_full_synced_at TEXT,
  last_sync_mode TEXT,
  last_sync_status TEXT,
  last_daily_refresh_date TEXT,
  must_full_resync INTEGER NOT NULL DEFAULT 0,
  expired_at TEXT,
  invalidate_reason TEXT,
  last_error_at TEXT,
  last_error_code TEXT,
  last_error_message TEXT,
  consecutive_failures INTEGER NOT NULL DEFAULT 0,
  refresh_started_at TEXT,
  refresh_owner TEXT,
  cache_version INTEGER NOT NULL DEFAULT 1,
  schema_version INTEGER NOT NULL DEFAULT 1,
  PRIMARY KEY (profile_name, resource_name)
);
`

const ddlCacheMeta = `
CREATE TABLE IF NOT EXISTS cache_meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
`
