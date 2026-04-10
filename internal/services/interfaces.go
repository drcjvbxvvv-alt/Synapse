package services

// This file defines narrow interfaces for the three largest services.
// Handlers hold these interfaces instead of concrete structs, enabling
// mock injection in tests without touching service implementations.
//
// Each interface contains only the methods actually called by handlers.
// Service-to-service calls may continue using concrete types.

import (
	"context"
	"encoding/json"

	"github.com/shaia/Synapse/internal/models"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

// ─── PrometheusQuerier ────────────────────────────────────────────────────────

// PrometheusQuerier is the interface MonitoringHandler and NodeHandler depend on.
// *PrometheusService satisfies this interface automatically.
type PrometheusQuerier interface {
	TestConnection(ctx context.Context, config *models.MonitoringConfig) error
	QueryClusterMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, timeRange, step string) (*models.ClusterMetricsData, error)
	QueryNodeMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, nodeName, timeRange, step string) (*models.ClusterMetricsData, error)
	QueryPodMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, namespace, podName, timeRange, step string) (*models.ClusterMetricsData, error)
	QueryWorkloadMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName, namespace, workloadName, timeRange, step string) (*models.ClusterMetricsData, error)
	QueryNodeListMetrics(ctx context.Context, config *models.MonitoringConfig, clusterName string) ([]models.NodeMetricItem, error)
}

// ─── OMQuerier ────────────────────────────────────────────────────────────────

// OMQuerier is the interface OMHandler depends on.
// *OMService satisfies this interface automatically.
type OMQuerier interface {
	GetHealthDiagnosis(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.HealthDiagnosisResponse, error)
	GetResourceTop(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint, req *models.ResourceTopRequest) (*models.ResourceTopResponse, error)
	GetControlPlaneStatus(ctx context.Context, clientset *kubernetes.Clientset, clusterID uint) (*models.ControlPlaneStatusResponse, error)
}

// ─── MeshQuerier ──────────────────────────────────────────────────────────────

// ─── Compile-time interface satisfaction checks ───────────────────────────────

// These blank assignments cause a compile error if a concrete type ever stops
// satisfying its interface, providing an early warning before tests run.
var (
	_ PrometheusQuerier = (*PrometheusService)(nil)
	_ OMQuerier         = (*OMService)(nil)
	_ MeshQuerier       = (*MeshService)(nil)
)

// ─── MeshQuerier ──────────────────────────────────────────────────────────────

// MeshQuerier is the interface MeshHandler depends on.
// *MeshService satisfies this interface automatically.
type MeshQuerier interface {
	GetStatus(ctx context.Context, clientset kubernetes.Interface) MeshStatus
	GetTopology(ctx context.Context, clientset kubernetes.Interface, clusterID uint, namespace string) (*MeshTopology, error)
	ListVirtualServices(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]map[string]interface{}, error)
	GetVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error)
	CreateVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace string, body json.RawMessage) (*unstructured.Unstructured, error)
	UpdateVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string, body json.RawMessage) (*unstructured.Unstructured, error)
	DeleteVirtualService(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error
	ListDestinationRules(ctx context.Context, dynClient dynamic.Interface, namespace string) ([]map[string]interface{}, error)
	GetDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string) (*unstructured.Unstructured, error)
	CreateDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace string, body json.RawMessage) (*unstructured.Unstructured, error)
	UpdateDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string, body json.RawMessage) (*unstructured.Unstructured, error)
	DeleteDestinationRule(ctx context.Context, dynClient dynamic.Interface, namespace, name string) error
}
