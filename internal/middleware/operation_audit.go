package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"regexp"
	"strconv"
	"strings"
	"time"

	"kubepolaris/internal/constants"
	"kubepolaris/internal/services"
	"kubepolaris/pkg/logger"

	"github.com/gin-gonic/gin"
)

// routeRule 路由规则
type routeRule struct {
	Pattern      *regexp.Regexp
	Module       string
	Action       string
	ResourceType string
	// 动态提取资源名的参数索引
	ResourceNameIndex int
}

// 预编译的路由规则
var routeRules []routeRule

func init() {
	// 初始化路由规则
	rules := []struct {
		Pattern           string
		Module            string
		Action            string
		ResourceType      string
		ResourceNameIndex int // -1 表示不提取
	}{
		// 认证模块
		{`^/api/v1/auth/login$`, constants.ModuleAuth, constants.ActionLogin, "user", -1},
		{`^/api/v1/auth/logout$`, constants.ModuleAuth, constants.ActionLogout, "user", -1},
		{`^/api/v1/auth/change-password$`, constants.ModuleAuth, constants.ActionChangePassword, "user", -1},

		// 集群模块
		{`^/api/v1/clusters/import$`, constants.ModuleCluster, constants.ActionImport, "cluster", -1},
		{`^/api/v1/clusters/test-connection$`, constants.ModuleCluster, constants.ActionTest, "cluster", -1},
		{`^/api/v1/clusters/(\d+)$`, constants.ModuleCluster, "", "cluster", 1},

		// 节点模块
		{`^/api/v1/clusters/\d+/nodes/([^/]+)/cordon$`, constants.ModuleNode, constants.ActionCordon, "node", 1},
		{`^/api/v1/clusters/\d+/nodes/([^/]+)/uncordon$`, constants.ModuleNode, constants.ActionUncordon, "node", 1},
		{`^/api/v1/clusters/\d+/nodes/([^/]+)/drain$`, constants.ModuleNode, constants.ActionDrain, "node", 1},

		// Pod 模块
		{`^/api/v1/clusters/\d+/pods/([^/]+)/([^/]+)$`, constants.ModulePod, "", "pod", 2},

		// Deployment 模块
		{`^/api/v1/clusters/\d+/deployments/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "deployment", -1},
		{`^/api/v1/clusters/\d+/deployments/([^/]+)/([^/]+)/scale$`, constants.ModuleWorkload, constants.ActionScale, "deployment", 2},
		{`^/api/v1/clusters/\d+/deployments/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "deployment", 2},

		// Rollout 模块
		{`^/api/v1/clusters/\d+/rollouts/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "rollout", -1},
		{`^/api/v1/clusters/\d+/rollouts/([^/]+)/([^/]+)/scale$`, constants.ModuleWorkload, constants.ActionScale, "rollout", 2},
		{`^/api/v1/clusters/\d+/rollouts/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "rollout", 2},

		// StatefulSet 模块
		{`^/api/v1/clusters/\d+/statefulsets/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "statefulset", -1},
		{`^/api/v1/clusters/\d+/statefulsets/([^/]+)/([^/]+)/scale$`, constants.ModuleWorkload, constants.ActionScale, "statefulset", 2},
		{`^/api/v1/clusters/\d+/statefulsets/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "statefulset", 2},

		// DaemonSet 模块
		{`^/api/v1/clusters/\d+/daemonsets/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "daemonset", -1},
		{`^/api/v1/clusters/\d+/daemonsets/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "daemonset", 2},

		// Job 模块
		{`^/api/v1/clusters/\d+/jobs/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "job", -1},
		{`^/api/v1/clusters/\d+/jobs/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "job", 2},

		// CronJob 模块
		{`^/api/v1/clusters/\d+/cronjobs/yaml/apply$`, constants.ModuleWorkload, constants.ActionApply, "cronjob", -1},
		{`^/api/v1/clusters/\d+/cronjobs/([^/]+)/([^/]+)$`, constants.ModuleWorkload, "", "cronjob", 2},

		// ConfigMap 模块
		{`^/api/v1/clusters/\d+/configmaps$`, constants.ModuleConfig, constants.ActionCreate, "configmap", -1},
		{`^/api/v1/clusters/\d+/configmaps/([^/]+)/([^/]+)$`, constants.ModuleConfig, "", "configmap", 2},

		// Secret 模块
		{`^/api/v1/clusters/\d+/secrets$`, constants.ModuleConfig, constants.ActionCreate, "secret", -1},
		{`^/api/v1/clusters/\d+/secrets/([^/]+)/([^/]+)$`, constants.ModuleConfig, "", "secret", 2},

		// Service 模块
		{`^/api/v1/clusters/\d+/services$`, constants.ModuleNetwork, constants.ActionCreate, "service", -1},
		{`^/api/v1/clusters/\d+/services/([^/]+)/([^/]+)$`, constants.ModuleNetwork, "", "service", 2},

		// Ingress 模块
		{`^/api/v1/clusters/\d+/ingresses$`, constants.ModuleNetwork, constants.ActionCreate, "ingress", -1},
		{`^/api/v1/clusters/\d+/ingresses/([^/]+)/([^/]+)$`, constants.ModuleNetwork, "", "ingress", 2},

		// Namespace 模块
		{`^/api/v1/clusters/\d+/namespaces$`, constants.ModuleNamespace, constants.ActionCreate, "namespace", -1},
		{`^/api/v1/clusters/\d+/namespaces/([^/]+)$`, constants.ModuleNamespace, "", "namespace", 1},

		// 存储模块
		{`^/api/v1/clusters/\d+/pvcs/([^/]+)/([^/]+)$`, constants.ModuleStorage, "", "pvc", 2},
		{`^/api/v1/clusters/\d+/pvs/([^/]+)$`, constants.ModuleStorage, "", "pv", 1},
		{`^/api/v1/clusters/\d+/storageclasses/([^/]+)$`, constants.ModuleStorage, "", "storageclass", 1},

		// 监控配置模块
		{`^/api/v1/clusters/\d+/monitoring/config$`, constants.ModuleMonitoring, "", "monitoring_config", -1},
		{`^/api/v1/clusters/\d+/monitoring/test-connection$`, constants.ModuleMonitoring, constants.ActionTest, "monitoring_config", -1},

		// AlertManager 模块
		{`^/api/v1/clusters/\d+/alertmanager/config$`, constants.ModuleAlert, "", "alertmanager_config", -1},
		{`^/api/v1/clusters/\d+/alertmanager/test-connection$`, constants.ModuleAlert, constants.ActionTest, "alertmanager_config", -1},
		{`^/api/v1/clusters/\d+/silences$`, constants.ModuleAlert, constants.ActionCreate, "silence", -1},
		{`^/api/v1/clusters/\d+/silences/([^/]+)$`, constants.ModuleAlert, constants.ActionDelete, "silence", 1},

		// ArgoCD 模块
		{`^/api/v1/clusters/\d+/argocd/config$`, constants.ModuleArgoCD, "", "argocd_config", -1},
		{`^/api/v1/clusters/\d+/argocd/test-connection$`, constants.ModuleArgoCD, constants.ActionTest, "argocd_config", -1},
		{`^/api/v1/clusters/\d+/argocd/applications$`, constants.ModuleArgoCD, constants.ActionCreate, "application", -1},
		{`^/api/v1/clusters/\d+/argocd/applications/([^/]+)$`, constants.ModuleArgoCD, "", "application", 1},
		{`^/api/v1/clusters/\d+/argocd/applications/([^/]+)/sync$`, constants.ModuleArgoCD, constants.ActionSync, "application", 1},
		{`^/api/v1/clusters/\d+/argocd/applications/([^/]+)/rollback$`, constants.ModuleArgoCD, constants.ActionRollback, "application", 1},

		// 权限模块
		{`^/api/v1/permissions/user-groups$`, constants.ModulePermission, constants.ActionCreate, "user_group", -1},
		{`^/api/v1/permissions/user-groups/(\d+)$`, constants.ModulePermission, "", "user_group", 1},
		{`^/api/v1/permissions/user-groups/(\d+)/users$`, constants.ModulePermission, constants.ActionUpdate, "user_group_member", 1},
		{`^/api/v1/permissions/user-groups/(\d+)/users/(\d+)$`, constants.ModulePermission, constants.ActionDelete, "user_group_member", 2},
		{`^/api/v1/permissions/cluster-permissions$`, constants.ModulePermission, constants.ActionCreate, "cluster_permission", -1},
		{`^/api/v1/permissions/cluster-permissions/(\d+)$`, constants.ModulePermission, "", "cluster_permission", 1},
		{`^/api/v1/permissions/cluster-permissions/batch-delete$`, constants.ModulePermission, constants.ActionDelete, "cluster_permission", -1},

		// 系统设置模块
		{`^/api/v1/system/ldap/config$`, constants.ModuleSystem, "", "ldap_config", -1},
		{`^/api/v1/system/ldap/test-connection$`, constants.ModuleSystem, constants.ActionTest, "ldap_config", -1},
		{`^/api/v1/system/ldap/test-auth$`, constants.ModuleSystem, constants.ActionTest, "ldap_auth", -1},
		{`^/api/v1/system/ssh/config$`, constants.ModuleSystem, "", "ssh_config", -1},
	}

	for _, r := range rules {
		routeRules = append(routeRules, routeRule{
			Pattern:           regexp.MustCompile(r.Pattern),
			Module:            r.Module,
			Action:            r.Action,
			ResourceType:      r.ResourceType,
			ResourceNameIndex: r.ResourceNameIndex,
		})
	}
}

