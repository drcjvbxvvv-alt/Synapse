package github

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Register wires the GitHub Actions adapter into a Factory.
func Register(f *engine.Factory) error {
	if f == nil {
		return fmt.Errorf("github.Register: factory is nil")
	}
	return f.Register(engine.EngineGitHub, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return NewAdapter(cfg)
	})
}

// MustRegister panics if Register fails.
func MustRegister(f *engine.Factory) {
	if err := Register(f); err != nil {
		panic(err)
	}
}
