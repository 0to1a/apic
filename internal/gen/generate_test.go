package gen

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0to1a/apic/internal/contract"
	"github.com/0to1a/apic/internal/ir"
)

var update = flag.Bool("update", false, "write golden files instead of comparing against them")

func generateExample(t *testing.T) map[string][]byte {
	t.Helper()
	data, err := os.ReadFile("testdata/example.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c, err := contract.Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if errs := ir.Validate(c); len(errs) != 0 {
		t.Fatalf("Validate: %v", errs)
	}
	resolved, err := ir.Resolve(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	files, err := Generate(resolved)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return files
}

func generateYAML(t *testing.T, doc string) map[string][]byte {
	t.Helper()
	c, err := contract.Parse([]byte(doc))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if errs := ir.Validate(c); len(errs) != 0 {
		t.Fatalf("Validate: %v", errs)
	}
	resolved, err := ir.Resolve(c)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	files, err := Generate(resolved)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	return files
}

// writeModule writes files into dir/gen inside a throwaway module, so the
// generated package can be built and tested on its own.
func writeModule(t *testing.T, files map[string][]byte) string {
	t.Helper()
	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(genDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/testapp\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// TestGenerate_Golden compares Generate's output for testdata/example.yaml
// against the committed golden files in testdata/example/. Run with
// -update to (re)write them after an intentional codegen change.
func TestGenerate_Golden(t *testing.T) {
	files := generateExample(t)
	goldenDir := "testdata/example"

	if *update {
		if err := os.MkdirAll(goldenDir, 0o755); err != nil {
			t.Fatal(err)
		}
		for name, content := range files {
			if err := os.WriteFile(filepath.Join(goldenDir, name), content, 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return
	}

	for name, content := range files {
		want, err := os.ReadFile(filepath.Join(goldenDir, name))
		if err != nil {
			t.Fatalf("read golden %s (run `go test ./internal/gen/... -update` if this is expected): %v", name, err)
		}
		if string(want) != string(content) {
			t.Errorf("%s does not match golden. Run `go test ./internal/gen/... -update` if this change is expected.\n--- got ---\n%s", name, content)
		}
	}
}

// TestGenerate_SelfContained renders the example contract into its own
// module (no require/replace of github.com/0to1a/apic at all) and go-builds
// + go-vets it, proving the generated package is self-contained: it needs
// nothing from the apic module at build time.
func TestGenerate_SelfContained(t *testing.T) {
	files := generateExample(t)

	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(genDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	goMod := "module example.com/testapp\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, out)
		}
	}

	cmd := exec.Command("go", "list", "-deps", "./gen")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps ./gen failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "github.com/0to1a/apic") {
		t.Errorf("generated package must not depend on github.com/0to1a/apic, got deps:\n%s", out)
	}
}

// TestGenerate_MiddlewareIntegration proves an external middleware (unable to
// import anything unexported) can legally inject the context value a
// generated route with `use: [auth]` expects, via the generated WithAuth
// helper — the bug §1 of the design doc fixes.
func TestGenerate_MiddlewareIntegration(t *testing.T) {
	files := generateExample(t)

	dir := t.TempDir()
	genDir := filepath.Join(dir, "gen")
	if err := os.MkdirAll(genDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(genDir, name), content, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	const testSrc = `package gen

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubService struct{ Service }

func (stubService) PostLogin(ctx context.Context, req *PostLoginRequest) (*PostLoginResponse, error) {
	return &PostLoginResponse{Token: req.Auth.ID}, nil
}

// authMiddleware lives conceptually outside the gen package (it only uses
// exported names) but is compiled in-package here for test convenience.
func authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := WithAuth(r.Context(), User{ID: "u1"})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func TestAuthMiddlewareInjectsContext(t *testing.T) {
	mux := http.NewServeMux()
	mw := Middlewares{Auth: authMiddleware, Ratelimit: passthrough, Admin: passthrough}
	if err := RegisterRoutes(mux, stubService{}, mw); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest("POST", "/login", strings.NewReader("{\"username\":\"a\",\"password\":\"b\"}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func passthrough(next http.Handler) http.Handler { return next }
`
	if err := os.WriteFile(filepath.Join(genDir, "integration_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	goMod := "module example.com/testapp\n\ngo 1.22\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... failed: %v\n%s", err, out)
	}
}

// TestGenerate_WithHelper checks that middleware.go gets a With<Name> helper
// for every middleware declaring `provides` (auth), and none for those that
// don't (ratelimit, admin).
func TestGenerate_WithHelper(t *testing.T) {
	files := generateExample(t)
	src := string(files["middleware.go"])

	if !strings.Contains(src, "func WithAuth(ctx context.Context, v User) context.Context {") {
		t.Errorf("expected a WithAuth helper for the auth middleware (provides: User), got:\n%s", src)
	}
	for _, name := range []string{"WithRatelimit", "WithAdmin"} {
		if strings.Contains(src, "func "+name) {
			t.Errorf("did not expect a %s helper (no provides), got:\n%s", name, src)
		}
	}
}

// TestGenerate_CronIntegration runs the generated RunCrons against a stub
// service: the `on_start: true` job must fire immediately (its interval is
// 1h, so a tick can't explain it) and RunCrons must return once ctx is
// cancelled.
func TestGenerate_CronIntegration(t *testing.T) {
	dir := writeModule(t, generateExample(t))

	const testSrc = `package gen

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type cronService struct {
	Service
	cleanups atomic.Int32
}

func (s *cronService) CleanupSessions(ctx context.Context) error {
	s.cleanups.Add(1)
	return nil
}

func (s *cronService) SyncStock(ctx context.Context) error { return nil }

func TestRunCronsOnStart(t *testing.T) {
	svc := &cronService{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		RunCrons(ctx, svc)
		close(done)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for svc.cleanups.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("cleanup-sessions has on_start: true but was not called")
		}
		time.Sleep(5 * time.Millisecond)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("RunCrons did not return after ctx was cancelled")
	}

	if got := svc.cleanups.Load(); got != 1 {
		t.Fatalf("cleanup-sessions called %d times, want exactly 1 (its interval is 1h)", got)
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "gen", "crons_integration_test.go"), []byte(testSrc), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("go", "test", "./...")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go test ./... failed: %v\n%s", err, out)
	}
}

// TestGenerate_NoCronsStillEmitsCrons checks that crons.go is emitted (and
// compiles) for a contract without any `crons:` block. It's emitted
// unconditionally on purpose: `apic generate` never deletes files, so a
// conditionally emitted crons.go would linger forever — and be reported as
// stale by `apic diff` — the moment someone drops their crons: block.
func TestGenerate_NoCronsStillEmitsCrons(t *testing.T) {
	files := generateYAML(t, `
version: 1
service: Service
groups:
  - prefix: /
    use: []
    routes:
      - GET /health:
          resp:
            status: str!
`)
	if _, ok := files["crons.go"]; !ok {
		t.Fatal("crons.go must be emitted even when the contract has no crons")
	}

	dir := writeModule(t, files)
	for _, args := range [][]string{{"build", "./..."}, {"vet", "./..."}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("go %v failed: %v\n%s", args, err, out)
		}
	}
}
