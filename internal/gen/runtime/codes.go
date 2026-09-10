package runtime

// Code is an error code. Numbering matches google.golang.org/grpc/codes so
// the envelope's "code" field lines up with gRPC status codes without
// pulling in the grpc-go module as a runtime dependency.
type Code int

const (
	OK Code = iota
	Canceled
	Unknown
	InvalidArgument
	DeadlineExceeded
	NotFound
	AlreadyExists
	PermissionDenied
	ResourceExhausted
	FailedPrecondition
	Aborted
	OutOfRange
	Unimplemented
	Internal
	Unavailable
	DataLoss
	Unauthenticated
)

var codeNames = map[Code]string{
	OK:                 "OK",
	Canceled:           "CANCELED",
	Unknown:            "UNKNOWN",
	InvalidArgument:    "INVALID_ARGUMENT",
	DeadlineExceeded:   "DEADLINE_EXCEEDED",
	NotFound:           "NOT_FOUND",
	AlreadyExists:      "ALREADY_EXISTS",
	PermissionDenied:   "PERMISSION_DENIED",
	ResourceExhausted:  "RESOURCE_EXHAUSTED",
	FailedPrecondition: "FAILED_PRECONDITION",
	Aborted:            "ABORTED",
	OutOfRange:         "OUT_OF_RANGE",
	Unimplemented:      "UNIMPLEMENTED",
	Internal:           "INTERNAL",
	Unavailable:        "UNAVAILABLE",
	DataLoss:           "DATA_LOSS",
	Unauthenticated:    "UNAUTHENTICATED",
}

// String returns the gRPC-style name of the code, e.g. "NOT_FOUND".
func (c Code) String() string {
	if name, ok := codeNames[c]; ok {
		return name
	}
	return "UNKNOWN"
}

// HTTPStatus maps a Code to the HTTP status grpc-gateway uses for it.
func (c Code) HTTPStatus() int {
	switch c {
	case OK:
		return 200
	case Canceled:
		return 499
	case InvalidArgument, FailedPrecondition, OutOfRange:
		return 400
	case DeadlineExceeded:
		return 504
	case NotFound:
		return 404
	case AlreadyExists, Aborted:
		return 409
	case PermissionDenied:
		return 403
	case Unauthenticated:
		return 401
	case ResourceExhausted:
		return 429
	case Unimplemented:
		return 501
	case Unavailable:
		return 503
	case DataLoss, Internal, Unknown:
		return 500
	default:
		return 500
	}
}
