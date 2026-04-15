package services

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/shaia/Synapse/pkg/logger"
	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// TagRetentionService — Tag 保留策略 CRUD + 評估（CICD_ARCHITECTURE §11, M15）
// ---------------------------------------------------------------------------

// TagRetentionService 管理 Tag 保留策略。
type TagRetentionService struct {
	db          *gorm.DB
	registrySvc *RegistryService
}

// NewTagRetentionService 建立 TagRetentionService。
func NewTagRetentionService(db *gorm.DB, registrySvc *RegistryService) *TagRetentionService {
	return &TagRetentionService{db: db, registrySvc: registrySvc}
}

// CreatePolicy 建立保留策略。
func (s *TagRetentionService) CreatePolicy(ctx context.Context, policy *models.TagRetentionPolicy) error {
	if err := validateRetentionPolicy(policy); err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Create(policy).Error; err != nil {
		return fmt.Errorf("create tag retention policy: %w", err)
	}

	logger.Info("tag retention policy created",
		"policy_id", policy.ID,
		"registry_id", policy.RegistryID,
		"type", policy.RetentionType,
	)
	return nil
}

// GetPolicy 取得單一保留策略。
func (s *TagRetentionService) GetPolicy(ctx context.Context, id uint) (*models.TagRetentionPolicy, error) {
	var policy models.TagRetentionPolicy
	if err := s.db.WithContext(ctx).First(&policy, id).Error; err != nil {
		return nil, fmt.Errorf("get tag retention policy %d: %w", id, err)
	}
	return &policy, nil
}

// ListPolicies 列出 Registry 的保留策略。
func (s *TagRetentionService) ListPolicies(ctx context.Context, registryID uint) ([]models.TagRetentionPolicy, error) {
	var policies []models.TagRetentionPolicy
	if err := s.db.WithContext(ctx).
		Where("registry_id = ?", registryID).
		Order("id ASC").
		Find(&policies).Error; err != nil {
		return nil, fmt.Errorf("list tag retention policies: %w", err)
	}
	return policies, nil
}

