package ir

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Untitle lowercases the first rune of s. Shared by the Go and TypeScript
// codegen templates to derive a local variable name (Go: "Auth" -> "auth")
// or client method name (TS: "GetProfile" -> "getProfile") from a Go-style
// PascalCase identifier.
func Untitle(s string) string {
	if s == "" {
		return s
	}
	r, size := utf8.DecodeRuneInString(s)
	return string(unicode.ToLower(r)) + s[size:]
}

// toPascalCase turns a snake_case, kebab-case, or plain contract identifier
// into a Go exported name, e.g. "next_cursor" -> "NextCursor", "id" -> "ID",
// "user_id" -> "UserID", "X-Client-Id" -> "XClientID". "id" is
// special-cased to the Go-idiomatic all-caps initialism; other parts are
// just capitalized. Kebab-case matters for header names (PRD §5.2: header
// keys keep their original HTTP form, e.g. "X-Client-Id").
func toPascalCase(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' })
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.EqualFold(p, "id") {
			b.WriteString("ID")
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		b.WriteString(p[1:])
	}
	return b.String()
}

var pathParamRE = regexp.MustCompile(`\{([a-zA-Z0-9_]+)\}`)

// pathParams returns the names of the {param} segments in a contract path,
// in order.
func pathParams(path string) []string {
	matches := pathParamRE.FindAllStringSubmatch(path, -1)
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	return names
}

// defaultMethodName computes the interface method name PRD §5.2 describes:
// method + PascalCase path segments, with path params prefixed "By".
func defaultMethodName(method, path string) string {
	var b strings.Builder
	b.WriteString(toPascalCase(strings.ToLower(method)))
	for _, seg := range strings.Split(strings.Trim(path, "/"), "/") {
		if seg == "" {
			continue
		}
		if strings.HasPrefix(seg, "{") && strings.HasSuffix(seg, "}") {
			b.WriteString("By")
			b.WriteString(toPascalCase(seg[1 : len(seg)-1]))
			continue
		}
		b.WriteString(toPascalCase(seg))
	}
	return b.String()
}

func baseGoType(name string) GoType {
	switch name {
	case "str":
		return GoType{Kind: "string"}
	case "num":
		return GoType{Kind: "float64"}
	case "int":
		return GoType{Kind: "int64"}
	case "bool":
		return GoType{Kind: "bool"}
	default:
		return GoType{Kind: "named", Name: name}
	}
}

// parseTypeExpr parses a raw contract type expression ("str!", "[]Post!",
// "Post") into its GoType and whether it's required.
func parseTypeExpr(expr string) (GoType, bool) {
	required := strings.HasSuffix(expr, "!")
	base := strings.TrimSuffix(expr, "!")
	if strings.HasPrefix(base, "[]") {
		elem := baseGoType(strings.TrimPrefix(base, "[]"))
		return GoType{Kind: "slice", Elem: &elem}, required
	}
	return baseGoType(base), required
}
