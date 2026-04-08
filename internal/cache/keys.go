package cache

// EntityKey はキャッシュエントリを一意に識別するキー。
// profile_name, resource_name, entity_id の複合キー。
type EntityKey struct {
	Profile  string
	Resource string
	EntityID string
}

// NewEntityKey は EntityKey を生成する。
func NewEntityKey(profile, resource, entityID string) EntityKey {
	return EntityKey{
		Profile:  profile,
		Resource: resource,
		EntityID: entityID,
	}
}
