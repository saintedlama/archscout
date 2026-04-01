package common

import "testing"

func TestIsTestFilename(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{
			name:     "unix test file",
			filename: "pkg/foo_test.go",
			want:     true,
		},
		{
			name:     "windows test file",
			filename: "pkg\\foo_test.go",
			want:     true,
		},
		{
			name:     "non test file",
			filename: "pkg/foo.go",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsTestFilename(tt.filename)
			if got != tt.want {
				t.Fatalf("IsTestFilename(%q) = %t, want %t", tt.filename, got, tt.want)
			}
		})
	}
}
