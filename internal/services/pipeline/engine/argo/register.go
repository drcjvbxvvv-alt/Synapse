package argo

import (
	"fmt"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// Register wires the Argo Workflows adapter into a Factory.
func Register(f *engine.Factory, resolver ClusterResolver) error {
	if f == nil {
		return fmt.Errorf("argo.Register: factory is nil")
	}
	if resolver == nil {
		return fmt.Errorf("argo.Register: cluster resolver is required")
	}
	return f.Register(engine.EngineArgo, func(cfg *models.CIEngineConfig) (engine.CIEngineAdapter, error) {
		return NewAdapter(cfg, resolver)
	})
}

// MustRegister panics if Register fails.
func MustRegister(f *engine.Factory, resolver ClusterResolver) {
	if err := Register(f, resolver); err != nil {
		panic(err)
	}
}
