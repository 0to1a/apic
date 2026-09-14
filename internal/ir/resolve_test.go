package ir

import (
	"strings"
	"testing"
	"time"

	"github.com/0to1a/apic/internal/contract"
)

const exampleYAML = `
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
      - POST /login:
          body:
            username: str!
            password: str!
          resp:
            token: str!
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
            items: "[]Post!"
`

func mustResolve(t *testing.T) *ContractIR {
	t.Helper()
	c, err := contract.Parse([]byte(exampleYAML))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	out, err := Resolve(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return out
}

func TestResolve_Example(t *testing.T) {
	out := mustResolve(t)

	if out.ServiceName != "Service" {
		t.Fatalf("service = %q", out.ServiceName)
	}
	if len(out.Middlewares) != 3 || out.Middlewares[0].GoField != "Auth" || out.Middlewares[0].CtxKeyConst != "ctxKeyAuth" {
		t.Fatalf("middlewares = %+v", out.Middlewares)
	}

	byName := map[string]RouteIR{}
	for _, r := range out.Routes {
		byName[r.MethodName] = r
	}

	login, ok := byName["PostLogin"]
	if !ok {
		t.Fatalf("expected PostLogin route, got %v", keys(byName))
	}
	if login.ResponseType != "PostLoginResponse" {
		t.Fatalf("login resp type = %q", login.ResponseType)
	}
	if len(login.Request.Fields) != 3 { // username, password, + ctx auth (group applies "use: [auth]" to every route)
		t.Fatalf("login request fields = %+v", login.Request.Fields)
	}

	getByID, ok := byName["GetPostsByID"]
	if !ok {
		t.Fatalf("expected GetPostsByID, got %v", keys(byName))
	}
	if getByID.ResponseType != "Post" {
		t.Fatalf("expected resp type Post, got %q", getByID.ResponseType)
	}
	if len(getByID.Request.Fields) != 2 || getByID.Request.Fields[0].GoName != "ID" || getByID.Request.Fields[0].Source != SourcePath {
		t.Fatalf("getByID request fields = %+v", getByID.Request.Fields)
	}

	del, ok := byName["DeletePostsByID"]
	if !ok {
		t.Fatalf("expected DeletePostsByID, got %v", keys(byName))
	}
	if del.ResponseType != "" {
		t.Fatalf("expected empty resp (204), got %q", del.ResponseType)
	}

	getPosts, ok := byName["GetPosts"]
	if !ok {
		t.Fatalf("expected GetPosts, got %v", keys(byName))
	}
	// q + extends Page (limit, cursor) + header X-Client-Id + ctx auth(User) = 5
	if len(getPosts.Request.Fields) != 5 {
		t.Fatalf("GetPosts request fields = %+v", getPosts.Request.Fields)
	}
	var ctxField *FieldIR
	for i := range getPosts.Request.Fields {
		if getPosts.Request.Fields[i].Source == SourceCtx {
			ctxField = &getPosts.Request.Fields[i]
		}
	}
	if ctxField == nil || ctxField.Type.Name != "User" {
		t.Fatalf("expected ctx field of type User, got %+v", getPosts.Request.Fields)
	}

	comments, ok := byName["GetPostsByIDComments"]
	if !ok {
		t.Fatalf("expected GetPostsByIDComments, got %v", keys(byName))
	}
	if len(comments.Middlewares) != 0 {
		t.Fatalf("expected auth skipped, got %+v", comments.Middlewares)
	}
}

func TestResolve_DuplicateExtendsCollision(t *testing.T) {
	yaml := `
version: 1
service: Service

types:
  Page:
    limit: int
    cursor: str
  Post:
    extends: Page
    extends: Page
    id: str!

groups:
  - prefix: /
    use: []
    routes:
      - GET /posts:
          resp: Post
`
	c, err := contract.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = Resolve(c)
	if err == nil {
		t.Fatal("expected error for duplicate extends collision, got nil")
	}
	if !strings.Contains(err.Error(), "Limit") {
		t.Fatalf("expected error mentioning colliding field %q, got: %v", "Limit", err)
	}
}

func TestResolve_PathParamBodyFieldCollision(t *testing.T) {
	yaml := `
version: 1
service: Service

groups:
  - prefix: /
    use: []
    routes:
      - PUT /posts/{id}:
          body:
            id: str!
`
	c, err := contract.Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_, err = Resolve(c)
	if err == nil {
		t.Fatal("expected error for path/body field name collision, got nil")
	}
	if !strings.Contains(err.Error(), "ID") {
		t.Fatalf("expected error mentioning colliding field %q, got: %v", "ID", err)
	}
}

func keys(m map[string]RouteIR) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestResolve_Crons(t *testing.T) {
	c, err := contract.Parse([]byte(`
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
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	resolved, err := Resolve(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := []CronIR{
		{Label: "cleanup-sessions", MethodName: "CleanupSessions", Every: time.Hour, EverySrc: "1h", OnStart: true},
		{Label: "sync-inventory", MethodName: "SyncStock", Every: 30 * time.Second, EverySrc: "30s"},
	}
	if len(resolved.Crons) != len(want) {
		t.Fatalf("got %d crons, want %d: %+v", len(resolved.Crons), len(want), resolved.Crons)
	}
	for i, w := range want {
		if resolved.Crons[i] != w {
			t.Errorf("cron %d = %+v, want %+v", i, resolved.Crons[i], w)
		}
	}
}
