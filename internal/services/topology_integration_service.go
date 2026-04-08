package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ---- Prometheus metrics TTL cache ----
// 避免 15 秒自動刷新對 Prometheus 發出過多查詢

type cachedMetrics[T any] struct {
	value     T
	expiresAt time.Time
}

var (
	istioCache   sync.Map // key: clusterID string → cachedMetrics[map[string]*IstioEdgeMetrics]
	hubbleCache  sync.Map // key: clusterID string → cachedMetrics[map[string]*HubbleEdgeMetrics]
	metricsTTL   = 10 * time.Second
)

// ---- DTOs ----

// TopologyIntegrationStatus 偵測叢集是否安裝 Cilium / Istio
type TopologyIntegrationStatus struct {
	Cilium        bool   `json:"cilium"`
	CiliumVersion string `json:"ciliumVersion,omitempty"`
	// HubbleMetrics 為 true 時代表 Prometheus 有 Hubble 指標可查（hubble-relay 存在不等於 Prometheus 有指標）
	HubbleMetrics bool   `json:"hubbleMetrics"`
	Istio         bool   `json:"istio"`
	IstioVersion  string `json:"istioVersion,omitempty"`
}

// IstioEdgeMetrics 單條邊的 Istio 流量指標
type IstioEdgeMetrics struct {
	SourceWorkload       string
	SourceNamespace      string
	DestService          string  // destination_service_name（Phase D：建立 Workload→Service 邊用）
	DestServiceNamespace string  // destination_service_namespace
	DestWorkload         string
	DestNamespace        string
	RequestRate          float64 // req/s (1m rate)
	ErrorRate            float64 // 0.0-1.0 (5xx / total)
	LatencyP99ms         float64 // ms (P99)
}

// ---- Detection ----

// DetectIntegrations 偵測叢集是否安裝 Cilium / Istio（輕量，單次 API 呼叫）
func DetectIntegrations(ctx context.Context, clientset kubernetes.Interface) TopologyIntegrationStatus {
	status := TopologyIntegrationStatus{}

	// Istio: istiod pod with label app=istiod in istio-system
	istioPods, err := clientset.CoreV1().Pods("istio-system").List(ctx, metav1.ListOptions{
		LabelSelector: "app=istiod",
		Limit:         1,
	})
	if err == nil && len(istioPods.Items) > 0 {
		status.Istio = true
		// Extract version from container image tag
		if len(istioPods.Items[0].Spec.Containers) > 0 {
			img := istioPods.Items[0].Spec.Containers[0].Image
			for i := len(img) - 1; i >= 0; i-- {
				if img[i] == ':' {
					status.IstioVersion = img[i+1:]
					break
				}
			}
		}
	}

	// Cilium: hubble-relay Service in kube-system
	_, err = clientset.CoreV1().Services("kube-system").Get(ctx, "hubble-relay", metav1.GetOptions{})
	if err == nil {
		status.Cilium = true
		// Try to get Cilium version from cilium DaemonSet
		ds, err := clientset.AppsV1().DaemonSets("kube-system").Get(ctx, "cilium", metav1.GetOptions{})
		if err == nil && len(ds.Spec.Template.Spec.Containers) > 0 {
			img := ds.Spec.Template.Spec.Containers[0].Image
			for i := len(img) - 1; i >= 0; i-- {
				if img[i] == ':' {
					status.CiliumVersion = img[i+1:]
					break
				}
			}
		}

		// 探測 Prometheus 是否已抓取 Hubble 指標（hubble-relay 存在 ≠ Prometheus 有指標）
		// 使用 /api/v1/query 查詢 metadata 判斷指標是否存在，避免誤顯示 Hubble Switch
		status.HubbleMetrics = probeHubblePrometheus(ctx, clientset)
	}

	return status
}

