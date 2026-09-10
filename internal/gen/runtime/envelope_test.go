package runtime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestEncodeSuccess_OmitsMessageAndDetails(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeSuccess(&buf, map[string]string{"id": "1"}); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != float64(0) || got["status"] != "OK" {
		t.Fatalf("got %v", got)
	}
	if _, ok := got["message"]; ok {
		t.Fatalf("expected no \"message\" key, got %v", got)
	}
	if _, ok := got["data"]; !ok {
		t.Fatalf("expected \"data\" key, got %v", got)
	}
}

func TestEncodeStatus_OmitsData(t *testing.T) {
	var buf bytes.Buffer
	st := Error(NotFound, "post not found")
	if err := EncodeStatus(&buf, st); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["code"] != float64(5) || got["status"] != "NOT_FOUND" || got["message"] != "post not found" {
		t.Fatalf("got %v", got)
	}
	if _, ok := got["data"]; ok {
		t.Fatalf("expected no \"data\" key, got %v", got)
	}
}

type testLogger struct{ lines []string }

func (l *testLogger) Printf(format string, args ...any) {
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func TestWriteError_LogsNonStatusErrors(t *testing.T) {
	logger := &testLogger{}
	cfg := NewConfig(WithLogger(logger))
	w := httptest.NewRecorder()
	WriteError(w, cfg, fmt.Errorf("db exploded"))
	if w.Code != 500 {
		t.Fatalf("status = %d, want 500", w.Code)
	}
	if len(logger.lines) != 1 {
		t.Fatalf("expected the original error logged, got %v", logger.lines)
	}
	var got map[string]any
	json.Unmarshal(w.Body.Bytes(), &got)
	if got["message"] != "internal error" {
		t.Fatalf("expected a generic body message, got %v", got)
	}
}

func TestWriteError_StatusNotLogged(t *testing.T) {
	logger := &testLogger{}
	cfg := NewConfig(WithLogger(logger))
	w := httptest.NewRecorder()
	WriteError(w, cfg, Error(NotFound, "post not found"))
	if w.Code != 404 {
		t.Fatalf("status = %d, want 404", w.Code)
	}
	if len(logger.lines) != 0 {
		t.Fatalf("expected *Status errors not to be logged, got %v", logger.lines)
	}
}

func TestWriteSuccess_UsesStatusCoder(t *testing.T) {
	cfg := NewConfig()
	w := httptest.NewRecorder()
	WriteSuccess(w, cfg, created{})
	if w.Code != 201 {
		t.Fatalf("status = %d, want 201", w.Code)
	}
}

type created struct{}

func (created) StatusCode() int { return 201 }

func TestWriteNoContent(t *testing.T) {
	w := httptest.NewRecorder()
	WriteNoContent(w)
	if w.Code != 204 {
		t.Fatalf("status = %d, want 204", w.Code)
	}
}
