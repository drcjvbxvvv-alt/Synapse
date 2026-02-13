package services

import (
	"bufio"
	"context"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/clay-wangzhi/KubePolaris/internal/models"
	"github.com/clay-wangzhi/KubePolaris/pkg/logger"

	"github.com/google/uuid"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogAggregator 日志聚合器
type LogAggregator struct {
	clusterSvc *ClusterService
}

// NewLogAggregator 创建日志聚合器
func NewLogAggregator(clusterSvc *ClusterService) *LogAggregator {
	return &LogAggregator{
		clusterSvc: clusterSvc,
	}
}

// AggregateStream 聚合多个Pod的日志流
func (a *LogAggregator) AggregateStream(
	ctx context.Context,
	cluster *models.Cluster,
	targets []models.LogStreamTarget,
	opts *models.LogStreamOptions,
) (<-chan *models.LogEntry, error) {
	outputCh := make(chan *models.LogEntry, 1000)
	var wg sync.WaitGroup

	// 为每个目标启动日志流
	for _, target := range targets {
		wg.Add(1)
		go func(t models.LogStreamTarget) {
			defer wg.Done()
			a.streamPodLogs(ctx, cluster, t, opts, outputCh)
		}(target)
	}

	// 等待所有流结束后关闭输出通道
	go func() {
		wg.Wait()
		close(outputCh)
	}()

	return outputCh, nil
}

// streamPodLogs 单个Pod日志流
func (a *LogAggregator) streamPodLogs(
	ctx context.Context,
	cluster *models.Cluster,
	target models.LogStreamTarget,
	opts *models.LogStreamOptions,
	outputCh chan<- *models.LogEntry,
) {
	// 创建K8s客户端
	k8sClient, err := NewK8sClientForCluster(cluster)
	if err != nil {
		logger.Error("创建K8s客户端失败", "error", err)
		return
	}

	// 构建日志选项
	logOpts := &corev1.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}

	if target.Container != "" {
		logOpts.Container = target.Container
	}

	if opts != nil {
		if opts.TailLines > 0 {
			logOpts.TailLines = &opts.TailLines
		}
		if opts.SinceSeconds > 0 {
			logOpts.SinceSeconds = &opts.SinceSeconds
		}
		logOpts.Previous = opts.Previous
	}

	// 获取日志流
	stream, err := k8sClient.GetClientset().
		CoreV1().
		Pods(target.Namespace).
		GetLogs(target.Pod, logOpts).
		Stream(ctx)
	if err != nil {
		logger.Error("获取日志流失败", "pod", target.Pod, "error", err)
		return
	}
	defer func() {
		_ = stream.Close()
	}()

	reader := bufio.NewReader(stream)
	for {
		select {
		case <-ctx.Done():
			return
		default:
			line, err := reader.ReadString('\n')
			if err != nil {
				// 检查是否是正常关闭
				if ctx.Err() != nil {
					return
				}
				if strings.Contains(err.Error(), "closed") ||
					strings.Contains(err.Error(), "canceled") {
					return
				}
				logger.Error("读取日志失败", "pod", target.Pod, "error", err)
				return
			}

			entry := a.parseLogLine(line, target, cluster)
			select {
			case outputCh <- entry:
			case <-ctx.Done():
				return
			}
		}
	}
}

// parseLogLine 解析日志行
func (a *LogAggregator) parseLogLine(line string, target models.LogStreamTarget, cluster *models.Cluster) *models.LogEntry {
	entry := &models.LogEntry{
		ID:          uuid.New().String(),
		Type:        "container",
		ClusterID:   cluster.ID,
		ClusterName: cluster.Name,
		Namespace:   target.Namespace,
		PodName:     target.Pod,
		Container:   target.Container,
		Message:     strings.TrimSpace(line),
		Timestamp:   time.Now(),
	}

	// 解析时间戳 (K8s日志格式: 2024-01-01T00:00:00.000000000Z message)
	if len(line) > 30 && line[10] == 'T' {
		if t, err := time.Parse(time.RFC3339Nano, line[:30]); err == nil {
			entry.Timestamp = t
			entry.Message = strings.TrimSpace(line[31:])
		} else if len(line) > 20 {
			// 尝试其他时间格式
			if t, err := time.Parse(time.RFC3339, line[:20]); err == nil {
				entry.Timestamp = t
				entry.Message = strings.TrimSpace(line[21:])
			}
		}
	}

	// 智能识别日志级别
	entry.Level = a.detectLogLevel(entry.Message)

	return entry
}