// probeHubblePrometheus 快速探測 Prometheus 是否有 Hubble 指標，回傳 true 代表可查詢
func probeHubblePrometheus(ctx context.Context, clientset kubernetes.Interface) bool {
	// 使用 instant query 取樣本數：有結果代表 Prometheus 有在抓 Hubble 指標
	probeQL := `absent(hubble_flows_processed_total)`
	resps, err := queryHubblePrometheus(ctx, clientset, probeQL)
	if err != nil {
		return false
	}
	// absent() 在指標存在時返回空 result（absent = false）
	// 若 result 為空，代表 hubble_flows_processed_total 確實存在
	for _, r := range resps {
		if len(r.Data.Result) == 0 {
			return true // absent() 沒有匹配 = 指標存在
		}
	}
	return false
}

// ---- Istio Prometheus metrics ----

// promInstantResponse Prometheus /api/v1/query 回應結構
type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Metric map[string]string `json:"metric"`
			Value  []interface{}     `json:"value"` // [timestamp, "valueStr"]
		} `json:"result"`
	} `json:"data"`
}

// queryIstioPrometheus 透過 K8s API server proxy 查詢 Istio 叢集內的 Prometheus
func queryIstioPrometheus(ctx context.Context, clientset kubernetes.Interface, promQL string) ([]promInstantResponse, error) {
	// Try common Prometheus service names in istio-system
	serviceNames := []string{"prometheus", "kube-prometheus-stack-prometheus"}
	var lastErr error
	for _, svcName := range serviceNames {
		raw, err := clientset.CoreV1().RESTClient().
			Get().
			Namespace("istio-system").
			Resource("services").
			Name(fmt.Sprintf("%s:9090", svcName)).
			SubResource("proxy").
			Suffix("/api/v1/query").
			Param("query", promQL).
			DoRaw(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		var resp promInstantResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			lastErr = err
			continue
		}
		if resp.Status != "success" {
			lastErr = fmt.Errorf("prometheus status: %s", resp.Status)
			continue
		}
		return []promInstantResponse{resp}, nil
	}
	return nil, lastErr
}

