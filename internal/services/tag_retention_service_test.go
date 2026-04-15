package services

import (
	"testing"
	"time"

	"github.com/shaia/Synapse/internal/models"
	"github.com/stretchr/testify/assert"
)

func makeTags(names []string, ages []int) []RegistryTag {
	tags := make([]RegistryTag, len(names))
	for i, name := range names {
		tags[i] = RegistryTag{
			Name:      name,
			CreatedAt: time.Now().AddDate(0, 0, -ages[i]),
		}
	}
	return tags
}

func TestEvalKeepLastN(t *testing.T) {
	tags := makeTags([]string{"v1", "v2", "v3", "v4", "v5"}, []int{5, 4, 3, 2, 1})

	t.Run("keep 3", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 3)
		assert.Len(t, keep, 3)
		assert.Len(t, remove, 2)
		// Newest first
		assert.Equal(t, "v5", keep[0].Name)
		assert.Equal(t, "v4", keep[1].Name)
		assert.Equal(t, "v3", keep[2].Name)
	})

	t.Run("keep more than available", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 10)
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("keep 0 means keep all", func(t *testing.T) {
		keep, remove := evalKeepLastN(tags, 0)
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("empty tags", func(t *testing.T) {
		keep, remove := evalKeepLastN(nil, 3)
		assert.Empty(t, keep)
		assert.Empty(t, remove)
	})
}

func TestEvalKeepByAge(t *testing.T) {
	tags := makeTags([]string{"old", "medium", "new"}, []int{30, 10, 1})

	t.Run("keep 15 days", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 15)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
		assert.Equal(t, "old", remove[0].Name)
	})

	t.Run("keep 5 days", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 5)
		assert.Len(t, keep, 1)
		assert.Len(t, remove, 2)
		assert.Equal(t, "new", keep[0].Name)
	})

	t.Run("keep 0 means keep all", func(t *testing.T) {
		keep, remove := evalKeepByAge(tags, 0)
		assert.Len(t, keep, 3)
		assert.Empty(t, remove)
	})
}

func TestEvalKeepByRegex(t *testing.T) {
	tags := makeTags([]string{"v1.0.0", "v2.0.0", "latest", "dev-abc", "v3.0.0-rc1"}, []int{5, 4, 3, 2, 1})

	t.Run("keep semver pattern", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, `^v\d+\.\d+\.\d+$`)
		assert.Len(t, keep, 2) // v1.0.0, v2.0.0
		assert.Len(t, remove, 3)
	})

	t.Run("keep all v* tags", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, `^v`)
		assert.Len(t, keep, 3) // v1.0.0, v2.0.0, v3.0.0-rc1
		assert.Len(t, remove, 2)
	})

	t.Run("empty pattern keeps all", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, "")
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})

	t.Run("invalid regex keeps all", func(t *testing.T) {
		keep, remove := evalKeepByRegex(tags, "[invalid")
		assert.Len(t, keep, 5)
		assert.Empty(t, remove)
	})
}

func TestEvaluateRetention_Dispatch(t *testing.T) {
	tags := makeTags([]string{"a", "b", "c"}, []int{3, 2, 1})

	t.Run("keep_last_n", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepLastN, KeepCount: 2}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
	})

	t.Run("keep_by_age", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepByAge, KeepDays: 2}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 1)
		assert.Len(t, remove, 2)
	})

	t.Run("keep_by_regex", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: models.RetentionKeepByRegex, KeepPattern: "^[ab]$"}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 2)
		assert.Len(t, remove, 1)
	})

	t.Run("unknown type keeps all", func(t *testing.T) {
		policy := &models.TagRetentionPolicy{RetentionType: "unknown"}
		keep, remove := evaluateRetention(policy, tags)
		assert.Len(t, keep, 3)
		assert.Empty(t, remove)
	})
}

func TestValidateRetentionPolicy(t *testing.T) {
	t.Run("valid keep_last_n", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
		})
		assert.NoError(t, err)
	})

	t.Run("invalid keep_last_n zero", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     0,
		})
		assert.Error(t, err)
	})

	t.Run("valid keep_by_age", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByAge,
			KeepDays:      30,
		})
		assert.NoError(t, err)
	})

	t.Run("valid keep_by_regex", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByRegex,
			KeepPattern:   `^v\d+`,
		})
		assert.NoError(t, err)
	})

	t.Run("invalid regex", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepByRegex,
			KeepPattern:   "[invalid",
		})
		assert.Error(t, err)
	})

	t.Run("invalid type", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: "bad",
		})
		assert.Error(t, err)
	})

	t.Run("valid cron", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
			CronExpr:      "0 3 * * 0",
		})
		assert.NoError(t, err)
	})

	t.Run("invalid cron", func(t *testing.T) {
		err := validateRetentionPolicy(&models.TagRetentionPolicy{
			RetentionType: models.RetentionKeepLastN,
			KeepCount:     5,
			CronExpr:      "bad cron",
		})
		assert.Error(t, err)
	})
}
