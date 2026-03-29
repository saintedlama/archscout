package common_test

import (
	"testing"

	"github.com/saintedlama/goarch/common"
	"github.com/stretchr/testify/assert"
)

func TestFormatRefs_DefaultFormattingUsesLocationAndMatch(t *testing.T) {
	formatted := common.Refs{{
		Filename: "fixture.go",
		Line:     12,
		Column:   7,
		Kind:     common.RefKindFunction,
		Match:    "func RootErr",
	}}.Format()

	assert.Equal(t, "fixture.go:12:7 func RootErr", formatted)
}

func TestFormatRefs_OptionsCanCustomizeOutput(t *testing.T) {
	formatted := common.Refs{
		{
			PackageID: "github.com/saintedlama/goarch/testdata/fixturemod",
			Filename:  "main.go",
			Line:      19,
			Column:    9,
			Kind:      common.RefKindFunctionCall,
			Match:     `fmt.Errorf("root error")`,
		},
		{
			PackageID: "github.com/saintedlama/goarch/testdata/fixturemod/subpkg",
			Filename:  "sub.go",
			Line:      6,
			Column:    9,
			Kind:      common.RefKindFunctionCall,
			Match:     `fmt.Errorf("sub error")`,
		},
	}.Format(common.WithRefPackage(), common.WithRefSeparator(" | "), common.WithoutRefColumn())

	assert.Equal(
		t,
		`main.go:19 package github.com/saintedlama/goarch/testdata/fixturemod fmt.Errorf("root error") | sub.go:6 package github.com/saintedlama/goarch/testdata/fixturemod/subpkg fmt.Errorf("sub error")`,
		formatted,
	)
}

func TestRefs_JoinUsesDefaultRefFormatting(t *testing.T) {
	joined := common.Refs{
		{Filename: "a.go", Line: 2, Column: 1, Match: "func A"},
		{Filename: "b.go", Line: 3, Column: 5, Match: "func B"},
	}.Join("\n")

	assert.Equal(t, "a.go:2:1 func A\nb.go:3:5 func B", joined)
}

func TestRefs_EmptyFormatAndJoinReturnEmptyString(t *testing.T) {
	var refs common.Refs

	assert.Equal(t, "", refs.Format())
	assert.Equal(t, "", refs.Join(" | "))
	assert.Equal(t, "", refs.String())
}

func TestFormatRefs_DefaultSeparatorIsNewline(t *testing.T) {
	formatted := common.Refs{
		{Filename: "a.go", Line: 1, Column: 1, Match: "func A"},
		{Filename: "b.go", Line: 2, Column: 3, Match: "func B"},
	}.Format()

	assert.Equal(t, "* a.go:1:1 func A\n* b.go:2:3 func B", formatted)
}

func TestFormatRefs_WithoutSeparatorConcatenatesOutput(t *testing.T) {
	formatted := common.Refs{
		{Filename: "a.go", Line: 1, Column: 1, Match: "func A"},
		{Filename: "b.go", Line: 2, Column: 3, Match: "func B"},
	}.Format(common.WithoutSeparator())

	assert.Equal(t, "a.go:1:1 func Ab.go:2:3 func B", formatted)
}

func TestFormatRef_UsesKindWhenMatchIsOmitted(t *testing.T) {
	formatted := common.FormatRef(
		common.Ref{Kind: common.RefKindVariable},
		common.WithoutRefMatch(),
		common.WithRefKind(),
	)

	assert.Equal(t, "variable", formatted)
}

func TestFormatRef_WithoutFileUsesLineAndColumnLabels(t *testing.T) {
	formatted := common.FormatRef(
		common.Ref{Line: 11, Column: 4, Match: "func RootErr"},
		common.WithoutRefFile(),
	)

	assert.Equal(t, "line=11:4 func RootErr", formatted)
}

func TestRefs_StringMatchesDefaultFormat(t *testing.T) {
	refs := common.Refs{
		{Filename: "a.go", Line: 1, Column: 1, Match: "func A"},
		{Filename: "b.go", Line: 2, Column: 3, Match: "func B"},
	}

	assert.Equal(t, refs.Format(), refs.String())
}

func TestFormatRefs_SingleRefDoesNotAddListMarker(t *testing.T) {
	formatted := common.Refs{{
		Filename: "a.go",
		Line:     1,
		Column:   1,
		Match:    "func A",
	}}.Format()

	assert.Equal(t, "a.go:1:1 func A", formatted)
}