// parsePromFloat safely parses a Prometheus value string to float64
func parsePromFloat(v interface{}) float64 {
	s, ok := v.(string)
	if !ok {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// QueryIstioMetrics 查詢 Istio 的 Prometheus 取得流量指標（帶 TTL 快取，避免 15s 自動刷新重複查詢）
// Returns a map keyed by "sourceNs/sourceName→destNs/destName"
func QueryIstioMetrics(ctx context.Context, clientset kubernetes.Interface) (map[string]*IstioEdgeMetrics, error) {
	cacheKey := fmt.Sprintf("%p", clientset)
	if v, ok := istioCache.Load(cacheKey); ok {
		if entry, ok := v.(cachedMetrics[map[string]*IstioEdgeMetrics]); ok && time.Now().Before(entry.expiresAt) {
			return entry.value, nil
		}
	}
	result, err := queryIstioMetricsImpl(ctx, clientset)
	if err == nil {
		istioCache.Store(cacheKey, cachedMetrics[map[string]*IstioEdgeMetrics]{value: result, expiresAt: time.Now().Add(metricsTTL)})
	}
	return result, err
}

func queryIstioMetricsImpl(ctx context.Context, clientset kubernetes.Interface) (map[string]*IstioEdgeMetrics, error) {
	result := map[string]*IstioEdgeMetrics{}

	edgeKey := func(srcNs, src, dstNs, dst string) string {
		return srcNs + "/" + src + "→" + dstNs + "/" + dst
	}

	getOrCreate := func(m map[string]string) *IstioEdgeMetrics {
		src := m["source_workload"]
		srcNs := m["source_workload_namespace"]
		dst := m["destination_workload"]
		dstNs := m["destination_workload_namespace"]
		if src == "" || dst == "" {
			return nil
		}
		k := edgeKey(srcNs, src, dstNs, dst)
		if _, ok := result[k]; !ok {
			result[k] = &IstioEdgeMetrics{
				SourceWorkload:       src,
				SourceNamespace:      srcNs,
				DestService:          m["destination_service_name"],      // Phase D
				DestServiceNamespace: m["destination_service_namespace"], // Phase D
				DestWorkload:         dst,
				DestNamespace:        dstNs,
			}
		}
		return result[k]
	}

	// Query 1: total request rate
	// destination_service_name / destination_service_namespace 供 Phase D 建立 Workload→Service 邊
	rateQL := `sum(rate(istio_requests_total{reporter="destination"}[1m])) by (source_workload, source_workload_namespace, destination_service_name, destination_service_namespace, destination_workload, destination_workload_namespace)`
	resps, err := queryIstioPrometheus(ctx, clientset, rateQL)
	if err != nil {
		return nil, fmt.Errorf("query request rate: %w", err)
	}
	totalRates := map[string]float64{}
	for _, resp := range resps {
		for _, r := range resp.Data.Result {
			if em := getOrCreate(r.Metric); em != nil && len(r.Value) == 2 {
				rate := parsePromFloat(r.Value[1])
				em.RequestRate = rate
				k := edgeKey(r.Metric["source_workload_namespace"], r.Metric["source_workload"],
					r.Metric["destination_workload_namespace"], r.Metric["destination_workload"])
				totalRates[k] = rate
			}
		}
	}

	// Query 2: 5xx error rate
	errorQL := `sum(rate(istio_requests_total{reporter="destination",response_code=~"5.."}[1m])) by (source_workload, source_workload_namespace, destination_service_name, destination_service_namespace, destination_workload, destination_workload_namespace)`
	errResps, err := queryIstioPrometheus(ctx, clientset, errorQL)
	if err == nil {
		for _, resp := range errResps {
			for _, r := range resp.Data.Result {
				if em := getOrCreate(r.Metric); em != nil && len(r.Value) == 2 {
					errRate := parsePromFloat(r.Value[1])
					k := edgeKey(r.Metric["source_workload_namespace"], r.Metric["source_workload"],
						r.Metric["destination_workload_namespace"], r.Metric["destination_workload"])
					total := totalRates[k]
					if total > 0 {
						em.ErrorRate = errRate / total
					}
				}
			}
		}
	}

	// Query 3: P99 latency
	latencyQL := `histogram_quantile(0.99, sum(rate(istio_request_duration_milliseconds_bucket{reporter="destination"}[1m])) by (le, source_workload, source_workload_namespace, destination_service_name, destination_service_namespace, destination_workload, destination_workload_namespace))`
	latResps, err := queryIstioPrometheus(ctx, clientset, latencyQL)
	if err == nil {
		for _, resp := range latResps {
			for _, r := range resp.Data.Result {
				if em := getOrCreate(r.Metric); em != nil && len(r.Value) == 2 {
					em.LatencyP99ms = parsePromFloat(r.Value[1])
				}
			}
		}
	}

	return result, nil
}

// ---- Cilium Hubble metrics ----

// HubbleEdgeMetrics single namespace-pair Hubble flow data
type HubbleEdgeMetrics struct {
	SourceNamespace string
	DestNamespace   string
	FlowRate        float64 // forwarded flows/s
	DropRate        float64 // 0.0-1.0 (dropped / total)
	TopDropReason   string
}

// parseHubbleNs extracts namespace from "namespace/pod-name" label (or returns as-is)
func parseHubbleNs(label string) string {
	if i := strings.IndexByte(label, '/'); i >= 0 {
		return label[:i]
	}
	return label
}

// queryHubblePrometheus tries multiple Prometheus endpoints that may scrape Cilium/Hubble metrics
func queryHubblePrometheus(ctx context.Context, clientset kubernetes.Interface, promQL string) ([]promInstantResponse, error) {
	type endpoint struct{ ns, svc string }
	endpoints := []endpoint{
		{"cilium-monitoring", "prometheus"},
		{"kube-system", "prometheus"},
		{"monitoring", "prometheus"},
		{"monitoring", "kube-prometheus-stack-prometheus"},
		{"istio-system", "prometheus"},
	}
	var lastErr error
	for _, ep := range endpoints {
		raw, err := clientset.CoreV1().RESTClient().
			Get().
			Namespace(ep.ns).
			Resource("services").
			Name(fmt.Sprintf("%s:9090", ep.svc)).
			SubResource("proxy").
			Suffix("/api/v1/query").
			Param("query", promQL).
			DoRaw(ctx)
		if err != nil {
			lastErr = err
			continue
		}
		var resp promInstantResponse
		if err := json.Unmarshal(raw, &resp); err != nil {
			lastErr = err
			continue
		}
		if resp.Status != "success" || len(resp.Data.Result) == 0 {
			lastErr = fmt.Errorf("prometheus %s/%s: status=%s results=%d", ep.ns, ep.svc, resp.Status, len(resp.Data.Result))
			continue
		}
		return []promInstantResponse{resp}, nil
	}
	return nil, lastErr
}

// QueryHubbleMetrics queries Cilium Hubble flow metrics via Prometheus（帶 TTL 快取）.
// Returns a map keyed by "srcNamespace→dstNamespace".
func QueryHubbleMetrics(ctx context.Context, clientset kubernetes.Interface) (map[string]*HubbleEdgeMetrics, error) {
	cacheKey := fmt.Sprintf("hubble-%p", clientset)
	if v, ok := hubbleCache.Load(cacheKey); ok {
		if entry, ok := v.(cachedMetrics[map[string]*HubbleEdgeMetrics]); ok && time.Now().Before(entry.expiresAt) {
			return entry.value, nil
		}
	}
	result, err := queryHubbleMetricsImpl(ctx, clientset)
	if err == nil {
		hubbleCache.Store(cacheKey, cachedMetrics[map[string]*HubbleEdgeMetrics]{value: result, expiresAt: time.Now().Add(metricsTTL)})
	}
	return result, err
}

func queryHubbleMetricsImpl(ctx context.Context, clientset kubernetes.Interface) (map[string]*HubbleEdgeMetrics, error) {
	result := map[string]*HubbleEdgeMetrics{}

	hubbleKey := func(src, dst string) string { return src + "→" + dst }
	getOrCreate := func(src, dst string) *HubbleEdgeMetrics {
		k := hubbleKey(src, dst)
		if _, ok := result[k]; !ok {
			result[k] = &HubbleEdgeMetrics{SourceNamespace: src, DestNamespace: dst}
		}
		return result[k]
	}

	// Query 1: forwarded flows（不過濾 direction，避免 Cilium 版本間大小寫差異；僅按 verdict 過濾）
	fwdQL := `sum(rate(hubble_flows_processed_total{verdict="FORWARDED"}[1m])) by (source, destination)`
	fwdResps, err := queryHubblePrometheus(ctx, clientset, fwdQL)
	if err != nil {
		return nil, fmt.Errorf("query hubble forward rate: %w", err)
	}
	totalFlows := map[string]float64{}
	for _, resp := range fwdResps {
		for _, r := range resp.Data.Result {
			if len(r.Value) != 2 {
				continue
			}
			srcNs := parseHubbleNs(r.Metric["source"])
			dstNs := parseHubbleNs(r.Metric["destination"])
			if srcNs == "" || dstNs == "" {
				continue
			}
			rate := parsePromFloat(r.Value[1])
			em := getOrCreate(srcNs, dstNs)
			em.FlowRate += rate
			totalFlows[hubbleKey(srcNs, dstNs)] += rate
		}
	}

	// Query 2: dropped flows — soft failure (no Hubble drop metrics ≠ fatal error)
	dropQL := `sum(rate(hubble_flows_processed_total{verdict="DROPPED"}[1m])) by (source, destination, reason)`
	dropResps, err := queryHubblePrometheus(ctx, clientset, dropQL)
	if err == nil {
		dropByKey := map[string]float64{}
		reasonByKey := map[string]string{}
		for _, resp := range dropResps {
			for _, r := range resp.Data.Result {
				if len(r.Value) != 2 {
					continue
				}
				srcNs := parseHubbleNs(r.Metric["source"])
				dstNs := parseHubbleNs(r.Metric["destination"])
				if srcNs == "" || dstNs == "" {
					continue
				}
				k := hubbleKey(srcNs, dstNs)
				drops := parsePromFloat(r.Value[1])
				dropByKey[k] += drops
				if drops > 0 && reasonByKey[k] == "" {
					reasonByKey[k] = r.Metric["reason"]
				}
			}
		}
		for k, em := range result {
			drops := dropByKey[k]
			total := totalFlows[k] + drops
			if total > 0 {
				em.DropRate = drops / total
				em.TopDropReason = reasonByKey[k]
			}
		}
	}

	return result, nil
}
