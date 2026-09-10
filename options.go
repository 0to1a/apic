package apic

import (
	"io"
	"log"
)

// Encoder writes a success or error envelope. The default is
// EncodeSuccess/EncodeStatus; pass a custom one via WithEncoder to swap the
// JSON library or response shape.
type Encoder struct {
	Success func(w io.Writer, data any) error
	Error   func(w io.Writer, st *Status) error
}

// Logger receives the original error before it's collapsed into a generic
// Internal message, so unexpected errors are never fully swallowed.
type Logger interface {
	Printf(format string, args ...any)
}

// Config holds the values Options mutate. Generated RegisterRoutes code
// builds one from opts and uses its Encoder/Logger.
type Config struct {
	Encoder Encoder
	Logger  Logger
}

// Option customizes RegisterRoutes.
type Option func(*Config)

// WithEncoder overrides the envelope encoder.
func WithEncoder(enc Encoder) Option {
	return func(c *Config) { c.Encoder = enc }
}

// WithLogger overrides where internal errors are logged.
func WithLogger(l Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// NewConfig applies opts on top of the defaults.
func NewConfig(opts ...Option) *Config {
	c := &Config{
		Encoder: Encoder{Success: EncodeSuccess, Error: EncodeStatus},
		Logger:  log.Default(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}
