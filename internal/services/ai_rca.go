package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/pkg/logger"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// RCAResult AI 根因分析結果
type RCAResult struct {
	PodName     string     `json:"pod_name"`
	Namespace   string     `json:"namespace"`
	Status      string     `json:"status"`
	Analysis    string     `json:"analysis"`
	Summary     string     `json:"summary"`
	Suggestions []string   `json:"suggestions"`
	Severity    string     `json:"severity"` // critical, warning, info
	CollectedAt string     `json:"collected_at"`
	Context     RCAContext `json:"context"`
}

// RCAContext 分析所收集的原始上下文
type RCAContext struct {
	PodPhase        string   `json:"pod_phase"`
	ContainerErrors []string `json:"container_errors"`
	RecentEvents    int      `json:"recent_events"`
	LogLines        int      `json:"log_lines"`
	OwnerKind       string   `json:"owner_kind"`
	OwnerName       string   `json:"owner_name"`
}

// RCAService AI 根因分析服務
type RCAService struct {
	aiConfigSvc *AIConfigService
}

// NewRCAService 建立 RCA 服務
func NewRCAService(aiConfigSvc *AIConfigService) *RCAService {
	return &RCAService{aiConfigSvc: aiConfigSvc}
}

// AnalyzePod 對指定 Pod 進行根因分析
func (s *RCAService) AnalyzePod(ctx context.Context, clientset kubernetes.Interface, namespace, podName, language string) (*RCAResult, error) {
	// 1. Get pod details
	pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod %s/%s: %w", namespace, podName, err)
	}

	// 2. Collect events
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		logger.Warn("RCA: failed to list events, continuing", "error", err)
		events = &corev1.EventList{}
	}

	// 3. Collect logs from failing containers
	var logSnippets []string
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil || cs.RestartCount > 0 {
			logs, logErr := s.getContainerLogs(ctx, clientset, namespace, podName, cs.Name)
			if logErr != nil {
				logger.Warn("RCA: failed to get container logs", "container", cs.Name, "error", logErr)
				continue
			}
			logSnippets = append(logSnippets, fmt.Sprintf("=== Container: %s ===\n%s", cs.Name, logs))
		}
	}
	// If no failing containers found, get logs from the first container
	if len(logSnippets) == 0 && len(pod.Spec.Containers) > 0 {
		logs, logErr := s.getContainerLogs(ctx, clientset, namespace, podName, pod.Spec.Containers[0].Name)
		if logErr == nil {
			logSnippets = append(logSnippets, fmt.Sprintf("=== Container: %s ===\n%s", pod.Spec.Containers[0].Name, logs))
		}
	}

	// 4. Find owner workload info
	ownerKind, ownerName := "", ""
	if len(pod.OwnerReferences) > 0 {
		ownerKind = pod.OwnerReferences[0].Kind
		ownerName = pod.OwnerReferences[0].Name
	}

	// 5. Build context for AI
	contextStr := s.buildContextString(pod, events, logSnippets, ownerKind, ownerName)

	// 6. Call AI
	analysis, err := s.callAI(ctx, contextStr, language)
	if err != nil {
		return nil, fmt.Errorf("AI analysis for pod %s/%s: %w", namespace, podName, err)
	}

	// 7. Build container errors list
	var containerErrors []string
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			containerErrors = append(containerErrors, fmt.Sprintf("%s: %s (%s)", cs.Name, cs.State.Waiting.Reason, cs.State.Waiting.Message))
		}
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			containerErrors = append(containerErrors, fmt.Sprintf("%s: terminated with exit code %d (%s)", cs.Name, cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
		}
	}

	severity := "info"
	phase := string(pod.Status.Phase)
	if phase == "Failed" || containsAny(containerErrors, "CrashLoopBackOff", "OOMKilled", "Error") {
		severity = "critical"
	} else if phase == "Pending" || len(containerErrors) > 0 {
		severity = "warning"
	}

	return &RCAResult{
		PodName:     podName,
		Namespace:   namespace,
		Status:      phase,
		Analysis:    analysis,
		Summary:     extractFirstLine(analysis),
		Suggestions: extractSuggestions(analysis),
		Severity:    severity,
		CollectedAt: time.Now().UTC().Format(time.RFC3339),
		Context: RCAContext{
			PodPhase:        phase,
			ContainerErrors: containerErrors,
			RecentEvents:    len(events.Items),
			LogLines:        countLines(logSnippets),
			OwnerKind:       ownerKind,
			OwnerName:       ownerName,
		},
	}, nil
}

func (s *RCAService) getContainerLogs(ctx context.Context, clientset kubernetes.Interface, namespace, podName, container string) (string, error) {
	tailLines := int64(100)
	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	logBytes, err := req.DoRaw(ctx)
	if err != nil {
		// Try previous terminated container logs
		opts.Previous = true
		req = clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
		logBytes, err = req.DoRaw(ctx)
		if err != nil {
			return "", fmt.Errorf("get logs for %s: %w", container, err)
		}
	}
	return string(logBytes), nil
}