// OperationAudit 操作审计中间件
func OperationAudit(logSvc *services.OperationLogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 只记录非 GET 请求
		if c.Request.Method == "GET" {
			c.Next()
			return
		}

		// 跳过健康检查等
		path := c.Request.URL.Path
		if strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") {
			c.Next()
			return
		}

		// 跳过 WebSocket 请求（由终端审计单独处理）
		if strings.HasPrefix(path, "/ws/") {
			c.Next()
			return
		}

		startTime := time.Now()

		// 读取并缓存请求体
		var requestBody interface{}
		if c.Request.Body != nil && c.Request.ContentLength > 0 {
			bodyBytes, err := io.ReadAll(c.Request.Body)
			if err == nil && len(bodyBytes) > 0 {
				c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				json.Unmarshal(bodyBytes, &requestBody)
			}
		}

		// 执行请求
		c.Next()

		// 解析路由信息
		module, action, resourceType, resourceName := parseRoute(c, path)

		// 根据 HTTP 方法确定默认操作
		if action == "" {
			action = methodToAction(c.Request.Method)
		}

		// 获取集群信息
		var clusterID *uint
		clusterName := ""
		if cid := c.Param("clusterID"); cid != "" {
			if id, err := strconv.ParseUint(cid, 10, 32); err == nil {
				uid := uint(id)
				clusterID = &uid
			}
		}
		// 尝试从 context 获取集群名称（如果 handler 设置了）
		if cn, exists := c.Get("cluster_name"); exists {
			clusterName = cn.(string)
		}

		// 获取用户信息
		var userID *uint
		username := ""
		if uid := c.GetUint("user_id"); uid > 0 {
			userID = &uid
		}
		if un := c.GetString("username"); un != "" {
			username = un
		}

		// 获取错误信息
		errorMessage := ""
		if err, exists := c.Get("error_message"); exists {
			if errStr, ok := err.(string); ok {
				errorMessage = errStr
			}
		}

		// 从 namespace 参数获取命名空间
		namespace := c.Param("namespace")
		if namespace == "" {
			namespace = c.Param("ns")
		}

		// 如果资源名还是空的，尝试从常见参数获取
		if resourceName == "" {
			if name := c.Param("name"); name != "" {
				resourceName = name
			} else if appName := c.Param("appName"); appName != "" {
				resourceName = appName
			}
		}

		// 构建日志条目
		entry := &services.LogEntry{
			UserID:       userID,
			Username:     username,
			Method:       c.Request.Method,
			Path:         path,
			Query:        c.Request.URL.RawQuery,
			Module:       module,
			Action:       action,
			ClusterID:    clusterID,
			ClusterName:  clusterName,
			Namespace:    namespace,
			ResourceType: resourceType,
			ResourceName: resourceName,
			RequestBody:  requestBody,
			StatusCode:   c.Writer.Status(),
			Success:      c.Writer.Status() < 400,
			ErrorMessage: errorMessage,
			ClientIP:     c.ClientIP(),
			UserAgent:    c.Request.UserAgent(),
			Duration:     time.Since(startTime).Milliseconds(),
		}

		// 异步记录
		logSvc.RecordAsync(entry)

		logger.Debug("操作审计记录",
			"module", module,
			"action", action,
			"path", path,
			"user", username,
			"success", entry.Success)
	}
}

