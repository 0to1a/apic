package apic

import "testing"

func TestCode_StringAndHTTPStatus(t *testing.T) {
	cases := []struct {
		code Code
		name string
		http int
	}{
		{OK, "OK", 200},
		{NotFound, "NOT_FOUND", 404},
		{InvalidArgument, "INVALID_ARGUMENT", 400},
		{Unauthenticated, "UNAUTHENTICATED", 401},
		{PermissionDenied, "PERMISSION_DENIED", 403},
		{ResourceExhausted, "RESOURCE_EXHAUSTED", 429},
		{Internal, "INTERNAL", 500},
		{Unavailable, "UNAVAILABLE", 503},
	}
	for _, tc := range cases {
		if got := tc.code.String(); got != tc.name {
			t.Errorf("%v.String() = %q, want %q", tc.code, got, tc.name)
		}
		if got := tc.code.HTTPStatus(); got != tc.http {
			t.Errorf("%v.HTTPStatus() = %d, want %d", tc.code, got, tc.http)
		}
	}
}

func TestCode_UnknownDefaultsTo500(t *testing.T) {
	var c Code = 999
	if got := c.String(); got != "UNKNOWN" {
		t.Errorf("String() = %q, want UNKNOWN", got)
	}
	if got := c.HTTPStatus(); got != 500 {
		t.Errorf("HTTPStatus() = %d, want 500", got)
	}
}
