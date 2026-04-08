package repository

// ReadOptions controls the behavior of repository read operations.
type ReadOptions struct {
	// Refresh, if true, triggers a delta refresh (DeltaRefresh).
	Refresh bool
	// ForceRefresh, if true, triggers a full refresh (ForceRefresh).
	// Takes precedence over Refresh.
	ForceRefresh bool
	// Limit is the maximum number of entries to return. 0 means unlimited.
	Limit int
}
