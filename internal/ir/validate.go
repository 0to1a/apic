package ir

import (
	"fmt"
	"strings"
	"time"

	"github.com/0to1a/apic/internal/contract"
)

// ValidationError is one problem found in the contract. Error() always
// names the location and, when there's an actionable one, a suggested fix
// (PRD §5.6).
type ValidationError struct {
	Line       int
	Message    string
	Suggestion string
}

func (e *ValidationError) Error() string {
	if e.Suggestion == "" {
		return fmt.Sprintf("line %d: %s", e.Line, e.Message)
	}
	return fmt.Sprintf("line %d: %s — %s", e.Line, e.Message, e.Suggestion)
}

func isBaseType(s string) bool {
	switch s {
	case "str", "num", "int", "bool":
		return true
	}
	return false
}

// baseTypeName strips the "!" required suffix and "[]" slice prefix off a
// raw type expression, leaving the bare type name to look up.
func baseTypeName(expr string) string {
	return strings.TrimPrefix(strings.TrimSuffix(expr, "!"), "[]")
}

// Validate checks every §5.6 rule against the raw parsed contract and
// returns every violation found — it never stops at the first one, so a
// single run gives complete feedback (important for the LLM workflow in
// PRD §9). Resolve assumes its input already passed Validate.
func Validate(c *contract.Contract) []error {
	var errs []error

	typesByName := map[string]contract.TypeDef{}
	for _, t := range c.Types {
		typesByName[t.Name] = t
	}
	mwByName := map[string]contract.Middleware{}
	for _, m := range c.Middlewares {
		mwByName[m.Name] = m
	}

	checkTypeRef := func(where string, line int, expr string) {
		base := baseTypeName(expr)
		if isBaseType(base) {
			return
		}
		if _, ok := typesByName[base]; !ok {
			errs = append(errs, &ValidationError{Line: line, Message: fmt.Sprintf("%s: unknown type %q", where, base), Suggestion: "define it under types:"})
		}
	}
	checkFields := func(where string, blockName string, fields contract.Fields, pathParamSet map[string]bool) {
		for _, f := range fields {
			if f.Extends != "" {
				if _, ok := typesByName[f.Extends]; !ok {
					errs = append(errs, &ValidationError{Line: f.Loc.Line, Message: fmt.Sprintf("%s: unknown type %q", where, f.Extends), Suggestion: "define it under types:"})
				}
				continue
			}
			checkTypeRef(where, f.Loc.Line, f.Type)
			if pathParamSet != nil && pathParamSet[f.Name] && (blockName == "query" || blockName == "header") {
				errs = append(errs, &ValidationError{
					Line:       f.Loc.Line,
					Message:    fmt.Sprintf("%s: %s field %q collides with path parameter {%s}", where, blockName, f.Name, f.Name),
					Suggestion: "rename the field or the path parameter",
				})
			}
		}
	}

	// Types block: every extends/field type must resolve.
	for _, t := range c.Types {
		checkFields(fmt.Sprintf("type %s", t.Name), "", t.Fields, nil)
	}

	seenPaths := map[string]int{}          // "METHOD /full/path" -> line of first occurrence
	seenMethodNames := map[string]string{} // method name -> "METHOD /path" of first occurrence

	for gi, g := range c.Groups {
		if !g.PrefixSet {
			errs = append(errs, &ValidationError{Line: g.Loc.Line, Message: fmt.Sprintf("group #%d: missing \"prefix\"", gi+1), Suggestion: "add \"prefix: /\" (or another base path)"})
		}
		if !g.UseSet {
			errs = append(errs, &ValidationError{Line: g.Loc.Line, Message: fmt.Sprintf("group #%d: missing \"use\"", gi+1), Suggestion: "add \"use: []\" if no middleware"})
		}
		for _, name := range g.Use {
			if _, ok := mwByName[name]; !ok {
				errs = append(errs, &ValidationError{Line: g.Loc.Line, Message: fmt.Sprintf("group #%d: unknown middleware %q", gi+1, name), Suggestion: "declare it under middlewares:"})
			}
		}

		for _, r := range g.Routes {
			fullPath := joinPath(g.Prefix, r.Path)
			key := r.Method + " " + fullPath
			where := fmt.Sprintf("route %s", key)

			if firstLine, dup := seenPaths[key]; dup {
				errs = append(errs, &ValidationError{Line: r.Loc.Line, Message: fmt.Sprintf("duplicate route %q (first defined at line %d)", key, firstLine), Suggestion: "remove or change one of the two routes"})
			} else {
				seenPaths[key] = r.Loc.Line
			}

			methodName := r.Name
			if methodName == "" {
				methodName = defaultMethodName(r.Method, fullPath)
			}
			if first, dup := seenMethodNames[methodName]; dup && first != key {
				errs = append(errs, &ValidationError{
					Line:       r.Loc.Line,
					Message:    fmt.Sprintf("method name %q already used by %s", methodName, first),
					Suggestion: fmt.Sprintf("add name: <Something> to %s to disambiguate", key),
				})
			} else {
				seenMethodNames[methodName] = key
			}

			for _, name := range r.Skip {
				if !containsStr(g.Use, name) {
					errs = append(errs, &ValidationError{Line: r.Loc.Line, Message: fmt.Sprintf("%s: skip %q not used by this group's \"use\"", where, name), Suggestion: "remove it from skip, or add it to the group's use"})
				}
			}
			for _, name := range r.Use {
				if _, ok := mwByName[name]; !ok {
					errs = append(errs, &ValidationError{Line: r.Loc.Line, Message: fmt.Sprintf("%s: unknown middleware %q", where, name), Suggestion: "declare it under middlewares:"})
				}
			}

			pathParamSet := map[string]bool{}
			for _, p := range pathParams(fullPath) {
				pathParamSet[p] = true
			}
			checkFields(where, "body", r.Body, pathParamSet)
			checkFields(where, "query", r.Query, pathParamSet)
			checkFields(where, "header", r.Header, pathParamSet)
			if r.Resp.Ref != "" {
				checkTypeRef(where, r.Resp.Loc.Line, r.Resp.Ref)
			} else {
				checkFields(where, "resp", r.Resp.Fields, nil)
			}
		}
	}

	// Crons are checked after every route, so a route always stays the first
	// owner of a method name and the collision message reads the same way.
	seenCronNames := map[string]bool{}
	for _, cr := range c.Crons {
		where := fmt.Sprintf("cron %q", cr.Name)
		if cr.Name == "" {
			errs = append(errs, &ValidationError{Line: cr.Loc.Line, Message: "cron name must not be empty"})
			continue
		}
		if seenCronNames[cr.Name] {
			errs = append(errs, &ValidationError{Line: cr.Loc.Line, Message: fmt.Sprintf("duplicate cron %q", cr.Name), Suggestion: "give each cron job a unique name"})
		}
		seenCronNames[cr.Name] = true

		switch d, err := time.ParseDuration(cr.Every); {
		case cr.Every == "":
			errs = append(errs, &ValidationError{Line: cr.Loc.Line, Message: fmt.Sprintf("%s: \"every\" is required", where), Suggestion: "add every: 1h (a Go duration: 30s, 5m, 1h, 1h30m)"})
		case err != nil:
			errs = append(errs, &ValidationError{Line: cr.Loc.Line, Message: fmt.Sprintf("%s: invalid every %q", where, cr.Every), Suggestion: "use a Go duration like 30s, 5m, 1h"})
		case d <= 0:
			errs = append(errs, &ValidationError{Line: cr.Loc.Line, Message: fmt.Sprintf("%s: every %q must be greater than zero", where, cr.Every)})
		}

		methodName := cronMethodName(cr)
		if first, dup := seenMethodNames[methodName]; dup {
			errs = append(errs, &ValidationError{
				Line:       cr.Loc.Line,
				Message:    fmt.Sprintf("method name %q already used by %s", methodName, first),
				Suggestion: fmt.Sprintf("add name: <Something> to %s to disambiguate", where),
			})
		} else {
			seenMethodNames[methodName] = where
		}
	}

	return errs
}
