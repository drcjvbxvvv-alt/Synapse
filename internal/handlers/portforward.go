package handlers

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/clay-wangzhi/Synapse/internal/k8s"
	"github.com/clay-wangzhi/Synapse/internal/models"
	"github.com/clay-wangzhi/Synapse/internal/response"
	"github.com/clay-wangzhi/Synapse/internal/services"
	"github.com/clay-wangzhi/Synapse/pkg/logger"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
)

// portForwardStop 儲存活躍 Port-Forward 的停止 channel
var (
	pfMu      sync.Mutex
	pfStopMap = make(map[uint]chan struct{}) // sessionID → stopChan
)

// PortForwardHandler Port-Forward 處理器
type PortForwardHandler struct {
	db         *gorm.DB
	clusterSvc *services.ClusterService
	k8sMgr     *k8s.ClusterInformerManager
}

func NewPortForwardHandler(db *gorm.DB, clusterSvc *services.ClusterService, k8sMgr *k8s.ClusterInformerManager) *PortForwardHandler {
	return &PortForwardHandler{db: db, clusterSvc: clusterSvc, k8sMgr: k8sMgr}
}

// pickFreePort 分配一個空閒的本地埠（12000-13000 範圍）
func pickFreePort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return port, nil
}

// StartPortForward 建立 Port-Forward 會話
// POST /api/v1/clusters/:clusterID/pods/:namespace/:name/portforward
func (h *PortForwardHandler) StartPortForward(c *gin.Context) {
	clusterIDStr := c.Param("clusterID")
	namespace := c.Param("namespace")
	podName := c.Param("name")

	clusterID, err := parseClusterID(clusterIDStr)
	if err != nil {
		response.BadRequest(c, "無效的叢集 ID")
		return
	}

	var req struct {
		PodPort int `json:"podPort" binding:"required,min=1,max=65535"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "請求參數錯誤: "+err.Error())
		return
	}

	cluster, err := h.clusterSvc.GetCluster(clusterID)
	if err != nil {
		response.NotFound(c, "叢集不存在")
		return
	}

	k8sClient, err := h.k8sMgr.GetK8sClient(cluster)
	if err != nil {
		response.InternalError(c, "取得 K8s 客戶端失敗: "+err.Error())
		return
	}

	localPort, err := pickFreePort()
	if err != nil {
		response.InternalError(c, "分配本地埠失敗: "+err.Error())
		return
	}

	restConfig := k8sClient.GetRestConfig()
	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		response.InternalError(c, "建立 SPDY transport 失敗: "+err.Error())
		return
	}

	pfReq := k8sClient.GetClientset().CoreV1().RESTClient().Post().
		Resource("pods").
		Namespace(namespace).
		Name(podName).
		SubResource("portforward")

	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", pfReq.URL())

	stopCh := make(chan struct{})
	readyCh := make(chan struct{})
	errBuf := &bytes.Buffer{}

	fw, err := portforward.New(
		dialer,
		[]string{fmt.Sprintf("%d:%d", localPort, req.PodPort)},
		stopCh, readyCh,
		errBuf, errBuf,
	)
	if err != nil {
		response.InternalError(c, "建立 Port-Forward 失敗: "+err.Error())
		return
	}

	// 在背景啟動 Port-Forward
	pfErrCh := make(chan error, 1)
	go func() {
		pfErrCh <- fw.ForwardPorts()
	}()

	// 等待就緒或失敗（最多 10 秒）
	select {
	case <-readyCh:
		// Port-Forward 就緒
	case err := <-pfErrCh:
		response.InternalError(c, "Port-Forward 啟動失敗: "+err.Error())
		return
	case <-time.After(10 * time.Second):
		close(stopCh)
		response.InternalError(c, "Port-Forward 啟動逾時")
		return
	}

	// 儲存會話
	userID := c.GetUint("user_id")
	username, _ := c.Get("username")
	session := &models.PortForwardSession{
		ClusterID:   clusterID,
		ClusterName: cluster.Name,
		Namespace:   namespace,
		PodName:     podName,
		PodPort:     req.PodPort,
		LocalPort:   localPort,
		UserID:      userID,
		Username:    username.(string),
		Status:      "active",
	}
	if err := h.db.Create(session).Error; err != nil {
		close(stopCh)
		response.InternalError(c, "儲存會話失敗: "+err.Error())
		return
	}

	pfMu.Lock()
	pfStopMap[session.ID] = stopCh
	pfMu.Unlock()

	// 背景監控：若 Port-Forward 結束則更新狀態
	go func() {
		select {
		case <-pfErrCh:
		case <-stopCh:
		}
		now := time.Now()
		h.db.Model(session).Updates(map[string]interface{}{"status": "stopped", "stopped_at": &now})
		pfMu.Lock()
		delete(pfStopMap, session.ID)
		pfMu.Unlock()
		logger.Info("Port-Forward 已結束", "sessionID", session.ID, "pod", podName)
	}()

	logger.Info("Port-Forward 啟動", "pod", podName, "podPort", req.PodPort, "localPort", localPort)
	response.OK(c, gin.H{
		"sessionId": session.ID,
		"localPort": localPort,
		"podPort":   req.PodPort,
		"message":   fmt.Sprintf("Port-Forward 已啟動：後端埠 %d → Pod %s:%d", localPort, podName, req.PodPort),
	})
}

// StopPortForward 停止 Port-Forward 會話
// DELETE /api/v1/portforwards/:sessionId
func (h *PortForwardHandler) StopPortForward(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("sessionId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "無效的 session ID")
		return
	}

	pfMu.Lock()
	stopCh, ok := pfStopMap[uint(id)]
	if ok {
		close(stopCh)
	}
	pfMu.Unlock()

	now := time.Now()
	h.db.Model(&models.PortForwardSession{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": "stopped", "stopped_at": &now,
	})

	response.OK(c, gin.H{"message": "Port-Forward 已停止"})
}

// ListPortForwards 列出活躍 Port-Forward 會話
// GET /api/v1/portforwards?status=active
func (h *PortForwardHandler) ListPortForwards(c *gin.Context) {
	userID := c.GetUint("user_id")
	status := c.DefaultQuery("status", "active")

	var sessions []models.PortForwardSession
	db := h.db.Where("user_id = ?", userID)
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if err := db.Order("created_at desc").Find(&sessions).Error; err != nil {
		response.InternalError(c, "查詢失敗: "+err.Error())
		return
	}
	response.OK(c, gin.H{"items": sessions, "total": len(sessions)})
}
