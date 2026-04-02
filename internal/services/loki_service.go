package services

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/models"
)

// LokiService queries a Loki instance via its HTTP API.
type LokiService struct {
	source *models.LogSourceConfig
	client *http.Client
}

// NewLokiService creates a LokiService for the given source config.
func NewLokiService(src *models.LogSourceConfig) *LokiService {
	return &LokiService{
		source: src,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// lokiQueryRangeResp mirrors the Loki /query_range response envelope.
type lokiQueryRangeResp struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][2]string       `json:"values"` // [nanosecond timestamp, log line]
		} `json:"result"`
	} `json:"data"`
}

// QueryRange executes a LogQL range query against Loki and returns log entries.
func (s *LokiService) QueryRange(query string, startTime, endTime time.Time, limit int) ([]models.LogEntry, error) {
	if limit <= 0 || limit > 5000 {
		limit = 500
	}

	params := url.Values{}
	params.Set("query", query)
	params.Set("start", strconv.FormatInt(startTime.UnixNano(), 10))
	params.Set("end", strconv.FormatInt(endTime.UnixNano(), 10))
	params.Set("limit", strconv.Itoa(limit))
	params.Set("direction", "backward")

	reqURL := strings.TrimRight(s.source.URL, "/") + "/loki/api/v1/query_range?" + params.Encode()

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("loki: build request: %w", err)
	}

	if s.source.Username != "" {
		req.SetBasicAuth(s.source.Username, s.source.Password)
	}
	if s.source.APIKey != "" {
		req.Header.Set("X-Scope-OrgID", s.source.APIKey)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("loki: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("loki: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loki: status %d: %s", resp.StatusCode, string(body))
	}

	var lokiResp lokiQueryRangeResp
	if err := json.Unmarshal(body, &lokiResp); err != nil {
		return nil, fmt.Errorf("loki: parse response: %w", err)
	}

	var entries []models.LogEntry
	for _, stream := range lokiResp.Data.Result {
		ns := stream.Stream["namespace"]
		pod := stream.Stream["pod"]
		container := stream.Stream["container"]
		node := stream.Stream["node"]

		for _, val := range stream.Values {
			nsNano, _ := strconv.ParseInt(val[0], 10, 64)
			ts := time.Unix(0, nsNano)

			entries = append(entries, models.LogEntry{
				ID:          fmt.Sprintf("loki-%d", nsNano),
				Timestamp:   ts,
				Type:        "loki",
				Level:       detectLevel(val[1]),
				ClusterID:   s.source.ClusterID,
				ClusterName: s.source.Name,
				Namespace:   ns,
				PodName:     pod,
				Container:   container,
				NodeName:    node,
				Message:     val[1],
				Labels:      stream.Stream,
			})
		}
	}
	return entries, nil
}

// detectLevel guesses log level from the message text.
func detectLevel(msg string) string {
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "err "):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "debug"):
		return "debug"
	default:
		return "info"
	}
}
