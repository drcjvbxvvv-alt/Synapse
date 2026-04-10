package handlers

import (
	"net/http"

	"github.com/gorilla/websocket"

	"github.com/shaia/Synapse/internal/k8s"
	"github.com/shaia/Synapse/internal/middleware"
	"github.com/shaia/Synapse/internal/services"
)

// LogCenterHandler 日誌中心處理器
type LogCenterHandler struct {
	clusterSvc *services.ClusterService
	k8sMgr     *k8s.ClusterInformerManager
	aggregator *services.LogAggregator
	upgrader   websocket.Upgrader
}

// NewLogCenterHandler 建立日誌中心處理器
func NewLogCenterHandler(clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *LogCenterHandler {
	return &LogCenterHandler{
		clusterSvc: clusterSvc,
		k8sMgr:     k8sMgr,
		aggregator: services.NewLogAggregator(clusterSvc),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				return middleware.IsOriginAllowed(origin)
			},
			ReadBufferSize:  wsBufferSize,
			WriteBufferSize: wsBufferSize,
		},
	}
}
