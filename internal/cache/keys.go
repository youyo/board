package cache

// EntityKey is a key that uniquely identifies a cache entry.
// Composite key of profile_name, resource_name, and entity_id.
type EntityKey struct {
	Profile  string
	Resource string
	EntityID string
}

// NewEntityKey creates an EntityKey.
func NewEntityKey(profile, resource, entityID string) EntityKey {
	return EntityKey{
		Profile:  profile,
		Resource: resource,
		EntityID: entityID,
	}
}
