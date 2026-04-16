package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/shaia/Synapse/internal/services/pipeline/engine"
	"github.com/shaia/Synapse/internal/services/pipeline/engine/github"
)

func TestCIEngineService_ListAvailableEngines_IncludesRegisteredGitHub(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{})
	}))
	defer srv.Close()

	f := engine.NewFactory()
	require.NoError(t, github.Register(f))

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	gormDB, err := gorm.Open(postgres.New(postgres.Config{
		Conn:                 db,
		PreferSimpleProtocol: true,
	}), &gorm.Config{Logger: gormlogger.Default.LogMode(gormlogger.Silent)})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, _ := gormDB.DB(); sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	svc := NewCIEngineService(gormDB, f)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "engine_type", "enabled", "endpoint", "token", "extra_json",
		}).AddRow(
			1, "github-main", "github", true, srv.URL, "pat-xyz",
			`{"owner":"o","repo":"r","workflow_id":"build.yml"}`,
		))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2)

	gh := got[1]
	require.Equal(t, "github", gh.Type)
	require.True(t, gh.Available, "github should be reachable, error=%s", gh.Error)
	require.NotEmpty(t, gh.Version)
	require.NotNil(t, gh.Capabilities)
	require.True(t, gh.Capabilities.SupportsDAG)
	require.True(t, gh.Capabilities.SupportsLiveLog)
}
