package repository

// applyLimit truncates entities to limit if limit > 0.
// Returns entities unchanged if limit <= 0 or len(entities) <= limit.
func applyLimit[T any](entities []T, limit int) []T {
	if limit > 0 && len(entities) > limit {
		return entities[:limit]
	}
	return entities
}
