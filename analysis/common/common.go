package common

// Ref identifies a source location for a matched entry.
type Ref struct {
	PackageID   string
	PackageName string
	Filename    string
	Line        int
	Column      int
}

// Finding is a single matcher/analyzer result tied to a source position.
type Finding struct {
	Filename string
	Line     int
	Column   int
	Package  string
	Message  string
}

// FindingFromRef builds a Finding from a Ref and message.
func FindingFromRef(ref Ref, msg string) Finding {
	return Finding{
		Filename: ref.Filename,
		Line:     ref.Line,
		Column:   ref.Column,
		Package:  ref.PackageName,
		Message:  msg,
	}
}

// MessageOrDefault returns fallback when message is empty.
func MessageOrDefault(message, fallback string) string {
	if message == "" {
		return fallback
	}
	return message
}
