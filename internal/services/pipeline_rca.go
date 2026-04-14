package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ---------------------------------------------------------------------------
// Pipeline RCA — 失敗 Run 一鍵 AI 根因分析（CICD_ARCHITECTURE §16.3）
//
// 設計原則：
//   - 收集失敗 Step 的最後 200 行 log
//   - 收集 K8s Job / Pod describe 輸出
//   - 包含 Pipeline 定義片段（step name, type, image, command）
//   - 組裝 context 交給 AI 分析
// ---------------------------------------------------------------------------

const pipelineRCALogTailLines = 200

// PipelineRCAContext 包含 AI 根因分析所需的所有上下文。
type PipelineRCAContext struct {
	PipelineName string                  `json:"pipeline_name"`
	RunID        uint                    `json:"run_id"`
	TriggerType  string                  `json:"trigger_type"`
	RunError     string                  `json:"run_error"`
	RunDuration  string                  `json:"run_duration"`
	FailedSteps  []PipelineRCAStepDetail `json:"failed_steps"`
	StepSummary  string                  `json:"step_summary"` // e.g. "3/5 succeeded, 1 failed, 1 cancelled"
}

// PipelineRCAStepDetail 單個失敗 Step 的診斷資料。
type PipelineRCAStepDetail struct {
	StepName   string `json:"step_name"`
	StepType   string `json:"step_type"`
	Image      string `json:"image"`
	Command    string `json:"command"`
	ExitCode   *int   `json:"exit_code"`
	Error      string `json:"error"`
	RetryCount int    `json:"retry_count"`
	Duration   string `json:"duration"`
	LogTail    string `json:"log_tail"`    // last N lines
	JobStatus  string `json:"job_status"`  // K8s Job condition summary
	PodStatus  string `json:"pod_status"`  // K8s Pod status summary
	PodEvents  string `json:"pod_events"`  // K8s events for the Pod
}

// PipelineRCAResult AI 根因分析結果。
type PipelineRCAResult struct {
	PipelineName string   `json:"pipeline_name"`
	RunID        uint     `json:"run_id"`
	Analysis     string   `json:"analysis"`
	Summary      string   `json:"summary"`
	Suggestions  []string `json:"suggestions"`
	Severity     string   `json:"severity"`
	CollectedAt  string   `json:"collected_at"`
}

// PipelineRCAService 提供 Pipeline 失敗的 AI 根因分析。
type PipelineRCAService struct {
	db          *gorm.DB
	logSvc      *PipelineLogService
	aiConfigSvc *AIConfigService
}

// NewPipelineRCAService 建立 Pipeline RCA 服務。
func NewPipelineRCAService(db *gorm.DB, logSvc *PipelineLogService, aiConfigSvc *AIConfigService) *PipelineRCAService {
	return &PipelineRCAService{
		db:          db,
		logSvc:      logSvc,
		aiConfigSvc: aiConfigSvc,
	}
}

// BuildContext 收集失敗 Run 的診斷上下文（不呼叫 AI）。
func (s *PipelineRCAService) BuildContext(ctx context.Context, runID uint, clientset kubernetes.Interface) (*PipelineRCAContext, error) {
	// 1. Load run
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		return nil, fmt.Errorf("load pipeline run %d: %w", runID, err)
	}
	if run.Status != models.PipelineRunStatusFailed {
		return nil, fmt.Errorf("run %d status is %q, not failed", runID, run.Status)
	}

	// 2. Load pipeline name
	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).Select("id, name").First(&pipeline, run.PipelineID).Error; err != nil {
		return nil, fmt.Errorf("load pipeline %d: %w", run.PipelineID, err)
	}

	// 3. Load all step runs
	var stepRuns []models.StepRun
	if err := s.db.WithContext(ctx).
		Where("pipeline_run_id = ?", runID).
		Order("step_index ASC").
		Find(&stepRuns).Error; err != nil {
		return nil, fmt.Errorf("load step runs for run %d: %w", runID, err)
	}

	// 4. Collect details for failed steps
	var failedDetails []PipelineRCAStepDetail
	statusCounts := map[string]int{}
	for i := range stepRuns {
		sr := &stepRuns[i]
		statusCounts[sr.Status]++

		if sr.Status != models.StepRunStatusFailed {
			continue
		}

		detail := PipelineRCAStepDetail{
			StepName:   sr.StepName,
			StepType:   sr.StepType,
			Image:      sr.Image,
			Command:    sr.Command,
			ExitCode:   sr.ExitCode,
			Error:      sr.Error,
			RetryCount: sr.RetryCount,
		}

		// Duration
		if sr.StartedAt != nil && sr.FinishedAt != nil {
			detail.Duration = sr.FinishedAt.Sub(*sr.StartedAt).Round(time.Second).String()
		}

		// Logs from DB
		detail.LogTail = s.getStepLogTail(ctx, sr.ID)

		// K8s Job/Pod info (best-effort)
		if clientset != nil && sr.JobName != "" && sr.JobNamespace != "" {
			detail.JobStatus = s.getJobStatus(ctx, clientset, sr.JobNamespace, sr.JobName)
			detail.PodStatus, detail.PodEvents = s.getJobPodInfo(ctx, clientset, sr.JobNamespace, sr.JobName)
		}

		failedDetails = append(failedDetails, detail)
	}

	// 5. Build step summary
	total := len(stepRuns)
	summary := fmt.Sprintf("%d/%d steps: ", total, total)
	parts := []string{}
	for _, status := range []string{"success", "failed", "cancelled", "skipped", "pending", "running"} {
		if c := statusCounts[status]; c > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", c, status))
		}
	}
	summary += strings.Join(parts, ", ")

	// 6. Run duration
	var runDuration string
	if run.StartedAt != nil && run.FinishedAt != nil {
		runDuration = run.FinishedAt.Sub(*run.StartedAt).Round(time.Second).String()
	}

	return &PipelineRCAContext{
		PipelineName: pipeline.Name,
		RunID:        runID,
		TriggerType:  run.TriggerType,
		RunError:     run.Error,
		RunDuration:  runDuration,
		FailedSteps:  failedDetails,
		StepSummary:  summary,
	}, nil
}

