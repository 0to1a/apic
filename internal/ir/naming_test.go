package ir

import "testing"

func TestToPascalCase(t *testing.T) {
	cases := map[string]string{
		"next_cursor": "NextCursor",
		"id":          "ID",
		"user_id":     "UserID",
		"X-Client-Id": "XClientID",
		"q":           "Q",
	}
	for in, want := range cases {
		if got := toPascalCase(in); got != want {
			t.Errorf("toPascalCase(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDefaultMethodName(t *testing.T) {
	cases := []struct{ method, path, want string }{
		{"GET", "/posts/{id}", "GetPostsByID"},
		{"POST", "/login", "PostLogin"},
		{"GET", "/posts/{id}/comments", "GetPostsByIDComments"},
		{"DELETE", "/posts/{id}", "DeletePostsByID"},
	}
	for _, tc := range cases {
		if got := defaultMethodName(tc.method, tc.path); got != tc.want {
			t.Errorf("defaultMethodName(%q, %q) = %q, want %q", tc.method, tc.path, got, tc.want)
		}
	}
}

func TestParseTypeExpr(t *testing.T) {
	gt, required := parseTypeExpr("str!")
	if gt.Kind != "string" || !required {
		t.Fatalf("str! -> %+v required=%v", gt, required)
	}
	gt, required = parseTypeExpr("[]Post!")
	if gt.Kind != "slice" || gt.Elem.Kind != "named" || gt.Elem.Name != "Post" || !required {
		t.Fatalf("[]Post! -> %+v required=%v", gt, required)
	}
	gt, required = parseTypeExpr("cursor")
	if gt.Kind != "named" || gt.Name != "cursor" || required {
		t.Fatalf("cursor -> %+v required=%v", gt, required)
	}
}
