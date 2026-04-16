package engine

import (
	"fmt"
	"sort"
	"sync"

	"github.com/shaia/Synapse/internal/models"
)

// AdapterBuilder constructs an adapter from a connection config. Implementations
// live in their own files (native.go, gitlab.go, …) and register via Register().
//
// For the built-in Native engine, cfg may be nil.
type AdapterBuilder func(cfg *models.CIEngineConfig) (CIEngineAdapter, error)

// Factory is a concurrency-safe registry of engine type → builder mappings.
// A zero Factory{} is NOT ready for use; callers must use NewFactory().
type Factory struct {
	mu       sync.RWMutex
	builders map[EngineType]AdapterBuilder
}

// NewFactory returns a Factory with no adapters registered. Adapters opt in
// by calling Register().
func NewFactory() *Factory {
	return &Factory{
		builders: make(map[EngineType]AdapterBuilder),
	}
}

// Register associates a builder with an engine type. Returns an error if the
// engine type is empty or already registered so accidental double-registration
// surfaces immediately (common bug pattern when wiring init() functions).
func (f *Factory) Register(t EngineType, b AdapterBuilder) error {
	if t == "" {
		return fmt.Errorf("engine.Register: engine type must not be empty")
	}
	if b == nil {
		return fmt.Errorf("engine.Register: builder for %q must not be nil", t)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.builders[t]; exists {
		return fmt.Errorf("engine.Register: engine type %q: %w", t, ErrAlreadyRegistered)
	}
	f.builders[t] = b
	return nil
}

// MustRegister is like Register but panics on error. Intended for package
// init(); panicking here is correct because a programming bug has prevented
// the factory from reaching a valid state.
func (f *Factory) MustRegister(t EngineType, b AdapterBuilder) {
	if err := f.Register(t, b); err != nil {
		panic(err)
	}
}

// Unregister removes an engine type. Returns false when the type was not
// registered. Primarily intended for tests.
func (f *Factory) Unregister(t EngineType) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, exists := f.builders[t]; !exists {
		return false
	}
	delete(f.builders, t)
	return true
}

// Build resolves the registered builder for cfg.EngineType and invokes it.
// Returns ErrInvalidInput when cfg is nil or the engine type is unknown.
//
// For EngineNative, callers may pass a nil cfg (the native builder handles
// the nil case); for every other engine a non-nil cfg is required.
func (f *Factory) Build(cfg *models.CIEngineConfig) (CIEngineAdapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("engine.Build: cfg is nil: %w", ErrInvalidInput)
	}
	t := EngineType(cfg.EngineType)
	if !t.IsValid() {
		return nil, fmt.Errorf("engine.Build: unknown engine type %q: %w", cfg.EngineType, ErrInvalidInput)
	}
	f.mu.RLock()
	builder, ok := f.builders[t]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("engine.Build: no adapter registered for engine type %q: %w", t, ErrUnsupported)
	}
	adapter, err := builder(cfg)
	if err != nil {
		return nil, fmt.Errorf("build adapter for %q: %w", t, err)
	}
	return adapter, nil
}

// BuildNative is a convenience shortcut for the built-in engine. Returns
// ErrUnsupported if no native adapter has been registered yet.
func (f *Factory) BuildNative() (CIEngineAdapter, error) {
	f.mu.RLock()
	builder, ok := f.builders[EngineNative]
	f.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("engine.BuildNative: native adapter not registered: %w", ErrUnsupported)
	}
	adapter, err := builder(nil)
	if err != nil {
		return nil, fmt.Errorf("build native adapter: %w", err)
	}
	return adapter, nil
}

// Registered returns the sorted list of currently registered engine types.
// Used by admin / diagnostics endpoints.
func (f *Factory) Registered() []EngineType {
	f.mu.RLock()
	defer f.mu.RUnlock()
	out := make([]EngineType, 0, len(f.builders))
	for t := range f.builders {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// IsRegistered reports whether a builder is registered for t.
func (f *Factory) IsRegistered(t EngineType) bool {
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, ok := f.builders[t]
	return ok
}

// ---------------------------------------------------------------------------
// Default factory — package-level singleton for convenience.
// ---------------------------------------------------------------------------
//
// Most code paths consume the Default factory; tests that need isolation
// should construct a local *Factory instead of mutating Default.

var defaultFactory = NewFactory()

// Default returns the package-level Factory singleton.
func Default() *Factory { return defaultFactory }

// ResetDefaultForTest clears the Default factory. DO NOT call this outside of
// tests; it is exported only for _test.go files in downstream packages that
// need a clean slate before registering their own mocks.
func ResetDefaultForTest() {
	defaultFactory = NewFactory()
}
