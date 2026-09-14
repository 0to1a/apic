package contract

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Loc is a YAML source location, used to build error messages that point
// at the exact line the problem is on.
type Loc struct {
	Line, Col int
}

// FieldEntry is one line inside a fields block: either a named field
// ("name: type") or an `extends: Name` directive. A fields block preserves
// declaration order and may contain more than one Extends entry.
type FieldEntry struct {
	Name    string // "" when Extends != ""
	Type    string // raw type expr, e.g. "str!", "[]Post!", "Post"
	Extends string // "" unless this entry is `extends: Name`
	Loc     Loc
}

type Fields []FieldEntry

type Resp struct {
	Ref    string // named type reference, e.g. "Post" — mutually exclusive with Fields
	Fields Fields
	Empty  bool // true when the route has no `resp` key (-> HTTP 204)
	Loc    Loc
}

type Route struct {
	Method string
	Path   string
	Name   string
	Body   Fields
	Query  Fields
	Header Fields
	Resp   Resp
	Use    []string
	Skip   []string
	Loc    Loc
}

// Cron is one entry under the top-level `crons:` key: a job name plus the
// interval it runs at. Every is kept as the raw contract string — parsing it
// into a time.Duration is ir.Validate's job, like every other semantic rule.
type Cron struct {
	Name    string // job name as written, e.g. "cleanup-sessions"
	Every   string // raw duration expr, e.g. "1h"
	OnStart bool   // run once at RunCrons start, before the first tick
	Method  string // `name:` override for the Service method; "" means derive it
	Loc     Loc
}

type Group struct {
	Prefix    string
	Use       []string
	Routes    []Route
	PrefixSet bool // true iff "prefix:" was present — a group with none is a §5.6 error
	UseSet    bool // true iff "use:" was present — same
	Loc       Loc
}

type Middleware struct {
	Name     string
	Provides string
	Loc      Loc
}

type TypeDef struct {
	Name   string
	Fields Fields
	Loc    Loc
}

type Contract struct {
	Version     int
	Service     string
	Out         string // output directory for generated files; CLI-only. Empty means "no out: key was present" — cmd/apic applies the "gen/" default, not this package.
	TS          string // path to a single generated TypeScript client file; CLI-only. Empty means "don't generate TS at all" — unlike Out, there is no default.
	Middlewares []Middleware
	Types       []TypeDef
	Crons       []Cron
	Groups      []Group
}

// Parse reads a contract from YAML source. It only rejects structurally
// broken YAML (wrong node kinds, unknown keys, malformed route lines) — the
// semantic rules in the design doc (§5.6) are checked later by ir.Validate.
func Parse(data []byte) (*Contract, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML: %w", err)
	}
	if len(root.Content) == 0 {
		return nil, fmt.Errorf("empty contract")
	}
	doc := root.Content[0]
	if doc.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: contract root must be a mapping", doc.Line)
	}

	c := &Contract{}
	for i := 0; i < len(doc.Content); i += 2 {
		key, val := doc.Content[i], doc.Content[i+1]
		var err error
		switch key.Value {
		case "version":
			c.Version, err = strconv.Atoi(val.Value)
		case "service":
			c.Service = val.Value
		case "out":
			c.Out = val.Value
		case "ts":
			c.TS = val.Value
		case "middlewares":
			c.Middlewares, err = parseMiddlewares(val)
		case "types":
			c.Types, err = parseTypeDefs(val)
		case "crons":
			c.Crons, err = parseCrons(val)
		case "groups":
			c.Groups, err = parseGroups(val)
		default:
			err = fmt.Errorf("line %d: unknown top-level key %q", key.Line, key.Value)
		}
		if err != nil {
			return nil, err
		}
	}
	if c.Groups == nil {
		return nil, fmt.Errorf("line %d: \"groups\" is required", doc.Line)
	}
	return c, nil
}

func isNull(node *yaml.Node) bool {
	return node == nil || node.Tag == "!!null"
}

func parseStringList(node *yaml.Node) ([]string, error) {
	if isNull(node) {
		return nil, nil
	}
	if node.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: expected a list", node.Line)
	}
	list := make([]string, 0, len(node.Content))
	for _, item := range node.Content {
		list = append(list, item.Value)
	}
	return list, nil
}

// parseFields walks a mapping node into an ordered Fields list, keeping
// every `extends:` entry (a plain map would silently drop all but the last).
func parseFields(node *yaml.Node) (Fields, error) {
	if isNull(node) {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: expected a mapping of field: type", node.Line)
	}
	var fields Fields
	for i := 0; i < len(node.Content); i += 2 {
		key, val := node.Content[i], node.Content[i+1]
		loc := Loc{key.Line, key.Column}
		if key.Value == "extends" {
			fields = append(fields, FieldEntry{Extends: val.Value, Loc: loc})
			continue
		}
		fields = append(fields, FieldEntry{Name: key.Value, Type: val.Value, Loc: loc})
	}
	return fields, nil
}

func parseResp(node *yaml.Node) (Resp, error) {
	if isNull(node) {
		return Resp{Empty: true}, nil
	}
	loc := Loc{node.Line, node.Column}
	if node.Kind == yaml.ScalarNode {
		return Resp{Ref: node.Value, Loc: loc}, nil
	}
	fields, err := parseFields(node)
	if err != nil {
		return Resp{}, err
	}
	return Resp{Fields: fields, Loc: loc}, nil
}

var routeMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
}

func splitRouteLine(s string) (method, path string, err error) {
	parts := strings.SplitN(s, " ", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("route %q must be \"METHOD /path\"", s)
	}
	method, path = parts[0], parts[1]
	if !routeMethods[method] {
		return "", "", fmt.Errorf("route %q: unknown method %q", s, method)
	}
	if !strings.HasPrefix(path, "/") {
		return "", "", fmt.Errorf("route %q: path must start with \"/\"", s)
	}
	return method, path, nil
}

