package services

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
)

// ---------------------------------------------------------------------------
// Matrix Builds — 同層並行展開
//
// 設計（CICD_ARCHITECTURE §M13b）：
//   - StepDef.Matrix 定義 param name → value[]
//   - 笛卡兒積展開為 N 個 StepRun（如 2×3 = 6 個並行 Step）
//   - 所有展開的 StepRun 必須成功，Step 才算成功
//   - Matrix params 透過環境變數注入（MATRIX_<KEY>=<value>）
//   - 展開的 StepRun.StepName = "<original>/<combo>" 如 "build/go1.21-linux"
// ---------------------------------------------------------------------------

// MatrixCombo 一組矩陣參數組合。
type MatrixCombo struct {
	Values map[string]string // param_name → selected_value
	Label  string            // 用於 StepRun 命名，如 "go1.21-linux"
}

// ExpandMatrix 將 matrix 定義展開為笛卡兒積組合。
// 若 matrix 為空或 nil，回傳 nil（非 matrix step）。
func ExpandMatrix(matrix map[string][]string) []MatrixCombo {
	if len(matrix) == 0 {
		return nil
	}

	// 取得排序後的 key（確保展開順序穩定）
	keys := make([]string, 0, len(matrix))
	for k := range matrix {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 笛卡兒積
	combos := []MatrixCombo{{Values: make(map[string]string), Label: ""}}
	for _, key := range keys {
		values := matrix[key]
		if len(values) == 0 {
			continue
		}
		var newCombos []MatrixCombo
		for _, combo := range combos {
			for _, val := range values {
				newValues := make(map[string]string, len(combo.Values)+1)
				for k, v := range combo.Values {
					newValues[k] = v
				}
				newValues[key] = val

				label := combo.Label
				if label != "" {
					label += "-"
				}
				label += val

				newCombos = append(newCombos, MatrixCombo{
					Values: newValues,
					Label:  label,
				})
			}
		}
		combos = newCombos
	}

	return combos
}

// IsMatrixStep 判斷 StepDef 是否為 matrix step。
func IsMatrixStep(step StepDef) bool {
	return len(step.Matrix) > 0
}

// executeMatrixStep 並行執行 matrix 展開的所有 sub-steps。
// 回傳 true 表示任一 sub-step 失敗。
func (s *PipelineScheduler) executeMatrixStep(
	ctx context.Context,
	run *models.PipelineRun,
	step StepDef,
	parentStepRun *models.StepRun,
) bool {
	combos := ExpandMatrix(step.Matrix)
	if len(combos) == 0 {
		// 不應該到這裡，但 fallback 為一般執行
		return s.executeStepWithRetry(ctx, run, parentStepRun, step)
	}

	logger.Info("expanding matrix step",
		"step_name", step.Name,
		"run_id", run.ID,
		"combinations", len(combos),
	)

	// 將 parent step 標記為 running
	parentStepRun.Status = models.StepRunStatusRunning
	if err := s.db.WithContext(ctx).Save(parentStepRun).Error; err != nil {
		logger.Error("failed to save matrix parent step", "step_run_id", parentStepRun.ID, "error", err)
	}

	// 建立並行 sub-step runs
	type matrixResult struct {
		combo  MatrixCombo
		failed bool
	}

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		results []matrixResult
	)

	for _, combo := range combos {
		combo := combo // capture
		wg.Add(1)

		go func() {
			defer wg.Done()

			// 為每個 combo 建立獨立的 StepRun
			subName := fmt.Sprintf("%s/%s", step.Name, combo.Label)
			dependsOnJSON, _ := json.Marshal(step.DependsOn)

			// 將 matrix params 注入 config 的 env
			subConfig := injectMatrixEnv(step.Config, combo.Values)

			subSR := &models.StepRun{
				PipelineRunID: run.ID,
				StepName:      subName,
				StepType:      step.Type,
				StepIndex:     parentStepRun.StepIndex,
				Status:        models.StepRunStatusPending,
				Image:         step.Image,
				Command:       step.Command,
				ConfigJSON:    subConfig,
				DependsOn:     string(dependsOnJSON),
				MaxRetries:    step.MaxRetries,
			}

			if err := s.db.WithContext(ctx).Create(subSR).Error; err != nil {
				logger.Error("failed to create matrix sub-step",
					"step_name", subName, "error", err)
				mu.Lock()
				results = append(results, matrixResult{combo: combo, failed: true})
				mu.Unlock()
				return
			}

			failed := s.executeStepWithRetry(ctx, run, subSR, step)

			mu.Lock()
			results = append(results, matrixResult{combo: combo, failed: failed})
			mu.Unlock()
		}()
	}

	wg.Wait()

	// 統整結果：任一 sub-step 失敗 → parent 失敗
	anyFailed := false
	for _, r := range results {
		if r.failed {
			anyFailed = true
			break
		}
	}

	// 更新 parent step 狀態
	if anyFailed {
		parentStepRun.Status = models.StepRunStatusFailed
		parentStepRun.Error = "one or more matrix combinations failed"
	} else {
		parentStepRun.Status = models.StepRunStatusSuccess
	}
	if err := s.db.WithContext(ctx).Save(parentStepRun).Error; err != nil {
		logger.Error("failed to save matrix parent step result", "step_run_id", parentStepRun.ID, "error", err)
	}

	return anyFailed
}

// injectMatrixEnv 將 matrix combo 參數注入 StepConfig 的 env map。
// 環境變數格式：MATRIX_<UPPER_KEY>=<value>
func injectMatrixEnv(configJSON string, matrixValues map[string]string) string {
	var cfg StepConfig
	if configJSON != "" {
		if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
			cfg = StepConfig{}
		}
	}
	if cfg.Env == nil {
		cfg.Env = make(map[string]string)
	}

	for k, v := range matrixValues {
		envKey := "MATRIX_" + strings.ToUpper(strings.ReplaceAll(k, "-", "_"))
		cfg.Env[envKey] = v
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return configJSON
	}
	return string(data)
}
