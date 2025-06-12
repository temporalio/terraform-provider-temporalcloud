package types

func MapDiff[K comparable, V comparable](old, planned map[K]V) (added, removed, modified map[K]V) {
	added = make(map[K]V)
	removed = make(map[K]V)
	modified = make(map[K]V)

	// Identify added and modified elements
	for k, v := range planned {
		if oldVal, exists := old[k]; !exists {
			// Key doesn't exist in old map - it's added
			added[k] = v
		} else if oldVal != v {
			// Key exists but value is different - it's modified
			modified[k] = v
		}
	}

	// Identify removed elements
	for k, v := range old {
		if _, exists := planned[k]; !exists {
			removed[k] = v
		}
	}

	return added, removed, modified
}