func (s *RCAService) buildContextString(pod *corev1.Pod, events *corev1.EventList, logSnippets []string, ownerKind, ownerName string) string {
	var b strings.Builder
	b.WriteString("## Pod Information\n")
	b.WriteString(fmt.Sprintf("- Name: %s\n", pod.Name))
	b.WriteString(fmt.Sprintf("- Namespace: %s\n", pod.Namespace))
	b.WriteString(fmt.Sprintf("- Phase: %s\n", pod.Status.Phase))
	b.WriteString(fmt.Sprintf("- Node: %s\n", pod.Spec.NodeName))
	if ownerKind != "" {
		b.WriteString(fmt.Sprintf("- Owner: %s/%s\n", ownerKind, ownerName))
	}

	b.WriteString("\n## Container Statuses\n")
	for _, cs := range pod.Status.ContainerStatuses {
		b.WriteString(fmt.Sprintf("- %s: ready=%v, restartCount=%d", cs.Name, cs.Ready, cs.RestartCount))
		if cs.State.Waiting != nil {
			b.WriteString(fmt.Sprintf(", waiting: %s (%s)", cs.State.Waiting.Reason, cs.State.Waiting.Message))
		}
		if cs.State.Terminated != nil {
			b.WriteString(fmt.Sprintf(", terminated: exitCode=%d reason=%s", cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n## Pod Conditions\n")
	for _, cond := range pod.Status.Conditions {
		if cond.Status != corev1.ConditionTrue {
			b.WriteString(fmt.Sprintf("- %s: %s (reason: %s, message: %s)\n", cond.Type, cond.Status, cond.Reason, cond.Message))
		}
	}

	if len(events.Items) > 0 {
		b.WriteString("\n## Recent Events (last 50)\n")
		count := len(events.Items)
		start := 0
		if count > 50 {
			start = count - 50
		}
		for _, ev := range events.Items[start:] {
			b.WriteString(fmt.Sprintf("- [%s] %s: %s (count: %d)\n", ev.Type, ev.Reason, ev.Message, ev.Count))
		}
	}

	if len(logSnippets) > 0 {
		b.WriteString("\n## Container Logs (last 100 lines per container)\n")
		for _, snippet := range logSnippets {
			b.WriteString(snippet)
			b.WriteString("\n")
		}
	}

	return b.String()
}

func (s *RCAService) callAI(ctx context.Context, rcaContext, language string) (string, error) {
	aiConfig, err := s.aiConfigSvc.GetConfigWithAPIKey()
	if err != nil || aiConfig == nil {
		return "", fmt.Errorf("AI not configured: %w", err)
	}
	if !aiConfig.Enabled || aiConfig.APIKey == "" {
		return "", fmt.Errorf("AI is not enabled")
	}

	langInstruction := "Reply in the same language the user writes in."
	if language != "" {
		langInstruction = fmt.Sprintf("Always reply in %s.", language)
	}

	systemPrompt := fmt.Sprintf(`You are a Kubernetes root cause analysis expert. Analyze the following pod diagnostic information and provide:

1. **Root Cause**: A clear explanation of why the pod is failing or unhealthy
2. **Impact**: What is the blast radius of this issue
3. **Suggestions**: Concrete steps to fix the issue (numbered list)
4. **Prevention**: How to prevent this in the future

Be concise and actionable. Use technical K8s terminology. %s`, langInstruction)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Please analyze this pod's root cause:\n\n%s", rcaContext)},
	}

	provider := NewAIProvider(aiConfig)
	resp, err := provider.Chat(ctx, ChatRequest{Messages: messages})
	if err != nil {
		return "", fmt.Errorf("send chat request: %w", err)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("empty AI response")
	}
	return resp.Choices[0].Message.Content, nil
}

// helper functions

func containsAny(items []string, keywords ...string) bool {
	for _, item := range items {
		for _, kw := range keywords {
			if strings.Contains(item, kw) {
				return true
			}
		}
	}
	return false
}

func extractFirstLine(s string) string {
	lines := strings.SplitN(s, "\n", 2)
	line := strings.TrimSpace(lines[0])
	// Remove markdown heading prefix
	line = strings.TrimLeft(line, "# ")
	if len(line) > 200 {
		return line[:200] + "..."
	}
	return line
}

func extractSuggestions(analysis string) []string {
	var suggestions []string
	lines := strings.Split(analysis, "\n")
	inSuggestions := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "suggestion") || strings.Contains(lower, "fix") || strings.Contains(lower, "step") || strings.Contains(lower, "建議") || strings.Contains(lower, "修復") {
			inSuggestions = true
			continue
		}
		if inSuggestions {
			if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
				suggestions = append(suggestions, strings.TrimLeft(trimmed, "- *"))
			} else if len(trimmed) > 2 && trimmed[0] >= '1' && trimmed[0] <= '9' && trimmed[1] == '.' {
				suggestions = append(suggestions, strings.TrimSpace(trimmed[2:]))
			} else if trimmed == "" && len(suggestions) > 0 {
				break
			}
		}
	}
	return suggestions
}

func countLines(snippets []string) int {
	total := 0
	for _, s := range snippets {
		total += strings.Count(s, "\n") + 1
	}
	return total
}
