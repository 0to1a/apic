package contract

import (
	"strings"
	"testing"
)

const example = `
version: 1
service: Service

middlewares:
  auth:
    provides: User
  ratelimit:
  admin:

types:
  User:
    id: str!
    name: str!
  Page:
    limit: int
    cursor: str
  Post:
    extends: Page
    id: str!
    extends: Page
    title: str!

groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
          resp:
            status: str!

  - prefix: /
    use: [auth]
    routes:
      - GET /profile:
          resp: User
      - GET /posts:
          query:
            q: str
            extends: Page
          header:
            X-Client-Id: str
          resp:
            items: "[]Post!"
            next_cursor: str
      - GET /posts/{id}:
          resp: Post
      - DELETE /posts/{id}:
      - GET /posts/{id}/comments:
          skip: [auth]
          resp:
            items: "[]Comment!"
`

func TestParse_Example(t *testing.T) {
	c, err := Parse([]byte(example))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Version != 1 || c.Service != "Service" {
		t.Fatalf("got version=%d service=%q", c.Version, c.Service)
	}
	if len(c.Middlewares) != 3 {
		t.Fatalf("expected 3 middlewares, got %d", len(c.Middlewares))
	}
	if c.Middlewares[0].Name != "auth" || c.Middlewares[0].Provides != "User" {
		t.Fatalf("auth middleware = %+v", c.Middlewares[0])
	}

	post := c.Types[2]
	if post.Name != "Post" {
		t.Fatalf("expected Post, got %q", post.Name)
	}
	extendsCount := 0
	for _, f := range post.Fields {
		if f.Extends != "" {
			extendsCount++
		}
	}
	if extendsCount != 2 {
		t.Fatalf("expected 2 extends entries preserved, got %d in %+v", extendsCount, post.Fields)
	}

	if len(c.Groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(c.Groups))
	}
	g2 := c.Groups[1]
	if len(g2.Routes) != 5 {
		t.Fatalf("expected 5 routes in group 2, got %d", len(g2.Routes))
	}

	del := g2.Routes[3]
	if del.Method != "DELETE" || del.Path != "/posts/{id}" || !del.Resp.Empty {
		t.Fatalf("DELETE route = %+v", del)
	}

	getPosts := g2.Routes[1]
	if len(getPosts.Query) != 2 || getPosts.Query[1].Extends != "Page" {
		t.Fatalf("GET /posts query = %+v", getPosts.Query)
	}
	if len(getPosts.Resp.Fields) != 2 {
		t.Fatalf("GET /posts resp fields = %+v", getPosts.Resp.Fields)
	}

	comments := g2.Routes[4]
	if len(comments.Skip) != 1 || comments.Skip[0] != "auth" {
		t.Fatalf("comments skip = %+v", comments.Skip)
	}
}

func TestParse_BadRouteLineNoColon(t *testing.T) {
	bad := `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /health
        resp:
          status: str!
`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatalf("expected a parse error for missing route colon")
	}
}

func TestParse_MissingGroups(t *testing.T) {
	bad := `
version: 1
service: Service
`
	if _, err := Parse([]byte(bad)); err == nil {
		t.Fatalf("expected error for missing groups")
	}
}

func TestParse_OutField(t *testing.T) {
	doc := `
out: build/server

version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
`
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Out != "build/server" {
		t.Fatalf("Out = %q, want %q", c.Out, "build/server")
	}
}

func TestParse_OutFieldDefaultsToEmpty(t *testing.T) {
	c, err := Parse([]byte(example))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.Out != "" {
		t.Fatalf("Out = %q, want empty string (no default applied by contract.Parse)", c.Out)
	}
}

func TestParse_TSField(t *testing.T) {
	doc := `
ts: web/src/lib/gen/api.ts

version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
`
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.TS != "web/src/lib/gen/api.ts" {
		t.Fatalf("TS = %q, want %q", c.TS, "web/src/lib/gen/api.ts")
	}
}

func TestParse_TSFieldDefaultsToEmpty(t *testing.T) {
	c, err := Parse([]byte(example))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if c.TS != "" {
		t.Fatalf("TS = %q, want empty string (no TS generated unless ts: is present)", c.TS)
	}
}

func TestParse_Crons(t *testing.T) {
	doc := `
version: 1
service: Service
crons:
  - cleanup-sessions:
      every: 1h
      on_start: true
  - sync-inventory:
      every: 30s
      name: SyncStock
groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
`
	c, err := Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	want := []Cron{
		{Name: "cleanup-sessions", Every: "1h", OnStart: true, Loc: Loc{Line: 5, Col: 5}},
		{Name: "sync-inventory", Every: "30s", Method: "SyncStock", Loc: Loc{Line: 8, Col: 5}},
	}
	if len(c.Crons) != len(want) {
		t.Fatalf("got %d crons, want %d: %+v", len(c.Crons), len(want), c.Crons)
	}
	for i, w := range want {
		if c.Crons[i] != w {
			t.Errorf("cron %d = %+v, want %+v", i, c.Crons[i], w)
		}
	}
}

func TestParse_CronsDefaultsToEmpty(t *testing.T) {
	c, err := Parse([]byte(example))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(c.Crons) != 0 {
		t.Fatalf("Crons = %+v, want empty (no crons: key present)", c.Crons)
	}
}

func TestParse_CronErrors(t *testing.T) {
	const head = "version: 1\nservice: Service\ngroups:\n  - prefix: /\n    use: []\n    routes:\n      - GET /health:\n"
	tests := []struct {
		name, crons, want string
	}{
		{"on_start not a bool", "crons:\n  - job:\n      every: 1h\n      on_start: yesterday\n", "on_start must be true or false"},
		{"unknown key", "crons:\n  - job:\n      every: 1h\n      timezone: UTC\n", `unknown cron key "timezone"`},
		{"not a list", "crons:\n  job:\n    every: 1h\n", "crons must be a list"},
		{"body not a mapping", "crons:\n  - job: 1h\n", "cron body must be a mapping"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(head + tt.crons))
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want it to contain %q", err, tt.want)
			}
		})
	}
}
