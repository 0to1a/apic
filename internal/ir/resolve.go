package ir

import (
	"fmt"
	"strings"
	"time"

	"github.com/0to1a/apic/internal/contract"
)

func joinPath(prefix, path string) string {
	return strings.TrimSuffix(prefix, "/") + path
}

func resolveMiddlewares(defs []contract.Middleware) []MiddlewareIR {
	out := make([]MiddlewareIR, 0, len(defs))
	for _, d := range defs {
		m := MiddlewareIR{Name: d.Name, GoField: toPascalCase(d.Name), Provides: d.Provides}
		if d.Provides != "" {
			m.CtxKeyConst = "ctxKey" + m.GoField
		}
		out = append(out, m)
	}
	return out
}

// resolveFields flattens a contract fields block (following `extends`) into
// IR fields tagged with source. visiting tracks in-progress `extends` names
// to reject cycles.
func resolveFields(fields contract.Fields, types map[string]contract.TypeDef, source Source, visiting map[string]bool) ([]FieldIR, error) {
	var out []FieldIR
	for _, f := range fields {
		if f.Extends != "" {
			if visiting[f.Extends] {
				return nil, fmt.Errorf("line %d: type %q extends itself", f.Loc.Line, f.Extends)
			}
			td, ok := types[f.Extends]
			if !ok {
				return nil, fmt.Errorf("line %d: unknown type %q — define it under types:", f.Loc.Line, f.Extends)
			}
			visiting[f.Extends] = true
			nested, err := resolveFields(td.Fields, types, source, visiting)
			delete(visiting, f.Extends)
			if err != nil {
				return nil, err
			}
			out = append(out, nested...)
			continue
		}
		goType, required := parseTypeExpr(f.Type)
		if err := checkTypeRefs(goType, f.Loc.Line, types); err != nil {
			return nil, err
		}
		out = append(out, FieldIR{
			GoName:     toPascalCase(f.Name),
			Type:       goType,
			Pointer:    !required && goType.Kind != "slice",
			Source:     source,
			SourceName: f.Name,
		})
	}
	return out, nil
}

func checkTypeRefs(t GoType, line int, types map[string]contract.TypeDef) error {
	target := t
	if t.Kind == "slice" {
		target = *t.Elem
	}
	if target.Kind == "named" {
		if _, ok := types[target.Name]; !ok {
			return fmt.Errorf("line %d: unknown type %q — define it under types:", line, target.Name)
		}
	}
	return nil
}

func resolveNamedTypes(defs []contract.TypeDef, types map[string]contract.TypeDef) ([]StructIR, error) {
	var out []StructIR
	for _, d := range defs {
		fields, err := resolveFields(d.Fields, types, SourceBody, map[string]bool{d.Name: true})
		if err != nil {
			return nil, err
		}
		if err := checkFieldNameCollisions(d.Name, fields); err != nil {
			return nil, err
		}
		out = append(out, StructIR{Name: d.Name, Fields: fields})
	}
	return out, nil
}

// checkFieldNameCollisions returns an error if two fields in the assembled
// struct share the same GoName (e.g. duplicate extends, or a path/body/
// query/header/middleware name collision).
func checkFieldNameCollisions(structName string, fields []FieldIR) error {
	seen := map[string]bool{}
	for _, f := range fields {
		if seen[f.GoName] {
			return fmt.Errorf("field %q appears more than once in %s (duplicate extends, or a path/body/query/header/middleware name collision) — rename one of them", f.GoName, structName)
		}
		seen[f.GoName] = true
	}
	return nil
}

