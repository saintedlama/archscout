package common

// RefKind describes the kind of source entry a ref points at.
type RefKind string

const (
	RefKindPackage      RefKind = "package"
	RefKindFile         RefKind = "file"
	RefKindType         RefKind = "type"
	RefKindFunction     RefKind = "function"
	RefKindVariable     RefKind = "variable"
	RefKindFunctionCall RefKind = "functioncall"
)

// Ref identifies a source location for a matched entry.
type Ref struct {
	PackageID   string
	PackageName string
	Filename    string
	Line        int
	Column      int
	Kind        RefKind
	Match       string
}
