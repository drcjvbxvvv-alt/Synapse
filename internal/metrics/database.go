package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

// DBMetrics holds Prometheus metrics for the GORM database layer.
type DBMetrics struct {
	QueryDuration *prometheus.HistogramVec
	SlowQueries   *prometheus.CounterVec
	ErrorsTotal   *prometheus.CounterVec
}

const slowQueryThreshold = 500 * time.Millisecond

func newDBMetrics(reg prometheus.Registerer) *DBMetrics {
	m := &DBMetrics{
		QueryDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "synapse",
			Name:      "db_query_duration_seconds",
			Help:      "GORM query latency in seconds.",
			Buckets:   []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		}, []string{"operation"}),

		SlowQueries: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "db_slow_queries_total",
			Help:      "Total number of slow GORM queries (>500ms).",
		}, []string{"operation"}),

		ErrorsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "synapse",
			Name:      "db_errors_total",
			Help:      "Total number of GORM query errors.",
		}, []string{"operation"}),
	}
	reg.MustRegister(m.QueryDuration, m.SlowQueries, m.ErrorsTotal)
	return m
}

// Register attaches Before/After GORM callbacks for the four core operations.
func (m *DBMetrics) Register(db *gorm.DB) {
	m.registerQuery(db)
	m.registerCreate(db)
	m.registerUpdate(db)
	m.registerDelete(db)
}

func (m *DBMetrics) makeAfterFn(op string) func(*gorm.DB) {
	startKey := "synapse:metrics:start:" + op
	return func(tx *gorm.DB) {
		v, ok := tx.InstanceGet(startKey)
		if !ok {
			return
		}
		dur := time.Since(v.(time.Time))
		m.QueryDuration.WithLabelValues(op).Observe(dur.Seconds())
		if dur >= slowQueryThreshold {
			m.SlowQueries.WithLabelValues(op).Inc()
		}
		if tx.Error != nil && tx.Error != gorm.ErrRecordNotFound {
			m.ErrorsTotal.WithLabelValues(op).Inc()
		}
	}
}

func (m *DBMetrics) makeBeforeFn(op string) func(*gorm.DB) {
	startKey := "synapse:metrics:start:" + op
	return func(tx *gorm.DB) { tx.InstanceSet(startKey, time.Now()) }
}

func (m *DBMetrics) registerQuery(db *gorm.DB) {
	_ = db.Callback().Query().Before("gorm:query").Register("synapse:metrics:before:query", m.makeBeforeFn("query"))
	_ = db.Callback().Query().After("gorm:after_query").Register("synapse:metrics:after:query", m.makeAfterFn("query"))
}

func (m *DBMetrics) registerCreate(db *gorm.DB) {
	_ = db.Callback().Create().Before("gorm:begin_transaction").Register("synapse:metrics:before:create", m.makeBeforeFn("create"))
	_ = db.Callback().Create().After("gorm:commit_or_rollback_transaction").Register("synapse:metrics:after:create", m.makeAfterFn("create"))
}

func (m *DBMetrics) registerUpdate(db *gorm.DB) {
	_ = db.Callback().Update().Before("gorm:begin_transaction").Register("synapse:metrics:before:update", m.makeBeforeFn("update"))
	_ = db.Callback().Update().After("gorm:commit_or_rollback_transaction").Register("synapse:metrics:after:update", m.makeAfterFn("update"))
}

func (m *DBMetrics) registerDelete(db *gorm.DB) {
	_ = db.Callback().Delete().Before("gorm:begin_transaction").Register("synapse:metrics:before:delete", m.makeBeforeFn("delete"))
	_ = db.Callback().Delete().After("gorm:commit_or_rollback_transaction").Register("synapse:metrics:after:delete", m.makeAfterFn("delete"))
}
