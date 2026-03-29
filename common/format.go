package common

import (
	"fmt"
	"strconv"
	"strings"
)

// Refs is a convenience slice wrapper for matched refs.
type Refs []Ref

// Format renders all refs using the provided options.
func (refs Refs) Format(opts ...RefFormatOption) string {
	return FormatRefs(refs, opts...)
}

// Join joins all refs formatted with default options using separator.
func (refs Refs) Join(separator string) string {
	if len(refs) == 0 {
		return ""
	}

	formatted := make([]string, 0, len(refs))
	for _, ref := range refs {
		formatted = append(formatted, ref.String())
	}

	return strings.Join(formatted, separator)
}

// String renders all refs with default formatting.
func (refs Refs) String() string {
	return refs.Format()
}

// RefFormatOptions controls human-readable ref formatting.
type RefFormatOptions struct {
	IncludeFile    bool
	IncludeLine    bool
	IncludeColumn  bool
	IncludePackage bool
	IncludeMatch   bool
	IncludeKind    bool
	EntrySeparator string
}

// RefFormatOption mutates ref formatting behavior.
type RefFormatOption func(*RefFormatOptions)

// DefaultRefFormatOptions returns the default ref formatting configuration.
func DefaultRefFormatOptions() RefFormatOptions {
	return RefFormatOptions{
		IncludeFile:    true,
		IncludeLine:    true,
		IncludeColumn:  true,
		IncludeMatch:   true,
		EntrySeparator: ", ",
	}
}

// WithRefPackage includes package information in formatted refs.
func WithRefPackage() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludePackage = true
	}
}

// WithRefKind includes the ref kind in formatted refs.
func WithRefKind() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludeKind = true
	}
}

// WithoutRefFile omits the filename from formatted refs.
func WithoutRefFile() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludeFile = false
	}
}

// WithoutRefLine omits the line number from formatted refs.
func WithoutRefLine() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludeLine = false
	}
}

// WithoutRefColumn omits the column number from formatted refs.
func WithoutRefColumn() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludeColumn = false
	}
}

// WithoutRefMatch omits the matched-node representation from formatted refs.
func WithoutRefMatch() RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.IncludeMatch = false
	}
}

// WithRefSeparator configures the separator used by FormatRefs.
func WithRefSeparator(separator string) RefFormatOption {
	return func(opts *RefFormatOptions) {
		opts.EntrySeparator = separator
	}
}

// Format renders a single ref using the provided options.
func (ref Ref) Format(opts ...RefFormatOption) string {
	return FormatRef(ref, opts...)
}

// String renders a single ref with default formatting.
func (ref Ref) String() string {
	return FormatRef(ref)
}

// FormatRef renders a single ref using the provided options.
func FormatRef(ref Ref, opts ...RefFormatOption) string {
	config := DefaultRefFormatOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	var parts []string
	if location := formatLocation(ref, config); location != "" {
		parts = append(parts, location)
	}
	if config.IncludePackage && ref.PackageID != "" {
		parts = append(parts, fmt.Sprintf("package %s", ref.PackageID))
	}
	if config.IncludeMatch && ref.Match != "" {
		parts = append(parts, ref.Match)
	} else if config.IncludeKind && ref.Kind != "" {
		parts = append(parts, string(ref.Kind))
	}

	if len(parts) == 0 {
		if ref.Kind != "" {
			return string(ref.Kind)
		}
		return "ref"
	}

	return strings.Join(parts, " ")
}

// FormatRefs renders a slice of refs using the provided options.
func FormatRefs(refs Refs, opts ...RefFormatOption) string {
	if len(refs) == 0 {
		return ""
	}

	config := DefaultRefFormatOptions()
	for _, opt := range opts {
		if opt != nil {
			opt(&config)
		}
	}

	formatted := make([]string, 0, len(refs))
	for _, ref := range refs {
		formatted = append(formatted, FormatRef(ref, opts...))
	}

	return strings.Join(formatted, config.EntrySeparator)
}

func formatLocation(ref Ref, opts RefFormatOptions) string {
	location := ""
	if opts.IncludeFile && ref.Filename != "" {
		location = ref.Filename
	}

	if opts.IncludeLine && ref.Line > 0 {
		if location != "" {
			location += ":" + strconv.Itoa(ref.Line)
		} else {
			location = "line=" + strconv.Itoa(ref.Line)
		}
	}

	if opts.IncludeColumn && ref.Column > 0 {
		switch {
		case location != "" && (opts.IncludeFile || opts.IncludeLine):
			location += ":" + strconv.Itoa(ref.Column)
		case location != "":
			location += " column=" + strconv.Itoa(ref.Column)
		default:
			location = "column=" + strconv.Itoa(ref.Column)
		}
	}

	return location
}
