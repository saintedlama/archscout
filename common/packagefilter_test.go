package common

import "testing"

func TestPackageMatchesAny(t *testing.T) {
	tests := []struct {
		name      string
		packageID string
		patterns  []string
		want      bool
	}{
		{
			name:      "exact package match",
			packageID: "github.com/acme/project/domain",
			patterns:  []string{"github.com/acme/project/domain"},
			want:      true,
		},
		{
			name:      "wildcard matches base package",
			packageID: "github.com/acme/project/domain",
			patterns:  []string{"github.com/acme/project/domain/..."},
			want:      true,
		},
		{
			name:      "wildcard matches sub package",
			packageID: "github.com/acme/project/domain/subpkg",
			patterns:  []string{"github.com/acme/project/domain/..."},
			want:      true,
		},
		{
			name:      "wildcard does not match sibling package",
			packageID: "github.com/acme/project/other",
			patterns:  []string{"github.com/acme/project/domain/..."},
			want:      false,
		},
		{
			name:      "empty patterns never match",
			packageID: "github.com/acme/project/domain",
			patterns:  nil,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PackageMatchesAny(tt.packageID, tt.patterns...)
			if got != tt.want {
				t.Fatalf("PackageMatchesAny(%q, %v) = %t, want %t", tt.packageID, tt.patterns, got, tt.want)
			}
		})
	}
}
