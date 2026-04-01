package common

import "strings"

// IsTestFilename reports whether filename points to a Go test file.
func IsTestFilename(filename string) bool {
	normalized := strings.ReplaceAll(filename, "\\", "/")
	return strings.HasSuffix(normalized, "_test.go")
}
