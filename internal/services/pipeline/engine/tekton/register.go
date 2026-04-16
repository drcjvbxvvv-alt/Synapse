package tekton

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Register wires the Tekton adapter into a Factory.
//
// Unlike the GitLab and Jenkins adapters, Tekton needs a cluster resolver
// at registration time — each Adapter instance built from a CIEngineConfig
// will use it to obtain dynamic/discovery clients against the target
// Synapse-managed cluster.
//
// Returns engine.ErrAlreadyRegistered (wrapped) when the factory already
// has a builder for engine.EngineTekton. The caller (router startup) may
// treat this as benign.
func Register(f *engine.Factory, resolver ClusterResolver) error {
	if f == nil {
		return fmt.Errorf("tekton.Register: factory is nil")
	}
	if resolver == nil {
		return fmt.Errorf("tekton.Register: cluster resolver is required")
	}
	return f.Register(engine.EngineTekton, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return NewAdapter(cfg, resolver)
	})
}

// MustRegister panics when Register fails; intended for startup wiring
// where unrecoverable misconfiguration should fail loudly.
func MustRegister(f *engine.Factory, resolver ClusterResolver) {
	if err := Register(f, resolver); err != nil {
		panic(err)
	}
}
