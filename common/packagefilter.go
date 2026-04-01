package common

import "strings"

// PackageMatchesAny reports whether packageID matches any provided package patterns.
// A pattern ending in "/..." matches the base package and all of its sub-packages.
func PackageMatchesAny(packageID string, patterns ...string) bool {
	if len(patterns) == 0 {
		return false
	}

	for _, pattern := range patterns {
		if packageMatches(packageID, pattern) {
			return true
		}
	}

	return false
}

func packageMatches(packageID string, pattern string) bool {
	if packageID == "" || pattern == "" {
		return false
	}

	if before, ok := strings.CutSuffix(pattern, "/..."); ok {
		base := before
		if base == "" {
			return false
		}

		return packageID == base || strings.HasPrefix(packageID, base+"/")
	}

	return packageID == pattern
}