func containsStr(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func resolveRoute(g contract.Group, r contract.Route, types map[string]contract.TypeDef, mwByName map[string]MiddlewareIR) (RouteIR, []StructIR, error) {
	fullPath := joinPath(g.Prefix, r.Path)
	methodName := r.Name
	if methodName == "" {
		methodName = defaultMethodName(r.Method, fullPath)
	}

	for _, name := range r.Skip {
		if !containsStr(g.Use, name) {
			return RouteIR{}, nil, fmt.Errorf("line %d: skip %q — not used by this group's \"use\"", r.Loc.Line, name)
		}
	}
	skip := map[string]bool{}
	for _, name := range r.Skip {
		skip[name] = true
	}

	var mws []MiddlewareIR
	for _, name := range g.Use {
		if skip[name] {
			continue
		}
		mw, ok := mwByName[name]
		if !ok {
			return RouteIR{}, nil, fmt.Errorf("line %d: unknown middleware %q", g.Loc.Line, name)
		}
		mws = append(mws, mw)
	}
	for _, name := range r.Use {
		mw, ok := mwByName[name]
		if !ok {
			return RouteIR{}, nil, fmt.Errorf("line %d: unknown middleware %q", r.Loc.Line, name)
		}
		mws = append(mws, mw)
	}

	var fields []FieldIR
	for _, p := range pathParams(fullPath) {
		fields = append(fields, FieldIR{GoName: toPascalCase(p), Type: GoType{Kind: "string"}, Source: SourcePath, SourceName: p})
	}
	for _, block := range []struct {
		f contract.Fields
		s Source
	}{{r.Body, SourceBody}, {r.Query, SourceQuery}, {r.Header, SourceHeader}} {
		resolved, err := resolveFields(block.f, types, block.s, map[string]bool{})
		if err != nil {
			return RouteIR{}, nil, err
		}
		fields = append(fields, resolved...)
	}
	for _, mw := range mws {
		if mw.Provides != "" {
			fields = append(fields, FieldIR{
				GoName:      toPascalCase(mw.Name),
				Type:        GoType{Kind: "named", Name: mw.Provides},
				Source:      SourceCtx,
				SourceName:  mw.Name,
				CtxKeyConst: mw.CtxKeyConst,
			})
		}
	}

	if err := checkFieldNameCollisions(methodName+"Request", fields); err != nil {
		return RouteIR{}, nil, err
	}

	request := StructIR{Name: methodName + "Request", Fields: fields}

	var respType string
	var extra []StructIR
	switch {
	case r.Resp.Empty:
		// respType stays "" -> 204 No Content
	case r.Resp.Ref != "":
		if _, ok := types[r.Resp.Ref]; !ok {
			return RouteIR{}, nil, fmt.Errorf("line %d: unknown type %q — define it under types:", r.Resp.Loc.Line, r.Resp.Ref)
		}
		respType = r.Resp.Ref
	default:
		respFields, err := resolveFields(r.Resp.Fields, types, SourceBody, map[string]bool{})
		if err != nil {
			return RouteIR{}, nil, err
		}
		respType = methodName + "Response"
		extra = append(extra, StructIR{Name: respType, Fields: respFields})
	}

	return RouteIR{
		Method:       r.Method,
		Path:         fullPath,
		MuxPattern:   r.Method + " " + fullPath,
		MethodName:   methodName,
		Request:      request,
		ResponseType: respType,
		Middlewares:  mws,
	}, extra, nil
}

// Resolve turns a parsed contract into its IR. It performs the lookups
// needed to build the IR (unknown type/middleware references fail here);
// the remaining §5.6 rules — duplicate routes, method name collisions,
// group shape, path-param/query collisions — are checked by Validate.
func Resolve(c *contract.Contract) (*ContractIR, error) {
	typesByName := map[string]contract.TypeDef{}
	for _, t := range c.Types {
		typesByName[t.Name] = t
	}
	namedStructs, err := resolveNamedTypes(c.Types, typesByName)
	if err != nil {
		return nil, err
	}

	mws := resolveMiddlewares(c.Middlewares)
	mwByName := map[string]MiddlewareIR{}
	for _, m := range mws {
		mwByName[m.Name] = m
	}

	out := &ContractIR{ServiceName: c.Service, Middlewares: mws, Types: namedStructs}
	for _, g := range c.Groups {
		for _, r := range g.Routes {
			route, extra, err := resolveRoute(g, r, typesByName, mwByName)
			if err != nil {
				return nil, err
			}
			out.Types = append(out.Types, route.Request)
			out.Types = append(out.Types, extra...)
			out.Routes = append(out.Routes, route)
		}
	}

	for _, cr := range c.Crons {
		// Validate already rejected anything unparseable; this only fires
		// if Resolve was called without it.
		every, err := time.ParseDuration(cr.Every)
		if err != nil {
			return nil, fmt.Errorf("line %d: cron %q: invalid every %q", cr.Loc.Line, cr.Name, cr.Every)
		}
		out.Crons = append(out.Crons, CronIR{
			Label:      cr.Name,
			MethodName: cronMethodName(cr),
			Every:      every,
			EverySrc:   cr.Every,
			OnStart:    cr.OnStart,
		})
	}
	return out, nil
}
