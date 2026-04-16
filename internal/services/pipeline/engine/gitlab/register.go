package gitlab

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Register wires the GitLab adapter into a Factory.
//
// Unlike the Native engine (which has a single process-wide instance),
// each GitLab adapter is bound to one CIEngineConfig — the builder creates
// a fresh Adapter per Build() call so that changes to an engine config are
// picked up on the next request without requiring a process restart.
//
// Returns an error if the factory already has a builder registered for
// engine.EngineGitLab; callers in production should call this exactly once
// during service startup.
func Register(f *engine.Factory) error {
	if f == nil {
		return fmt.Errorf("gitlab.Register: factory is nil")
	}
	builder := func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return NewAdapter(cfg)
	}
	return f.Register(engine.EngineGitLab, builder)
}

// MustRegister is the panic-on-error companion to Register; useful from
// package init() or main-line startup where a failure to register a required
// adapter is unrecoverable.
func MustRegister(f *engine.Factory) {
	if err := Register(f); err != nil {
		panic(err)
	}
}
