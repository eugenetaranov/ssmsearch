package search

import "strings"

// Filter returns the keys that contain every pattern as a case-insensitive
// substring (AND logic). Empty patterns match everything.
func Filter(keys []string, patterns []string) []string {
	lowered := make([]string, 0, len(patterns))
	for _, p := range patterns {
		if p != "" {
			lowered = append(lowered, strings.ToLower(p))
		}
	}

	var result []string
	for _, key := range keys {
		if Match(key, lowered) {
			result = append(result, key)
		}
	}
	return result
}

// Match reports whether key contains every (already lowercased) pattern.
func Match(key string, lowered []string) bool {
	name := strings.ToLower(key)
	for _, p := range lowered {
		if !strings.Contains(name, p) {
			return false
		}
	}
	return true
}

// ByPrefix returns the keys starting with prefix. A prefix of "/" matches all.
func ByPrefix(keys []string, prefix string) []string {
	if prefix == "" || prefix == "/" {
		return keys
	}
	var result []string
	for _, k := range keys {
		if strings.HasPrefix(k, prefix) {
			result = append(result, k)
		}
	}
	return result
}
