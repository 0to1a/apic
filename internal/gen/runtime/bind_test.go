package runtime

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type getPostsRequest struct {
	Q          *string `query:"q" json:"-"`
	Limit      *int64  `query:"limit" json:"-"`
	ClientID   *string `header:"X-Client-Id" json:"-"`
	AuthUserID string  `json:"-"` // set directly by generated code, not by Bind
}

type getPostRequest struct {
	ID string `path:"id" json:"-"`
}

func TestBind_BodyRequired(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"alice","password":"secret"}`))
	var dst loginRequest
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dst.Username != "alice" || dst.Password != "secret" {
		t.Fatalf("got %+v", dst)
	}
}

func TestBind_BodyMissingRequired(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"username":"alice"}`))
	var dst loginRequest
	err := Bind(req, &dst)
	st, ok := err.(*Status)
	if !ok || st.Code != InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
	if _, ok := st.Details["body.password"]; !ok {
		t.Fatalf("expected body.password in details, got %v", st.Details)
	}
}

func TestBind_QueryAndHeaderOptional(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts?q=hi&limit=5", nil)
	req.Header.Set("X-Client-Id", "abc")
	var dst getPostsRequest
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dst.Q == nil || *dst.Q != "hi" {
		t.Fatalf("Q = %v", dst.Q)
	}
	if dst.Limit == nil || *dst.Limit != 5 {
		t.Fatalf("Limit = %v", dst.Limit)
	}
	if dst.ClientID == nil || *dst.ClientID != "abc" {
		t.Fatalf("ClientID = %v", dst.ClientID)
	}
}

func TestBind_QueryAndHeaderAbsent(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	var dst getPostsRequest
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dst.Q != nil || dst.Limit != nil || dst.ClientID != nil {
		t.Fatalf("expected all nil, got %+v %+v %+v", dst.Q, dst.Limit, dst.ClientID)
	}
}

func TestBind_PathRequired(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/posts/42", nil)
	req.SetPathValue("id", "42")
	var dst getPostRequest
	if err := Bind(req, &dst); err != nil {
		t.Fatalf("Bind: %v", err)
	}
	if dst.ID != "42" {
		t.Fatalf("ID = %q", dst.ID)
	}
}
