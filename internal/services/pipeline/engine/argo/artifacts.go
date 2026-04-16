package argo

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
)

// GetArtifacts walks status.nodes[].outputs.artifacts[] and emits one
// engine.Artifact per named artifact.
//
// Argo artifacts can live in several backends: S3, GCS, Azure Blob, OSS,
// Git, or an HTTP endpoint. The inline metadata only tells us the artifact
// **name** and its **destination reference** (backend-specific URL or key).
// This adapter exposes the name + a best-effort URL hint; the UI can
// deep-link to the Argo UI for signed downloads.
//
// Returns an empty slice (not nil, not error) when no artifacts are
// present — matching the contract across all adapters.
func (a *Adapter) GetArtifacts(ctx context.Context, runID string) ([]*engine.Artifact, error) {
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("argo.GetArtifacts: empty run id: %w", engine.ErrInvalidInput)
	}
	if err := requireResolver(a.resolver); err != nil {
		return nil, err
	}
	ns, err := a.extra.requireNamespace()
	if err != nil {
		return nil, err
	}
	dyn, err := a.resolver.Dynamic(a.clusterID)
	if err != nil {
		return nil, fmt.Errorf("argo.GetArtifacts: dynamic client: %w", engine.ErrUnavailable)
	}
	wf, err := dyn.Resource(gvrWorkflow).Namespace(ns).Get(ctx, runID, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("argo.GetArtifacts %s: %w", runID, mapK8sError(err))
	}

	// Use workflow-level finishedAt when available; otherwise startedAt; else now.
	var createdAt time.Time
	_, finished := readArgoTimes(wf.Object)
	if finished != nil {
		createdAt = *finished
	} else if started, _ := readArgoTimes(wf.Object); started != nil {
		createdAt = *started
	} else {
		createdAt = time.Now().UTC()
	}

	artifacts := extractArtifactsFromNodes(wf.Object)
	if len(artifacts) == 0 {
		return []*engine.Artifact{}, nil
	}
	out := make([]*engine.Artifact, 0, len(artifacts))
	for _, a := range artifacts {
		out = append(out, &engine.Artifact{
			Name:      a.Name,
			Kind:      "file",
			URL:       a.URL, // best-effort: the backend key/URL
			CreatedAt: createdAt,
		})
	}
	return out, nil
}

// artifactInfo captures the fields the adapter needs from an Argo artifact
// entry. Argo may also carry `s3.key`, `gcs.key`, etc.; we represent the
// destination uniformly in URL.
type artifactInfo struct {
	Name string
	URL  string
}

// extractArtifactsFromNodes walks every entry in status.nodes and flattens
// the `outputs.artifacts[]` lists. Duplicate names across different nodes
// are preserved (the node name is not part of our output, so callers see
// them as repeats — acceptable for M18e).
func extractArtifactsFromNodes(obj map[string]any) []artifactInfo {
	status, _ := obj["status"].(map[string]any)
	if status == nil {
		return nil
	}
	nodes, _ := status["nodes"].(map[string]any)
	if len(nodes) == 0 {
		return nil
	}
	var out []artifactInfo
	for _, raw := range nodes {
		node, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		outputs, _ := node["outputs"].(map[string]any)
		if outputs == nil {
			continue
		}
		arts, _ := outputs["artifacts"].([]any)
		for _, a := range arts {
			am, ok := a.(map[string]any)
			if !ok {
				continue
			}
			name, _ := am["name"].(string)
			if name == "" {
				continue
			}
			out = append(out, artifactInfo{
				Name: name,
				URL:  extractArtifactURL(am),
			})
		}
	}
	return out
}

// extractArtifactURL returns the first backend-specific URL/key we can
// surface. Priority: http → s3 → gcs → oss → raw. If none of those
// destination blocks is present, we return empty string — the adapter
// still emits the artifact (with URL="") so the UI can display it.
func extractArtifactURL(art map[string]any) string {
	if http, _ := art["http"].(map[string]any); http != nil {
		if u, _ := http["url"].(string); u != "" {
			return u
		}
	}
	for _, backend := range []string{"s3", "gcs", "oss", "azure"} {
		if b, _ := art[backend].(map[string]any); b != nil {
			if k, _ := b["key"].(string); k != "" {
				return backend + "://" + k
			}
		}
	}
	if raw, _ := art["raw"].(map[string]any); raw != nil {
		// Inline raw content is unusual for file artifacts; surface as
		// an opaque marker rather than the full data.
		return "raw://"
	}
	return ""
}
