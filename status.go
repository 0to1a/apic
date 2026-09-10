package apic

import "fmt"

// Status is the error type service methods return. RegisterRoutes maps it to
// an envelope and HTTP status via Code.HTTPStatus(); any other error becomes
// Internal (see ResolveError).
type Status struct {
	Code    Code
	Message string
	Details map[string]any
}

func (s *Status) Error() string {
	return fmt.Sprintf("%s: %s", s.Code, s.Message)
}

// Error builds a *Status with the given code and message.
func Error(code Code, message string) *Status {
	return &Status{Code: code, Message: message}
}

// Errorf builds a *Status with a formatted message.
func Errorf(code Code, format string, args ...any) *Status {
	return &Status{Code: code, Message: fmt.Sprintf(format, args...)}
}

// WithDetails attaches structured details and returns s for chaining.
func (s *Status) WithDetails(details map[string]any) *Status {
	s.Details = details
	return s
}

// ResolveError converts any error into a *Status: a *Status is returned
// as-is, anything else becomes Internal with a generic message. The original
// error is not included in the result — callers should log it themselves
// before calling ResolveError.
func ResolveError(err error) *Status {
	if st, ok := err.(*Status); ok {
		return st
	}
	return &Status{Code: Internal, Message: "internal error"}
}
