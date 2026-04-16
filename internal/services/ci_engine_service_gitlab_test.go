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
	"github.com/shaia/Synapse/internal/services/pipeline/engine/gitlab"
)

// TestCIEngineService_ListAvailableEngines_IncludesRegisteredGitLab exercises
// the full Stage-5 integration: Register the GitLab builder, seed a
// ci_engine_configs row via sqlmock, stand up a fake GitLab server, and
// verify the service reports the external engine with version + capabilities.
func TestCIEngineService_ListAvailableEngines_IncludesRegisteredGitLab(t *testing.T) {
	// 1. Fake GitLab server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{"version": "16.11.0"})
	}))
	defer srv.Close()

	// 2. Factory with the GitLab adapter registered
	f := engine.NewFactory()
	require.NoError(t, gitlab.Register(f))

	// 3. sqlmock-backed service
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

	// 4. Seed a GitLab row — columns match the full schema so AfterFind is
	//    happy. The mock dialect uses postgres but returns plain values.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "ci_engine_configs"`)).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "engine_type", "enabled", "endpoint",
			"token", "extra_json",
		}).AddRow(
			1, "gl-main", "gitlab", true, srv.URL,
			"secret-token", `{"project_id":42,"default_ref":"main"}`,
		))

	// 5. Call
	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2) // native + gitlab

	// native (not registered in this factory) is reported with Error
	native := got[0]
	require.Equal(t, "native", native.Type)
	require.False(t, native.Available)

	// gitlab should be reachable via the fake server
	gl := got[1]
	require.Equal(t, "gitlab", gl.Type)
	require.True(t, gl.Available, "gitlab should be reachable, got error: %s", gl.Error)
	require.Equal(t, "16.11.0", gl.Version)
	require.NotNil(t, gl.Capabilities)
	require.True(t, gl.Capabilities.SupportsDAG)
	require.True(t, gl.Capabilities.SupportsLiveLog)
}
