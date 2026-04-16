package engine

import "errors"

// Sentinel errors returned by CIEngineAdapter implementations. Handlers use
// errors.Is() to map these to HTTP status codes.
var (
	// ErrNotFound means the requested run/artifact does not exist (404).
	ErrNotFound = errors.New("engine: resource not found")

	// ErrInvalidInput means the request is malformed (400).
	ErrInvalidInput = errors.New("engine: invalid input")

	// ErrUnauthorized means the engine rejected the configured credentials (401).
	ErrUnauthorized = errors.New("engine: unauthorized")

	// ErrUnavailable means the engine is unreachable or returned 5xx (502/503).
	ErrUnavailable = errors.New("engine: engine unavailable")

	// ErrUnsupported means the adapter does not implement the requested feature
	// (mapped to 501). Use in combination with EngineCapabilities gating.
	ErrUnsupported = errors.New("engine: operation not supported by this adapter")

	// ErrAlreadyTerminal is returned by Cancel when the run is already finished.
	// This is an informational error; callers may treat it as success.
	ErrAlreadyTerminal = errors.New("engine: run already terminal")

	// ErrAlreadyRegistered is returned by Factory.Register when the given
	// engine type already has a builder. Startup code may use errors.Is to
	// treat this as benign (idempotent registration).
	ErrAlreadyRegistered = errors.New("engine: engine type already registered")
)
