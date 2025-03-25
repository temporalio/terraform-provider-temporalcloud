package types

func ListDiff[T comparable](old, new []T) (added, removed []T) {
	oldMap := make(map[T]struct{})
	newMap := make(map[T]struct{})

	// Populate maps for quick lookup
	for _, v := range old {
		oldMap[v] = struct{}{}
	}
	for _, v := range new {
		newMap[v] = struct{}{}
	}

	// Identify added elements
	for v := range newMap {
		if _, exists := oldMap[v]; !exists {
			added = append(added, v)
		}
	}

	// Identify removed elements
	for v := range oldMap {
		if _, exists := newMap[v]; !exists {
			removed = append(removed, v)
		}
	}

	return added, removed
}
