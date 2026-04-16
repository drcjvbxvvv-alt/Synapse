package engine

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/shaia/Synapse/internal/models"
)

// newStubBuilder returns an AdapterBuilder that yields stubAdapter instances
// stamped with the given engine type. Used by factory tests.
func newStubBuilder(t EngineType) AdapterBuilder {
	return func(cfg *models.CIEngineConfig) (CIEngineAdapter, error) {
		return &stubAdapter{typ: t, avail: true}, nil
	}
}

func TestFactory_Register_And_Build(t *testing.T) {
	f := NewFactory()
	if err := f.Register(EngineNative, newStubBuilder(EngineNative)); err != nil {
		t.Fatalf("Register native: %v", err)
	}
	if err := f.Register(EngineGitLab, newStubBuilder(EngineGitLab)); err != nil {
		t.Fatalf("Register gitlab: %v", err)
	}

	// Build gitlab via config.
	cfg := &models.CIEngineConfig{EngineType: "gitlab"}
	a, err := f.Build(cfg)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	if a.Type() != EngineGitLab {
		t.Fatalf("Type = %q, want gitlab", a.Type())
	}
}

func TestFactory_Register_NilCfgForBuild(t *testing.T) {
	f := NewFactory()
	if _, err := f.Build(nil); err == nil {
		t.Fatalf("expected error for nil cfg")
	} else if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFactory_Register_EmptyType(t *testing.T) {
	f := NewFactory()
	if err := f.Register(EngineType(""), newStubBuilder("x")); err == nil {
		t.Fatalf("expected error for empty engine type")
	}
}

func TestFactory_Register_NilBuilder(t *testing.T) {
	f := NewFactory()
	if err := f.Register(EngineNative, nil); err == nil {
		t.Fatalf("expected error for nil builder")
	}
}

func TestFactory_Register_Duplicate(t *testing.T) {
	f := NewFactory()
	_ = f.Register(EngineNative, newStubBuilder(EngineNative))
	if err := f.Register(EngineNative, newStubBuilder(EngineNative)); err == nil {
		t.Fatalf("expected duplicate registration error")
	}
}

func TestFactory_Build_UnknownType(t *testing.T) {
	f := NewFactory()
	cfg := &models.CIEngineConfig{EngineType: "circleci"}
	_, err := f.Build(cfg)
	if err == nil {
		t.Fatalf("expected error for unknown type")
	}
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFactory_Build_UnregisteredKnownType(t *testing.T) {
	f := NewFactory()
	cfg := &models.CIEngineConfig{EngineType: "gitlab"} // valid type but not registered
	_, err := f.Build(cfg)
	if err == nil {
		t.Fatalf("expected ErrUnsupported")
	}
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestFactory_Build_BuilderError(t *testing.T) {
	f := NewFactory()
	want := errors.New("builder-specific failure")
	_ = f.Register(EngineGitLab, func(*models.CIEngineConfig) (CIEngineAdapter, error) {
		return nil, want
	})
	cfg := &models.CIEngineConfig{EngineType: "gitlab"}
	_, err := f.Build(cfg)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !errors.Is(err, want) {
		t.Fatalf("expected builder error to wrap %v, got %v", want, err)
	}
}

func TestFactory_BuildNative(t *testing.T) {
	f := NewFactory()
	_ = f.Register(EngineNative, newStubBuilder(EngineNative))
	a, err := f.BuildNative()
	if err != nil {
		t.Fatalf("BuildNative: %v", err)
	}
	if a.Type() != EngineNative {
		t.Fatalf("type = %q, want native", a.Type())
	}
}

func TestFactory_BuildNative_NotRegistered(t *testing.T) {
	f := NewFactory()
	_, err := f.BuildNative()
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("expected ErrUnsupported, got %v", err)
	}
}

func TestFactory_Unregister(t *testing.T) {
	f := NewFactory()
	_ = f.Register(EngineNative, newStubBuilder(EngineNative))
	if !f.Unregister(EngineNative) {
		t.Fatalf("Unregister returned false")
	}
	if f.Unregister(EngineNative) {
		t.Fatalf("second Unregister should return false")
	}
	if f.IsRegistered(EngineNative) {
		t.Fatalf("IsRegistered should be false after Unregister")
	}
}

func TestFactory_Registered_Sorted(t *testing.T) {
	f := NewFactory()
	_ = f.Register(EngineGitLab, newStubBuilder(EngineGitLab))
	_ = f.Register(EngineNative, newStubBuilder(EngineNative))
	_ = f.Register(EngineTekton, newStubBuilder(EngineTekton))

	got := f.Registered()
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	// Must be sorted alphabetically by string value.
	want := []EngineType{EngineGitLab, EngineNative, EngineTekton}
	for i, v := range want {
		if got[i] != v {
			t.Fatalf("Registered()[%d] = %q, want %q", i, got[i], v)
		}
	}
}

func TestFactory_Concurrent_RegisterAndBuild(t *testing.T) {
	// The Factory is documented as concurrency-safe; race-detect tests run
	// via `go test -race` will flag any missing lock coverage.
	f := NewFactory()
	_ = f.Register(EngineNative, newStubBuilder(EngineNative))

	const workers = 20
	var wg sync.WaitGroup
	wg.Add(workers * 2)

	// half readers, half writers (Register/Unregister on non-native types)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			typ := EngineType(fmt.Sprintf("t-%d", i%5+10)) // "t-10" .. "t-14"
			_ = f.Register(typ, func(*models.CIEngineConfig) (CIEngineAdapter, error) {
				return &stubAdapter{typ: typ}, nil
			})
			_ = f.Unregister(typ)
		}()
		go func() {
			defer wg.Done()
			cfg := &models.CIEngineConfig{EngineType: "native"}
			_, _ = f.Build(cfg)
		}()
	}
	wg.Wait()
}

func TestDefaultFactory_ResetForTest(t *testing.T) {
	// Exercise the Default() singleton + reset helper so that downstream
	// packages can rely on it.
	def := Default()
	_ = def.Register(EngineNative, newStubBuilder(EngineNative))
	if !def.IsRegistered(EngineNative) {
		t.Fatal("Default did not register")
	}
	ResetDefaultForTest()
	def2 := Default()
	if def2.IsRegistered(EngineNative) {
		t.Fatal("ResetDefaultForTest did not clear Default")
	}
}

// ensure stubAdapter still satisfies the interface (guard against refactors
// that might drop a method accidentally)
var _ CIEngineAdapter = (*stubAdapter)(nil)

// Dummy uses of types to silence unused import warnings in some toolchains.
var (
	_ = io.EOF
	_ = context.Background
	_ = strings.NewReader
)