// UpdatePolicy 更新保留策略。
func (s *TagRetentionService) UpdatePolicy(ctx context.Context, id uint, updates map[string]interface{}) error {
	result := s.db.WithContext(ctx).Model(&models.TagRetentionPolicy{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update tag retention policy %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tag retention policy %d not found", id)
	}

	logger.Info("tag retention policy updated", "policy_id", id)
	return nil
}

// DeletePolicy 刪除保留策略。
func (s *TagRetentionService) DeletePolicy(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.TagRetentionPolicy{}, id)
	if result.Error != nil {
		return fmt.Errorf("delete tag retention policy %d: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("tag retention policy %d not found", id)
	}

	logger.Info("tag retention policy deleted", "policy_id", id)
	return nil
}

// ---------------------------------------------------------------------------
// Dry-run evaluation — returns which tags WOULD be deleted
// ---------------------------------------------------------------------------

// RetentionEvalResult 評估結果。
type RetentionEvalResult struct {
	Repository  string   `json:"repository"`
	TagsToKeep  []string `json:"tags_to_keep"`
	TagsToDelete []string `json:"tags_to_delete"`
}

// EvaluatePolicy 評估保留策略（dry-run），回傳哪些 tag 會被保留/刪除。
func (s *TagRetentionService) EvaluatePolicy(ctx context.Context, policyID uint) ([]RetentionEvalResult, error) {
	policy, err := s.GetPolicy(ctx, policyID)
	if err != nil {
		return nil, err
	}

	registry, err := s.registrySvc.GetRegistry(ctx, policy.RegistryID)
	if err != nil {
		return nil, fmt.Errorf("get registry for policy: %w", err)
	}

	adapter, err := NewRegistryAdapter(registry)
	if err != nil {
		return nil, fmt.Errorf("create registry adapter: %w", err)
	}

	// List repositories matching the policy's repository pattern
	repos, err := adapter.ListRepositories(ctx, registry.DefaultProject)
	if err != nil {
		return nil, fmt.Errorf("list repositories: %w", err)
	}

	var results []RetentionEvalResult

	for _, repo := range repos {
		// Match repository against policy pattern
		matched, _ := filepath.Match(policy.RepositoryMatch, repo.Name)
		if !matched && policy.RepositoryMatch != "*" {
			continue
		}

		tags, err := adapter.ListTags(ctx, repo.Name)
		if err != nil {
			logger.Warn("failed to list tags for retention evaluation",
				"repository", repo.Name,
				"error", err,
			)
			continue
		}

		// Filter tags by tag_match pattern
		var matchedTags []RegistryTag
		for _, tag := range tags {
			tagMatched, _ := filepath.Match(policy.TagMatch, tag.Name)
			if tagMatched || policy.TagMatch == "*" || policy.TagMatch == "" {
				matchedTags = append(matchedTags, tag)
			}
		}

		keep, remove := evaluateRetention(policy, matchedTags)

		if len(remove) > 0 {
			keepNames := make([]string, len(keep))
			for i, t := range keep {
				keepNames[i] = t.Name
			}
			removeNames := make([]string, len(remove))
			for i, t := range remove {
				removeNames[i] = t.Name
			}

			results = append(results, RetentionEvalResult{
				Repository:   repo.Name,
				TagsToKeep:   keepNames,
				TagsToDelete: removeNames,
			})
		}
	}

	// Update last_run_at and result
	now := time.Now()
	summary, _ := json.Marshal(map[string]interface{}{
		"evaluated_repos": len(results),
		"dry_run":         true,
	})
	s.db.WithContext(ctx).Model(&models.TagRetentionPolicy{}).
		Where("id = ?", policyID).
		Updates(map[string]interface{}{
			"last_run_at":     &now,
			"last_run_result": string(summary),
		})

	return results, nil
}

// ---------------------------------------------------------------------------
// Retention evaluation logic (pure function)
// ---------------------------------------------------------------------------

func evaluateRetention(policy *models.TagRetentionPolicy, tags []RegistryTag) (keep, remove []RegistryTag) {
	if len(tags) == 0 {
		return nil, nil
	}

	switch policy.RetentionType {
	case models.RetentionKeepLastN:
		return evalKeepLastN(tags, policy.KeepCount)
	case models.RetentionKeepByAge:
		return evalKeepByAge(tags, policy.KeepDays)
	case models.RetentionKeepByRegex:
		return evalKeepByRegex(tags, policy.KeepPattern)
	default:
		return tags, nil // unknown type → keep all
	}
}

func evalKeepLastN(tags []RegistryTag, n int) (keep, remove []RegistryTag) {
	if n <= 0 {
		return tags, nil
	}

	// Sort by CreatedAt descending (newest first)
	sorted := make([]RegistryTag, len(tags))
	copy(sorted, tags)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].CreatedAt.After(sorted[j].CreatedAt)
	})

	if n >= len(sorted) {
		return sorted, nil
	}

	return sorted[:n], sorted[n:]
}

func evalKeepByAge(tags []RegistryTag, days int) (keep, remove []RegistryTag) {
	if days <= 0 {
		return tags, nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)

	for _, tag := range tags {
		if tag.CreatedAt.After(cutoff) {
			keep = append(keep, tag)
		} else {
			remove = append(remove, tag)
		}
	}
	return
}

func evalKeepByRegex(tags []RegistryTag, pattern string) (keep, remove []RegistryTag) {
	if pattern == "" {
		return tags, nil
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		logger.Warn("invalid retention regex pattern", "pattern", pattern, "error", err)
		return tags, nil
	}

	for _, tag := range tags {
		if re.MatchString(tag.Name) {
			keep = append(keep, tag)
		} else {
			remove = append(remove, tag)
		}
	}
	return
}

// ---------------------------------------------------------------------------
// Validation
// ---------------------------------------------------------------------------

func validateRetentionPolicy(policy *models.TagRetentionPolicy) error {
	switch policy.RetentionType {
	case models.RetentionKeepLastN:
		if policy.KeepCount <= 0 {
			return fmt.Errorf("keep_last_n requires keep_count > 0")
		}
	case models.RetentionKeepByAge:
		if policy.KeepDays <= 0 {
			return fmt.Errorf("keep_by_age requires keep_days > 0")
		}
	case models.RetentionKeepByRegex:
		if policy.KeepPattern == "" {
			return fmt.Errorf("keep_by_regex requires keep_pattern")
		}
		if _, err := regexp.Compile(policy.KeepPattern); err != nil {
			return fmt.Errorf("invalid keep_pattern regex: %w", err)
		}
	default:
		return fmt.Errorf("invalid retention_type %q, must be keep_last_n|keep_by_age|keep_by_regex", policy.RetentionType)
	}

	if policy.CronExpr != "" {
		if err := ValidateCronExpression(policy.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
	}

	return nil
}