// parseRoute 从路由解析操作信息
func parseRoute(c *gin.Context, path string) (module, action, resourceType, resourceName string) {
	for _, rule := range routeRules {
		matches := rule.Pattern.FindStringSubmatch(path)
		if matches != nil {
			module = rule.Module
			action = rule.Action
			resourceType = rule.ResourceType

			// 提取资源名
			if rule.ResourceNameIndex > 0 && rule.ResourceNameIndex < len(matches) {
				resourceName = matches[rule.ResourceNameIndex]
			}

			return
		}
	}

	// 未匹配到规则，尝试从路径推断
	module = constants.ModuleUnknown
	action = methodToAction(c.Request.Method)
	resourceType = guessResourceType(path)
	resourceName = c.Param("name")

	return
}

// methodToAction 根据 HTTP 方法返回操作
func methodToAction(method string) string {
	switch method {
	case "POST":
		return constants.ActionCreate
	case "PUT", "PATCH":
		return constants.ActionUpdate
	case "DELETE":
		return constants.ActionDelete
	default:
		return strings.ToLower(method)
	}
}

// guessResourceType 从路径猜测资源类型
func guessResourceType(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	// 尝试找到有意义的资源类型
	resourceTypes := []string{
		"clusters", "nodes", "pods", "deployments", "statefulsets",
		"daemonsets", "jobs", "cronjobs", "configmaps", "secrets",
		"services", "ingresses", "namespaces", "pvcs", "pvs",
		"storageclasses", "rollouts", "applications",
	}

	for _, part := range parts {
		for _, rt := range resourceTypes {
			if part == rt {
				// 移除复数s
				if strings.HasSuffix(rt, "es") {
					return strings.TrimSuffix(rt, "es")
				}
				if strings.HasSuffix(rt, "s") {
					return strings.TrimSuffix(rt, "s")
				}
				return rt
			}
		}
	}

	return "unknown"
}
