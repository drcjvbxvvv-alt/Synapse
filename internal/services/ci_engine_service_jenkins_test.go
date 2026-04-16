package services

import (
	"context"
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
	"github.com/shaia/Synapse/internal/services/pipeline/engine/jenkins"
)

// TestCIEngineService_ListAvailableEngines_IncludesRegisteredJenkins
// exercises the Stage-5 integration: Register the Jenkins builder, seed a
// ci_engine_configs row via sqlmock, stand up a fake Jenkins that returns
// the X-Jenkins header, and verify the service reports the external engine
// as available with version + capabilities.
func TestCIEngineService_ListAvailableEngines_IncludesRegisteredJenkins(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Jenkins", "2.426.1")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	f := engine.NewFactory()
	require.NoError(t, jenkins.Register(f))

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
			"id", "name", "engine_type", "enabled", "endpoint",
			"username", "token", "extra_json",
		}).AddRow(
			1, "jenkins-main", "jenkins", true, srv.URL,
			"bot", "api-token", `{"job_path":"my-job"}`,
		))

	got, err := svc.ListAvailableEngines(context.Background())
	require.NoError(t, err)
	require.Len(t, got, 2) // native + jenkins

	jk := got[1]
	require.Equal(t, "jenkins", jk.Type)
	require.True(t, jk.Available, "jenkins should be reachable, error: %s", jk.Error)
	require.Equal(t, "2.426.1", jk.Version)
	require.NotNil(t, jk.Capabilities)
	require.True(t, jk.Capabilities.SupportsDAG)
	require.True(t, jk.Capabilities.SupportsLiveLog)
	require.False(t, jk.Capabilities.SupportsCaching)
}