// Analyze 收集上下文並呼叫 AI 進行根因分析。
func (s *PipelineRCAService) Analyze(ctx context.Context, runID uint, clientset kubernetes.Interface, language string) (*PipelineRCAResult, error) {
	rcaCtx, err := s.BuildContext(ctx, runID, clientset)
	if err != nil {
		return nil, err
	}

	contextStr := FormatPipelineRCAContext(rcaCtx)
	analysis, err := s.callAI(ctx, contextStr, language)
	if err != nil {
		return nil, fmt.Errorf("AI analysis for pipeline run %d: %w", runID, err)
	}

	return &PipelineRCAResult{
		PipelineName: rcaCtx.PipelineName,
		RunID:        runID,
		Analysis:     analysis,
		Summary:      extractFirstLine(analysis),
		Suggestions:  extractSuggestions(analysis),
		Severity:     "critical", // failed run is always critical
		CollectedAt:  time.Now().UTC().Format(time.RFC3339),
	}, nil
}

// FormatPipelineRCAContext 將上下文格式化為 AI 可讀的字串。
func FormatPipelineRCAContext(ctx *PipelineRCAContext) string {
	var b strings.Builder

	b.WriteString("## Pipeline Run Overview\n")
	b.WriteString(fmt.Sprintf("- Pipeline: %s\n", ctx.PipelineName))
	b.WriteString(fmt.Sprintf("- Run ID: %d\n", ctx.RunID))
	b.WriteString(fmt.Sprintf("- Trigger: %s\n", ctx.TriggerType))
	if ctx.RunDuration != "" {
		b.WriteString(fmt.Sprintf("- Duration: %s\n", ctx.RunDuration))
	}
	if ctx.RunError != "" {
		b.WriteString(fmt.Sprintf("- Run Error: %s\n", ctx.RunError))
	}
	b.WriteString(fmt.Sprintf("- Steps: %s\n", ctx.StepSummary))

	for i, step := range ctx.FailedSteps {
		b.WriteString(fmt.Sprintf("\n## Failed Step %d: %s\n", i+1, step.StepName))
		b.WriteString(fmt.Sprintf("- Type: %s\n", step.StepType))
		if step.Image != "" {
			b.WriteString(fmt.Sprintf("- Image: %s\n", step.Image))
		}
		if step.Command != "" {
			b.WriteString(fmt.Sprintf("- Command: %s\n", truncateStr(step.Command, 500)))
		}
		if step.ExitCode != nil {
			b.WriteString(fmt.Sprintf("- Exit Code: %d\n", *step.ExitCode))
		}
		if step.Error != "" {
			b.WriteString(fmt.Sprintf("- Error: %s\n", step.Error))
		}
		if step.RetryCount > 0 {
			b.WriteString(fmt.Sprintf("- Retries: %d\n", step.RetryCount))
		}
		if step.Duration != "" {
			b.WriteString(fmt.Sprintf("- Duration: %s\n", step.Duration))
		}

		if step.JobStatus != "" {
			b.WriteString(fmt.Sprintf("\n### K8s Job Status\n%s\n", step.JobStatus))
		}
		if step.PodStatus != "" {
			b.WriteString(fmt.Sprintf("\n### K8s Pod Status\n%s\n", step.PodStatus))
		}
		if step.PodEvents != "" {
			b.WriteString(fmt.Sprintf("\n### Pod Events\n%s\n", step.PodEvents))
		}
		if step.LogTail != "" {
			b.WriteString(fmt.Sprintf("\n### Step Logs (last %d lines)\n```\n%s\n```\n", pipelineRCALogTailLines, step.LogTail))
		}
	}

	return b.String()
}