// detectLogLevel 智能识别日志级别
func (a *LogAggregator) detectLogLevel(message string) string {
	lowerMsg := strings.ToLower(message)

	// 错误级别关键词
	errorPatterns := []string{"error", "err", "fail", "fatal", "exception", "panic", "critical"}
	for _, pattern := range errorPatterns {
		if strings.Contains(lowerMsg, pattern) {
			return "error"
		}
	}

	// 警告级别关键词
	warnPatterns := []string{"warn", "warning", "caution"}
	for _, pattern := range warnPatterns {
		if strings.Contains(lowerMsg, pattern) {
			return "warn"
		}
	}

	// 调试级别关键词
	debugPatterns := []string{"debug", "trace", "verbose"}
	for _, pattern := range debugPatterns {
		if strings.Contains(lowerMsg, pattern) {
			return "debug"
		}
	}

	return "info"
}

// GetContainerLogs 获取容器日志（非流式）
func (a *LogAggregator) GetContainerLogs(
	ctx context.Context,
	cluster *models.Cluster,
	namespace, podName, container string,
	tailLines int64,
	sinceSeconds int64,
	previous bool,
) (string, error) {
	// 创建K8s客户端
	k8sClient, err := NewK8sClientForCluster(cluster)
	if err != nil {
		return "", err
	}

	// 构建日志选项
	logOpts := &corev1.PodLogOptions{
		Timestamps: true,
		Previous:   previous,
	}

	if container != "" {
		logOpts.Container = container
	}

	if tailLines > 0 {
		logOpts.TailLines = &tailLines
	}

	if sinceSeconds > 0 {
		logOpts.SinceSeconds = &sinceSeconds
	}

	// 获取日志
	logs, err := k8sClient.GetClientset().
		CoreV1().
		Pods(namespace).
		GetLogs(podName, logOpts).
		Do(ctx).
		Raw()
	if err != nil {
		return "", err
	}

	return string(logs), nil
}

// SearchLogs 搜索日志
func (a *LogAggregator) SearchLogs(
	ctx context.Context,
	cluster *models.Cluster,
	query *models.LogQuery,
) ([]models.LogEntry, int, error) {
	// 创建K8s客户端
	k8sClient, err := NewK8sClientForCluster(cluster)
	if err != nil {
		return nil, 0, err
	}

	var results []models.LogEntry
	var regexPattern *regexp.Regexp

	// 编译正则表达式
	if query.Regex != "" {
		regexPattern, err = regexp.Compile(query.Regex)
		if err != nil {
			return nil, 0, err
		}
	}

	// 确定要搜索的命名空间
	namespaces := query.Namespaces
	if len(namespaces) == 0 {
		namespaces = []string{""} // 搜索所有命名空间
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 100
	}

	// 遍历命名空间获取日志并搜索
	for _, ns := range namespaces {
		pods, err := k8sClient.GetClientset().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			continue
		}

		for _, pod := range pods.Items {
			// 检查Pod名称过滤
			if len(query.Pods) > 0 && !contains(query.Pods, pod.Name) {
				continue
			}

			for _, container := range pod.Spec.Containers {
				// 检查容器过滤
				if len(query.Containers) > 0 && !contains(query.Containers, container.Name) {
					continue
				}

				// 获取日志
				logOpts := &corev1.PodLogOptions{
					Container:  container.Name,
					Timestamps: true,
				}

				tailLines := int64(limit * 10) // 获取更多行以便过滤
				logOpts.TailLines = &tailLines

				logs, err := k8sClient.GetClientset().
					CoreV1().
					Pods(pod.Namespace).
					GetLogs(pod.Name, logOpts).
					Do(ctx).
					Raw()
				if err != nil {
					continue
				}

				// 按行搜索
				lines := strings.Split(string(logs), "\n")
				for _, line := range lines {
					if line == "" {
						continue
					}

					// 关键词匹配
					if query.Keyword != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(query.Keyword)) {
						continue
					}

					// 正则匹配
					if regexPattern != nil && !regexPattern.MatchString(line) {
						continue
					}

					entry := a.parseLogLine(line, models.LogStreamTarget{
						Namespace: pod.Namespace,
						Pod:       pod.Name,
						Container: container.Name,
					}, cluster)

					// 日志级别过滤
					if len(query.Levels) > 0 && !contains(query.Levels, entry.Level) {
						continue
					}

					results = append(results, *entry)

					if len(results) >= limit {
						return results, len(results), nil
					}
				}
			}
		}
	}

	return results, len(results), nil
}

// contains 检查切片是否包含某个元素
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
