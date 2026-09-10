package ir

import (
	"strings"
	"testing"

	"github.com/0to1a/apic/internal/contract"
)

func validateYAML(t *testing.T, doc string) []error {
	t.Helper()
	c, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return Validate(c)
}

func assertOneError(t *testing.T, errs []error, substr string) {
	t.Helper()
	for _, e := range errs {
		if strings.Contains(e.Error(), substr) {
			return
		}
	}
	t.Fatalf("expected an error containing %q, got: %v", substr, errs)
}

func TestValidate_ValidContractHasNoErrors(t *testing.T) {
	errs := validateYAML(t, exampleYAML)
	if len(errs) != 0 {
		t.Fatalf("expected no errors, got %v", errs)
	}
}

func TestValidate_GroupMissingPrefixAndUse(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - routes:
      - GET /health:
`)
	assertOneError(t, errs, `missing "prefix"`)
	assertOneError(t, errs, `missing "use"`)
}

func TestValidate_UnknownType(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /posts:
          resp: Comment
`)
	assertOneError(t, errs, `unknown type "Comment"`)
}

func TestValidate_UnknownMiddleware(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: [auth]
    routes:
      - GET /posts:
`)
	assertOneError(t, errs, `unknown middleware "auth"`)
}

func TestValidate_SkipNotInGroupUse(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
middlewares:
  auth:
groups:
  - prefix: /
    use: []
    routes:
      - GET /posts:
          skip: [auth]
`)
	assertOneError(t, errs, `skip "auth" not used by this group`)
}

func TestValidate_DuplicateRoute(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /posts:
      - GET /posts:
`)
	assertOneError(t, errs, `duplicate route "GET /posts"`)
}

func TestValidate_MethodNameCollision(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - DELETE /users/{id}:
  - prefix: /admin
    use: []
    routes:
      - DELETE /users/{id}:
          name: DeleteUsersByID
`)
	assertOneError(t, errs, `method name "DeleteUsersByID" already used by`)
}

func TestValidate_PathParamCollidesWithQuery(t *testing.T) {
	errs := validateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /posts/{id}:
          query:
            id: str
`)
	assertOneError(t, errs, `field "id" collides with path parameter {id}`)
}
