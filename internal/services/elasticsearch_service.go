package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
)

// ElasticsearchService queries an Elasticsearch cluster via its REST API.
type ElasticsearchService struct {
	source *models.LogSourceConfig
	client *http.Client
}

// NewElasticsearchService creates an ElasticsearchService.
func NewElasticsearchService(src *models.LogSourceConfig) *ElasticsearchService {
	return &ElasticsearchService{
		source: src,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// esSearchResp mirrors the _search response envelope.
type esSearchResp struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []struct {
			Source map[string]interface{} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// Search queries Elasticsearch with a Lucene keyword query over a time range.
// index: the ES index pattern (e.g. "k8s-logs-*"); pass "" to use "_all".
func (s *ElasticsearchService) Search(index, query string, startTime, endTime time.Time, limit int) ([]models.LogEntry, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}
	if index == "" {
		index = "_all"
	}

	// Build ES query DSL
	dsl := map[string]interface{}{
		"size": limit,
		"sort": []map[string]interface{}{
			{"@timestamp": map[string]string{"order": "desc"}},
		},
		"query": map[string]interface{}{
			"bool": map[string]interface{}{
				"must": buildESMust(query, startTime, endTime),
			},
		},
	}

	body, err := json.Marshal(dsl)
	if err != nil {
		return nil, fmt.Errorf("es: marshal query: %w", err)
	}

	reqURL := strings.TrimRight(s.source.URL, "/") + "/" + index + "/_search"
	req, err := http.NewRequest(http.MethodPost, reqURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("es: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if s.source.Username != "" {
		req.SetBasicAuth(s.source.Username, s.source.Password)
	}
	if s.source.APIKey != "" {
		req.Header.Set("Authorization", "ApiKey "+s.source.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("es: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("es: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("es: status %d: %s", resp.StatusCode, string(respBody))
	}

	var esResp esSearchResp
	if err := json.Unmarshal(respBody, &esResp); err != nil {
		return nil, fmt.Errorf("es: parse response: %w", err)
	}

	var entries []models.LogEntry
	for _, hit := range esResp.Hits.Hits {
		entry := hitToLogEntry(hit.Source, s.source)
		entries = append(entries, entry)
	}
	return entries, nil
}

func buildESMust(keyword string, start, end time.Time) []interface{} {
	must := []interface{}{
		map[string]interface{}{
			"range": map[string]interface{}{
				"@timestamp": map[string]interface{}{
					"gte": start.UTC().Format(time.RFC3339),
					"lte": end.UTC().Format(time.RFC3339),
				},
			},
		},
	}
	if keyword != "" {
		must = append(must, map[string]interface{}{
			"query_string": map[string]interface{}{
				"query": keyword,
				"fields": []string{
					"message", "log.message", "msg",
					"kubernetes.namespace_name", "kubernetes.pod_name",
				},
			},
		})
	}
	return must
}

func hitToLogEntry(src map[string]interface{}, source *models.LogSourceConfig) models.LogEntry {
	getString := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := src[k]; ok {
				if s, ok := v.(string); ok {
					return s
				}
			}
		}
		return ""
	}

	tsStr := getString("@timestamp", "timestamp", "time")
	ts, _ := time.Parse(time.RFC3339, tsStr)

	message := getString("message", "log.message", "msg", "log")

	return models.LogEntry{
		ID:          getString("_id"),
		Timestamp:   ts,
		Type:        "elasticsearch",
		Level:       detectLevel(message),
		ClusterID:   source.ClusterID,
		ClusterName: source.Name,
		Namespace:   getString("kubernetes.namespace_name", "namespace"),
		PodName:     getString("kubernetes.pod_name", "pod"),
		Container:   getString("kubernetes.container_name", "container"),
		NodeName:    getString("kubernetes.host", "host", "node"),
		Message:     message,
	}
}
