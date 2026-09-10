package ir

// GoTypeString is the Go type string for a field: the bare type, prefixed
// with "*" when the field is optional (Pointer).
func GoTypeString(f FieldIR) string {
	s := f.Type.String()
	if f.Pointer {
		s = "*" + s
	}
	return s
}

// StructTag is the struct tag for a field, chosen by its Source: exactly
// one of path/query/header/json is set, and every non-json source also
// carries json:"-" so encoding/json's body decode (into the same struct)
// ignores it.
func StructTag(f FieldIR) string {
	switch f.Source {
	case SourcePath:
		return `path:"` + f.SourceName + `" json:"-"`
	case SourceQuery:
		return `query:"` + f.SourceName + `" json:"-"`
	case SourceHeader:
		return `header:"` + f.SourceName + `" json:"-"`
	case SourceCtx:
		return `json:"-"`
	default: // SourceBody, or "" for a plain domain-type field
		return `json:"` + f.SourceName + `"`
	}
}
