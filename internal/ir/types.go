package ir

// GoType describes the Go type a contract type expression resolves to.
type GoType struct {
	Kind string  // "string", "int64", "float64", "bool", "named", "slice"
	Elem *GoType // set when Kind == "slice"
	Name string  // set when Kind == "named": the Go type name to reference
}

func (t GoType) String() string {
	switch t.Kind {
	case "slice":
		return "[]" + t.Elem.String()
	case "named":
		return t.Name
	default:
		return t.Kind
	}
}

// Source is where a request field's value comes from.
type Source string

const (
	SourcePath   Source = "path"
	SourceQuery  Source = "query"
	SourceHeader Source = "header"
	SourceBody   Source = "json"
	SourceCtx    Source = "ctx"
)

// FieldIR is one field of a generated struct.
type FieldIR struct {
	GoName      string // Go field name, e.g. "NextCursor"
	Type        GoType
	Pointer     bool   // optional scalar fields are pointers; slices never are
	Source      Source // "" for a plain (non-request) struct field
	SourceName  string // original name for path/query/header/json; middleware name for ctx
	CtxKeyConst string // set only when Source == SourceCtx: the context key constant to read
}

// StructIR is one generated struct (named type, or auto-named request/response).
type StructIR struct {
	Name   string
	Fields []FieldIR
}

// MiddlewareIR is one declared middleware.
type MiddlewareIR struct {
	Name        string // contract name, e.g. "auth"
	GoField     string // Middlewares struct field, e.g. "Auth"
	Provides    string // Go type name it puts in context, "" if none
	CtxKeyConst string // generated context key constant name, "" if Provides == ""
}

// RouteIR is one resolved route, ready for codegen.
type RouteIR struct {
	Method       string
	Path         string // contract path, e.g. "/posts/{id}"
	MuxPattern   string // "METHOD /path", ready for http.ServeMux.Handle
	MethodName   string // Service interface method name
	Request      StructIR
	ResponseType string // Go type name used as `data`; "" means 204 No Content
	Middlewares  []MiddlewareIR
}

// ContractIR is the fully resolved contract, ready for codegen.
type ContractIR struct {
	ServiceName string
	Middlewares []MiddlewareIR
	Types       []StructIR // named types plus auto-named inline request/response types
	Routes      []RouteIR
}