// --- Internal helpers ---

func (s *PipelineRCAService) getStepLogTail(ctx context.Context, stepRunID uint) string {
	content, err := s.logSvc.GetLogContent(ctx, stepRunID)
	if err != nil {
		logger.Debug("pipeline RCA: log fetch failed", "step_run_id", stepRunID, "error", err)
		return ""
	}
	return tailLines(content, pipelineRCALogTailLines)
}

func (s *PipelineRCAService) getJobStatus(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string) string {
	job, err := clientset.BatchV1().Jobs(namespace).Get(ctx, jobName, metav1.GetOptions{})
	if err != nil {
		return fmt.Sprintf("(unavailable: %v)", err)
	}
	return formatJobConditions(job)
}

func (s *PipelineRCAService) getJobPodInfo(ctx context.Context, clientset kubernetes.Interface, namespace, jobName string) (podStatus, podEvents string) {
	pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return "", ""
	}

	pod := &pods.Items[len(pods.Items)-1] // most recent pod
	podStatus = formatPodStatus(pod)
	podEvents = s.getPodEvents(ctx, clientset, namespace, pod.Name)
	return
}

func (s *PipelineRCAService) getPodEvents(ctx context.Context, clientset kubernetes.Interface, namespace, podName string) string {
	events, err := clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil || len(events.Items) == 0 {
		return ""
	}

	var b strings.Builder
	start := 0
	if len(events.Items) > 30 {
		start = len(events.Items) - 30
	}
	for _, ev := range events.Items[start:] {
		b.WriteString(fmt.Sprintf("- [%s] %s: %s (count: %d)\n", ev.Type, ev.Reason, ev.Message, ev.Count))
	}
	return b.String()
}

func (s *PipelineRCAService) callAI(ctx context.Context, rcaContext, language string) (string, error) {
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

	systemPrompt := fmt.Sprintf(`You are a CI/CD pipeline failure analysis expert. Analyze the following pipeline run diagnostic information and provide:

1. **Root Cause**: A clear explanation of why the pipeline step(s) failed
2. **Impact**: What this failure means for the deployment or build process
3. **Suggestions**: Concrete steps to fix the issue (numbered list)
4. **Prevention**: How to prevent this in the future

Focus on the failed step logs, exit codes, and K8s events. Be concise and actionable. %s`, langInstruction)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: fmt.Sprintf("Please analyze this pipeline run failure:\n\n%s", rcaContext)},
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

// --- Pure utility functions ---

func tailLines(content string, maxLines int) string {
	lines := strings.Split(content, "\n")
	if len(lines) <= maxLines {
		return content
	}
	return strings.Join(lines[len(lines)-maxLines:], "\n")
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func formatJobConditions(job *batchv1.Job) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Active: %d, Succeeded: %d, Failed: %d",
		job.Status.Active, job.Status.Succeeded, job.Status.Failed))
	for _, cond := range job.Status.Conditions {
		parts = append(parts, fmt.Sprintf("Condition: %s=%s (reason: %s)",
			cond.Type, cond.Status, cond.Reason))
	}
	return strings.Join(parts, "\n")
}

func formatPodStatus(pod *corev1.Pod) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Phase: %s, Node: %s\n", pod.Status.Phase, pod.Spec.NodeName))
	for _, cs := range pod.Status.ContainerStatuses {
		b.WriteString(fmt.Sprintf("- Container %s: ready=%v, restarts=%d", cs.Name, cs.Ready, cs.RestartCount))
		if cs.State.Terminated != nil {
			b.WriteString(fmt.Sprintf(", exitCode=%d, reason=%s", cs.State.Terminated.ExitCode, cs.State.Terminated.Reason))
		}
		if cs.State.Waiting != nil {
			b.WriteString(fmt.Sprintf(", waiting: %s", cs.State.Waiting.Reason))
		}
		b.WriteString("\n")
	}
	return b.String()
}