func parseRoutes(seq *yaml.Node) ([]Route, error) {
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: routes must be a list", seq.Line)
	}
	var routes []Route
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return nil, fmt.Errorf("line %d: route must be written \"METHOD /path:\" followed by its keys (note the trailing colon)", item.Line)
		}
		keyNode, valNode := item.Content[0], item.Content[1]
		method, path, err := splitRouteLine(keyNode.Value)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", keyNode.Line, err)
		}
		r := Route{Method: method, Path: path, Loc: Loc{keyNode.Line, keyNode.Column}, Resp: Resp{Empty: true}}
		if !isNull(valNode) {
			if valNode.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("line %d: route body must be a mapping", valNode.Line)
			}
			for i := 0; i < len(valNode.Content); i += 2 {
				k, v := valNode.Content[i], valNode.Content[i+1]
				switch k.Value {
				case "name":
					r.Name = v.Value
				case "body":
					r.Body, err = parseFields(v)
				case "query":
					r.Query, err = parseFields(v)
				case "header":
					r.Header, err = parseFields(v)
				case "resp":
					r.Resp, err = parseResp(v)
				case "use":
					r.Use, err = parseStringList(v)
				case "skip":
					r.Skip, err = parseStringList(v)
				default:
					err = fmt.Errorf("line %d: unknown route key %q", k.Line, k.Value)
				}
				if err != nil {
					return nil, err
				}
			}
		}
		routes = append(routes, r)
	}
	return routes, nil
}

func parseGroups(seq *yaml.Node) ([]Group, error) {
	if isNull(seq) {
		return nil, fmt.Errorf("line %d: \"groups\" is required", seq.Line)
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: groups must be a list", seq.Line)
	}
	var groups []Group
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode {
			return nil, fmt.Errorf("line %d: group must be a mapping", item.Line)
		}
		g := Group{Loc: Loc{item.Line, item.Column}}
		var err error
		for i := 0; i < len(item.Content); i += 2 {
			k, v := item.Content[i], item.Content[i+1]
			switch k.Value {
			case "prefix":
				g.Prefix = v.Value
				g.PrefixSet = true
			case "use":
				g.Use, err = parseStringList(v)
				g.UseSet = true
			case "routes":
				g.Routes, err = parseRoutes(v)
			default:
				err = fmt.Errorf("line %d: unknown group key %q", k.Line, k.Value)
			}
			if err != nil {
				return nil, err
			}
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func parseMiddlewares(node *yaml.Node) ([]Middleware, error) {
	if isNull(node) {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: middlewares must be a mapping", node.Line)
	}
	var mws []Middleware
	for i := 0; i < len(node.Content); i += 2 {
		name, val := node.Content[i], node.Content[i+1]
		m := Middleware{Name: name.Value, Loc: Loc{name.Line, name.Column}}
		if !isNull(val) {
			if val.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("line %d: middleware %q must be a mapping or empty", val.Line, name.Value)
			}
			for j := 0; j < len(val.Content); j += 2 {
				k, v := val.Content[j], val.Content[j+1]
				if k.Value != "provides" {
					return nil, fmt.Errorf("line %d: unknown middleware key %q", k.Line, k.Value)
				}
				m.Provides = v.Value
			}
		}
		mws = append(mws, m)
	}
	return mws, nil
}

// parseCrons reads the top-level `crons:` sequence. Same shape as routes: a
// list of single-entry mappings, key = job name, value = the job's keys (or
// null when it has none, which ir.Validate then rejects for lacking `every`).
func parseCrons(seq *yaml.Node) ([]Cron, error) {
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("line %d: crons must be a list", seq.Line)
	}
	var crons []Cron
	for _, item := range seq.Content {
		if item.Kind != yaml.MappingNode || len(item.Content) != 2 {
			return nil, fmt.Errorf("line %d: cron must be written \"job-name:\" followed by its keys (note the trailing colon)", item.Line)
		}
		keyNode, valNode := item.Content[0], item.Content[1]
		cr := Cron{Name: keyNode.Value, Loc: Loc{keyNode.Line, keyNode.Column}}
		if !isNull(valNode) {
			if valNode.Kind != yaml.MappingNode {
				return nil, fmt.Errorf("line %d: cron body must be a mapping", valNode.Line)
			}
			for i := 0; i < len(valNode.Content); i += 2 {
				k, v := valNode.Content[i], valNode.Content[i+1]
				switch k.Value {
				case "every":
					cr.Every = v.Value
				case "name":
					cr.Method = v.Value
				case "on_start":
					b, err := strconv.ParseBool(v.Value)
					if err != nil {
						return nil, fmt.Errorf("line %d: on_start must be true or false", v.Line)
					}
					cr.OnStart = b
				default:
					return nil, fmt.Errorf("line %d: unknown cron key %q", k.Line, k.Value)
				}
			}
		}
		crons = append(crons, cr)
	}
	return crons, nil
}

func parseTypeDefs(node *yaml.Node) ([]TypeDef, error) {
	if isNull(node) {
		return nil, nil
	}
	if node.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("line %d: types must be a mapping", node.Line)
	}
	var types []TypeDef
	for i := 0; i < len(node.Content); i += 2 {
		name, val := node.Content[i], node.Content[i+1]
		fields, err := parseFields(val)
		if err != nil {
			return nil, err
		}
		types = append(types, TypeDef{Name: name.Value, Fields: fields, Loc: Loc{name.Line, name.Column}})
	}
	return types, nil
}
