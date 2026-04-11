package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shaia/Synapse/pkg/logger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ── GVRs ─────────────────────────────────────────────────────────────────────

var (
	chaosMeshGroup   = "chaos-mesh.org"
	chaosMeshVersion = "v1alpha1"

	podChaosGVR     = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "podchaos"}
	networkChaosGVR = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "networkchaos"}
	stressChaosGVR  = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "stresschaos"}
	httpChaosGVR    = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "httpchaos"}
	ioChaosGVR      = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "iochaos"}
	scheduleGVR     = schema.GroupVersionResource{Group: chaosMeshGroup, Version: chaosMeshVersion, Resource: "schedules"}
)

// allExperimentGVRs lists every experiment CRD to query when listing.
var allExperimentGVRs = []schema.GroupVersionResource{
	podChaosGVR, networkChaosGVR, stressChaosGVR, httpChaosGVR, ioChaosGVR,
}

// ── DTOs — all at package level (Brain pitfall: never inside functions) ──────

// ChaosStatus reports whether Chaos Mesh is installed in this cluster.
type ChaosStatus struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
}

// ChaosExperiment is a normalised view of a Chaos Mesh experiment object.
type ChaosExperiment struct {
	UID       string `json:"uid"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Phase     string `json:"phase"`    // Injecting | Waiting | Finished | Failed
	Duration  string `json:"duration"` // e.g. "1m"
	CreatedAt string `json:"created_at"`
}

// ChaosSchedule is a normalised Schedule CRD view.
type ChaosSchedule struct {
	Name        string `json:"name"`
	Namespace   string `json:"namespace"`
	CronExpr    string `json:"cron_expr"`
	Type        string `json:"type"` // PodChaos | NetworkChaos | ...
	Suspended   bool   `json:"suspended"`
	LastRunTime string `json:"last_run_time,omitempty"`
}

// PodChaosSpec holds PodChaos-specific options.
type PodChaosSpec struct {
	Action        string `json:"action"`         // pod-kill | pod-failure | container-kill
	ContainerName string `json:"container_name"` // for container-kill
}

// NetworkDelaySpec holds network delay options.
type NetworkDelaySpec struct {
	Latency     string `json:"latency"`     // e.g. "100ms"
	Jitter      string `json:"jitter"`      // e.g. "10ms"
	Correlation string `json:"correlation"` // 0-100 percentage string
}

// NetworkLossSpec holds network packet loss options.
type NetworkLossSpec struct {
	Loss        string `json:"loss"`
	Correlation string `json:"correlation"`
}

// NetworkChaosSpec holds NetworkChaos-specific options.
type NetworkChaosSpec struct {
	Action    string           `json:"action"`              // delay | loss | corrupt | partition
	Direction string           `json:"direction,omitempty"` // to | from | both
	Delay     NetworkDelaySpec `json:"delay,omitempty"`
	Loss      NetworkLossSpec  `json:"loss,omitempty"`
}

// StressCPUSpec holds CPU stressor options.
type StressCPUSpec struct {
	Workers int `json:"workers"` // goroutine count
	Load    int `json:"load"`    // 0-100 %
}

// StressMemSpec holds memory stressor options.
type StressMemSpec struct {
	Workers int    `json:"workers"`
	Size    string `json:"size"` // e.g. "256MB"
}

// StressChaosSpec holds StressChaos-specific options.
type StressChaosSpec struct {
	CPU    StressCPUSpec `json:"cpu,omitempty"`
	Memory StressMemSpec `json:"memory,omitempty"`
}

// TargetSelector defines which pods to target.
type TargetSelector struct {
	Namespace      string            `json:"namespace"`
	LabelSelectors map[string]string `json:"label_selectors,omitempty"`
	PodPhase       string            `json:"pod_phase,omitempty"` // Running | ...
}

// CreateScheduleRequest is the body for POST /chaos/schedules.
type CreateScheduleRequest struct {
	Name         string          `json:"name"      binding:"required,max=253"`
	Namespace    string          `json:"namespace" binding:"required"`
	CronExpr     string          `json:"cron_expr" binding:"required"` // e.g. "@every 1h" or "0 * * * *"
	Kind         string          `json:"kind"      binding:"required,oneof=PodChaos NetworkChaos StressChaos"`
	// Inner experiment spec — reuse CreateChaosRequest fields
	Duration     string          `json:"duration"`
	Target       TargetSelector  `json:"target"    binding:"required"`
	PodChaos     PodChaosSpec    `json:"pod_chaos"`
	NetworkChaos NetworkChaosSpec `json:"network_chaos"`
	StressChaos  StressChaosSpec `json:"stress_chaos"`
}

// CreateChaosRequest is the body for POST /chaos/experiments.
type CreateChaosRequest struct {
	Name         string          `json:"name"      binding:"required,max=253"`
	Namespace    string          `json:"namespace" binding:"required"`
	Kind         string          `json:"kind"      binding:"required,oneof=PodChaos NetworkChaos StressChaos"`
	Duration     string          `json:"duration"`              // e.g. "1m", "10m"; empty = indefinite
	Target       TargetSelector  `json:"target"    binding:"required"`
	PodChaos     PodChaosSpec    `json:"pod_chaos"`
	NetworkChaos NetworkChaosSpec `json:"network_chaos"`
	StressChaos  StressChaosSpec `json:"stress_chaos"`
}

// ── Service ──────────────────────────────────────────────────────────────────

// ChaosService provides detection and CRUD for Chaos Mesh experiments.
type ChaosService struct{}

// NewChaosService constructs a ChaosService.
func NewChaosService() *ChaosService { return &ChaosService{} }

// ─ Detection ─────────────────────────────────────────────────────────────────

// IsChaosMeshInstalled checks for the chaos-mesh.org/v1alpha1 API group.
// Gracefully returns false on any discovery error.
func (s *ChaosService) IsChaosMeshInstalled(ctx context.Context, clientset kubernetes.Interface) ChaosStatus {
	_, err := clientset.Discovery().ServerResourcesForGroupVersion(
		fmt.Sprintf("%s/%s", chaosMeshGroup, chaosMeshVersion),
	)
	if err != nil {
		return ChaosStatus{Installed: false}
	}
	return ChaosStatus{Installed: true, Version: chaosMeshVersion}
}

// ─ List ──────────────────────────────────────────────────────────────────────

// ListExperiments concurrently fetches experiments from all known CRD types.
func (s *ChaosService) ListExperiments(ctx context.Context, dyn dynamic.Interface, namespace string) ([]ChaosExperiment, error) {
	type result struct {
		items []ChaosExperiment
		err   error
	}

	ch := make(chan result, len(allExperimentGVRs))
	var wg sync.WaitGroup
	for _, gvr := range allExperimentGVRs {
		wg.Add(1)
		go func(gvr schema.GroupVersionResource) {
			defer wg.Done()
			items, err := s.listByGVR(ctx, dyn, gvr, namespace)
			ch <- result{items: items, err: err}
		}(gvr)
	}
	wg.Wait()
	close(ch)

	var all []ChaosExperiment
	for r := range ch {
		if r.err != nil {
			logger.Warn("list chaos experiments partial error", "error", r.err)
			continue // CRD absent — skip
		}
		all = append(all, r.items...)
	}
	return all, nil
}

func (s *ChaosService) listByGVR(ctx context.Context, dyn dynamic.Interface, gvr schema.GroupVersionResource, namespace string) ([]ChaosExperiment, error) {
	var ri dynamic.ResourceInterface
	if namespace == "" {
		ri = dyn.Resource(gvr)
	} else {
		ri = dyn.Resource(gvr).Namespace(namespace)
	}

	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", gvr.Resource, err)
	}

	kind := gvrToKind(gvr)
	items := make([]ChaosExperiment, 0, len(list.Items))
	for _, obj := range list.Items {
		items = append(items, toExperiment(obj, kind))
	}
	return items, nil
}

// ─ Single get ────────────────────────────────────────────────────────────────

// GetExperiment fetches a single experiment by kind/namespace/name.
func (s *ChaosService) GetExperiment(ctx context.Context, dyn dynamic.Interface, kind, namespace, name string) (*unstructured.Unstructured, error) {
	gvr, err := kindToGVR(kind)
	if err != nil {
		return nil, err
	}
	obj, err := dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get %s %s/%s: %w", kind, namespace, name, err)
	}
	return obj, nil
}

// ─ Create ────────────────────────────────────────────────────────────────────

// CreateExperiment builds and applies the Chaos Mesh CRD object.
func (s *ChaosService) CreateExperiment(ctx context.Context, dyn dynamic.Interface, req CreateChaosRequest) (*ChaosExperiment, error) {
	gvr, err := kindToGVR(req.Kind)
	if err != nil {
		return nil, err
	}

	obj, err := buildChaosObject(req)
	if err != nil {
		return nil, fmt.Errorf("build chaos object: %w", err)
	}

	created, err := dyn.Resource(gvr).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", req.Kind, err)
	}

	exp := toExperiment(*created, req.Kind)
	logger.Info("chaos experiment created", "kind", req.Kind, "namespace", req.Namespace, "name", req.Name)
	return &exp, nil
}

// ─ Delete ────────────────────────────────────────────────────────────────────

// DeleteExperiment removes a Chaos experiment by kind/namespace/name.
func (s *ChaosService) DeleteExperiment(ctx context.Context, dyn dynamic.Interface, kind, namespace, name string) error {
	gvr, err := kindToGVR(kind)
	if err != nil {
		return err
	}
	if err := dyn.Resource(gvr).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete %s %s/%s: %w", kind, namespace, name, err)
	}
	logger.Info("chaos experiment deleted", "kind", kind, "namespace", namespace, "name", name)
	return nil
}

// ─ Schedules ─────────────────────────────────────────────────────────────────

// ListSchedules lists Chaos Mesh Schedule CRDs.
func (s *ChaosService) ListSchedules(ctx context.Context, dyn dynamic.Interface, namespace string) ([]ChaosSchedule, error) {
	var ri dynamic.ResourceInterface
	if namespace == "" {
		ri = dyn.Resource(scheduleGVR)
	} else {
		ri = dyn.Resource(scheduleGVR).Namespace(namespace)
	}
	list, err := ri.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list schedules: %w", err)
	}

	out := make([]ChaosSchedule, 0, len(list.Items))
	for _, obj := range list.Items {
		out = append(out, toSchedule(obj))
	}
	return out, nil
}

// CreateSchedule creates a Chaos Mesh Schedule CRD.
func (s *ChaosService) CreateSchedule(ctx context.Context, dyn dynamic.Interface, req CreateScheduleRequest) (*ChaosSchedule, error) {
	// Build the inner experiment spec using the experiment builder.
	innerReq := CreateChaosRequest{
		Name:         req.Name + "-inner",
		Namespace:    req.Namespace,
		Kind:         req.Kind,
		Duration:     req.Duration,
		Target:       req.Target,
		PodChaos:     req.PodChaos,
		NetworkChaos: req.NetworkChaos,
		StressChaos:  req.StressChaos,
	}
	innerSpec, err := buildChaosSpec(innerReq)
	if err != nil {
		return nil, fmt.Errorf("build schedule inner spec: %w", err)
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", chaosMeshGroup, chaosMeshVersion),
			"kind":       "Schedule",
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": req.Namespace,
			},
			"spec": map[string]interface{}{
				"schedule": req.CronExpr,
				"type":     req.Kind,
				req.Kind:   innerSpec,
			},
		},
	}

	created, err := dyn.Resource(scheduleGVR).Namespace(req.Namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create schedule: %w", err)
	}

	sched := toSchedule(*created)
	logger.Info("chaos schedule created", "namespace", req.Namespace, "name", req.Name, "cron", req.CronExpr)
	return &sched, nil
}

// DeleteSchedule removes a Chaos Mesh Schedule CRD.
func (s *ChaosService) DeleteSchedule(ctx context.Context, dyn dynamic.Interface, namespace, name string) error {
	if err := dyn.Resource(scheduleGVR).Namespace(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("delete schedule %s/%s: %w", namespace, name, err)
	}
	logger.Info("chaos schedule deleted", "namespace", namespace, "name", name)
	return nil
}

// HasActiveExperiments returns true if any experiment in the namespace is currently injecting.
func (s *ChaosService) HasActiveExperiments(ctx context.Context, dyn dynamic.Interface, namespace string) bool {
	exps, err := s.ListExperiments(ctx, dyn, namespace)
	if err != nil {
		return false // Chaos Mesh not installed or transient error — don't block SLO
	}
	for _, e := range exps {
		if e.Phase == "Running" || e.Phase == "Injecting" {
			return true
		}
	}
	return false
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func gvrToKind(gvr schema.GroupVersionResource) string {
	switch gvr.Resource {
	case "podchaos":
		return "PodChaos"
	case "networkchaos":
		return "NetworkChaos"
	case "stresschaos":
		return "StressChaos"
	case "httpchaos":
		return "HTTPChaos"
	case "iochaos":
		return "IOChaos"
	default:
		return gvr.Resource
	}
}

func kindToGVR(kind string) (schema.GroupVersionResource, error) {
	switch kind {
	case "PodChaos":
		return podChaosGVR, nil
	case "NetworkChaos":
		return networkChaosGVR, nil
	case "StressChaos":
		return stressChaosGVR, nil
	case "HTTPChaos":
		return httpChaosGVR, nil
	case "IOChaos":
		return ioChaosGVR, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unknown chaos kind: %s", kind)
	}
}

func toExperiment(obj unstructured.Unstructured, kind string) ChaosExperiment {
	phase, _, _ := unstructured.NestedString(obj.Object, "status", "experiment", "desiredPhase")
	if phase == "" {
		phase, _, _ = unstructured.NestedString(obj.Object, "status", "phase")
	}
	dur, _, _ := unstructured.NestedString(obj.Object, "spec", "duration")

	createdAt := ""
	if !obj.GetCreationTimestamp().Time.IsZero() {
		createdAt = obj.GetCreationTimestamp().UTC().Format(time.RFC3339)
	}

	return ChaosExperiment{
		UID:       string(obj.GetUID()),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      kind,
		Phase:     phase,
		Duration:  dur,
		CreatedAt: createdAt,
	}
}

func toSchedule(obj unstructured.Unstructured) ChaosSchedule {
	cron, _, _ := unstructured.NestedString(obj.Object, "spec", "schedule")
	scheduleType, _, _ := unstructured.NestedString(obj.Object, "spec", "type")
	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	lastRun, _, _ := unstructured.NestedString(obj.Object, "status", "lastScheduleTime")

	return ChaosSchedule{
		Name:        obj.GetName(),
		Namespace:   obj.GetNamespace(),
		CronExpr:    cron,
		Type:        scheduleType,
		Suspended:   suspended,
		LastRunTime: lastRun,
	}
}

// buildChaosObject constructs the unstructured Chaos Mesh CRD from a CreateChaosRequest.
func buildChaosObject(req CreateChaosRequest) (*unstructured.Unstructured, error) {
	spec, err := buildChaosSpec(req)
	if err != nil {
		return nil, err
	}
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": fmt.Sprintf("%s/%s", chaosMeshGroup, chaosMeshVersion),
			"kind":       req.Kind,
			"metadata": map[string]interface{}{
				"name":      req.Name,
				"namespace": req.Namespace,
			},
			"spec": spec,
		},
	}
	return obj, nil
}

// buildChaosSpec builds the spec map shared by experiment objects and Schedule inner specs.
func buildChaosSpec(req CreateChaosRequest) (map[string]interface{}, error) {
	selector := map[string]interface{}{
		"namespaces": []interface{}{req.Target.Namespace},
	}
	if len(req.Target.LabelSelectors) > 0 {
		ls := make(map[string]interface{}, len(req.Target.LabelSelectors))
		for k, v := range req.Target.LabelSelectors {
			ls[k] = v
		}
		selector["labelSelectors"] = ls
	}

	spec := map[string]interface{}{
		"selector": selector,
		"mode":     "one",
	}
	if req.Duration != "" {
		spec["duration"] = req.Duration
	}

	switch req.Kind {
	case "PodChaos":
		action := req.PodChaos.Action
		if action == "" {
			action = "pod-kill"
		}
		spec["action"] = action
		if req.PodChaos.ContainerName != "" {
			spec["containerNames"] = []interface{}{req.PodChaos.ContainerName}
		}

	case "NetworkChaos":
		action := req.NetworkChaos.Action
		if action == "" {
			action = "delay"
		}
		spec["action"] = action
		dir := req.NetworkChaos.Direction
		if dir == "" {
			dir = "to"
		}
		spec["direction"] = dir
		switch action {
		case "delay":
			d := req.NetworkChaos.Delay
			delay := map[string]interface{}{"latency": d.Latency}
			if d.Jitter != "" {
				delay["jitter"] = d.Jitter
			}
			if d.Correlation != "" {
				delay["correlation"] = d.Correlation
			}
			spec["delay"] = delay
		case "loss":
			l := req.NetworkChaos.Loss
			loss := map[string]interface{}{"loss": l.Loss}
			if l.Correlation != "" {
				loss["correlation"] = l.Correlation
			}
			spec["loss"] = loss
		}

	case "StressChaos":
		var stressors []interface{}
		cpu := req.StressChaos.CPU
		if cpu.Workers > 0 {
			stressors = append(stressors, map[string]interface{}{
				"cpuStressor": map[string]interface{}{
					"workers": cpu.Workers,
					"load":    cpu.Load,
				},
			})
		}
		mem := req.StressChaos.Memory
		if mem.Workers > 0 {
			stressors = append(stressors, map[string]interface{}{
				"memoryStressor": map[string]interface{}{
					"workers": mem.Workers,
					"size":    mem.Size,
				},
			})
		}
		if len(stressors) == 0 {
			return nil, fmt.Errorf("StressChaos requires at least one stressor (cpu or memory)")
		}
		spec["stressors"] = stressors

	default:
		return nil, fmt.Errorf("unsupported chaos kind: %s", req.Kind)
	}

	return spec, nil
}
