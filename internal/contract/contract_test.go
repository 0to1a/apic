package contract

import "testing"

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
