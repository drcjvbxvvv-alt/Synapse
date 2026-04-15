package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestInformerAgeCollector_NeverSynced(t *testing.T) {
	getAges := func() map[string]float64 {
		return map[string]float64{
			"1": -1,
			"2": -1,
		}
	}

	collector := NewInformerAgeCollector(getAges)
	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register collector: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	var found bool
	for _, f := range families {
		if f.GetName() == "synapse_informer_last_sync_age_seconds" {
			found = true
			if len(f.GetMetric()) != 2 {
				t.Errorf("expected 2 metrics, got %d", len(f.GetMetric()))
			}
			for _, m := range f.GetMetric() {
				if m.GetGauge().GetValue() != -1 {
					t.Errorf("expected -1 for never-synced cluster, got %v", m.GetGauge().GetValue())
				}
			}
		}
	}
	if !found {
		t.Error("synapse_informer_last_sync_age_seconds metric not found")
	}
}

func TestInformerAgeCollector_SyncedClusters(t *testing.T) {
	getAges := func() map[string]float64 {
		return map[string]float64{
			"1": 5.0,
			"2": 120.5,
		}
	}

	collector := NewInformerAgeCollector(getAges)
	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register collector: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	values := map[string]float64{}
	for _, f := range families {
		if f.GetName() == "synapse_informer_last_sync_age_seconds" {
			for _, m := range f.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "cluster_id" {
						values[label.GetValue()] = m.GetGauge().GetValue()
					}
				}
			}
		}
	}

	if v, ok := values["1"]; !ok || v != 5.0 {
		t.Errorf("expected cluster 1 age=5.0, got %v", v)
	}
	if v, ok := values["2"]; !ok || v != 120.5 {
		t.Errorf("expected cluster 2 age=120.5, got %v", v)
	}
}

func TestInformerAgeCollector_EmptyMap(t *testing.T) {
	getAges := func() map[string]float64 {
		return map[string]float64{}
	}

	collector := NewInformerAgeCollector(getAges)
	reg := prometheus.NewRegistry()
	if err := reg.Register(collector); err != nil {
		t.Fatalf("register collector: %v", err)
	}

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	// No clusters → no metrics emitted (the family won't appear)
	for _, f := range families {
		if f.GetName() == "synapse_informer_last_sync_age_seconds" {
			if len(f.GetMetric()) != 0 {
				t.Errorf("expected 0 metrics for empty cluster map, got %d", len(f.GetMetric()))
			}
		}
	}
}

func TestInformerAgeCollector_Describe(t *testing.T) {
	collector := NewInformerAgeCollector(func() map[string]float64 { return nil })
	ch := make(chan *prometheus.Desc, 1)
	collector.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 descriptor, got %d", count)
	}
}

func TestInformerAgeCollector_MixedSyncState(t *testing.T) {
	getAges := func() map[string]float64 {
		return map[string]float64{
			"synced":   30.0,
			"unsynced": -1,
		}
	}

	collector := NewInformerAgeCollector(getAges)
	reg := prometheus.NewRegistry()
	reg.MustRegister(collector)

	families, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}

	values := map[string]float64{}
	for _, f := range families {
		if f.GetName() == "synapse_informer_last_sync_age_seconds" {
			for _, m := range f.GetMetric() {
				for _, label := range m.GetLabel() {
					if label.GetName() == "cluster_id" {
						values[label.GetValue()] = m.GetGauge().GetValue()
					}
				}
			}
		}
	}

	if v, ok := values["synced"]; !ok || v != 30.0 {
		t.Errorf("expected synced=30.0, got %v", v)
	}
	if v, ok := values["unsynced"]; !ok || v != -1 {
		t.Errorf("expected unsynced=-1, got %v", v)
	}
}

func TestRegistryIncludesK8sMetrics(t *testing.T) {
	r := New()
	if r.K8s == nil {
		t.Fatal("expected K8s metrics in registry")
	}
	if r.K8s.ClustersActive == nil {
		t.Error("expected ClustersActive gauge")
	}
	if r.K8s.InformerSynced == nil {
		t.Error("expected InformerSynced gauge vec")
	}
}
