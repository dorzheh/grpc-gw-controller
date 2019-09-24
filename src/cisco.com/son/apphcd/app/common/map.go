// Author  <dorzheho@cisco.com>

package common

// LabelsMap is intended to hold application instance labels
type Map map[string]string

// MakeLabelsMap allocates new map
func MakeMap() Map {
	return make(map[string]string)
}

// AddLabel adds a new label and its value to the map
func (m Map) Add(key, value string) {
	m[key] = value
}

func (m Map) KeyExists(key string) bool {
	if _, ok := m[key]; ok {
		return true
	}
	return false
}

func (m Map) KeyHasValue(key string) bool {
	if v, ok := m[key]; ok && v != "" {
		return true
	}
	return false
}

// GetAnnotation retrieves appropriate annotation value according to its key
func (m Map) Get(key string) string {
	if v, ok := m[key]; ok {
		return v
	}
	return ""
}

func (m Map) Delete(key string) {
	delete(m, key)
}

// Empty checks the map for existing labels
func (m Map) Empty() bool {
	return len(m) == 0
}

func MapAdd(m Map, key, value string) {
	m.Add(key, value)
}

func MapGet(m Map, key string) string {
	return m.Get(key)
}

// MapMerge appends data to the map
func MapMerge(dst Map, src Map, keysToExclude ...string) {
	for _, f := range keysToExclude {
		if src.KeyExists(f) {
			src.Delete(f)
		}
	}

	for k, v := range src {
		dst.Add(k, v)
	}
}

