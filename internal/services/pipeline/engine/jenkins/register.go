package jenkins

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Register wires the Jenkins adapter into a Factory.
//
// Each adapter is bound to one CIEngineConfig; the builder constructs a
// fresh Adapter per Build() call so DB updates to an engine config take
// effect on the next request without a process restart.
//
// Returns engine.ErrAlreadyRegistered (wrapped) when jenkins is already
// registered — startup code should treat that as benign.
func Register(f *engine.Factory) error {
	if f == nil {
		return fmt.Errorf("jenkins.Register: factory is nil")
	}
	return f.Register(engine.EngineJenkins, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return NewAdapter(cfg)
	})
}

// MustRegister panics if Register fails. Primarily for init() /
// main-line startup where unrecoverable wiring errors should fail loudly.
func MustRegister(f *engine.Factory) {
	if err := Register(f); err != nil {
		panic(err)
	}
}
