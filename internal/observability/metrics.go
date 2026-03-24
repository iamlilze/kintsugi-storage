package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

// Metrics хранит все прикладные метрики сервиса.
type Metrics struct {
	putRequests       prometheus.Counter   // Счетчик PUT-запросов.
	getRequests       prometheus.Counter   // Счетчик GET-запросов.
	hits              prometheus.Counter   // Счетчик успешных чтений.
	misses            prometheus.Counter   // Счетчик неуспешных чтений.
	expiredDeletions  prometheus.Counter   // Счетчик объектов, удаленных TTL cleanup'ом.
	objectsTotal      prometheus.Gauge     // Текущее число живых объектов в storage.
	snapshotDuration  prometheus.Histogram // Длительность сохранения snapshot.
	snapshotSizeBytes prometheus.Gauge     // Размер последнего snapshot в байтах.
}

// NewMetrics создает набор метрик и регистрирует их в переданном registry.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	if reg == nil {
		reg = prometheus.DefaultRegisterer
	}

	m := &Metrics{
		putRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "storage_put_requests_total",
			Help: "Total number of PUT /objects requests.",
		}),
		getRequests: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "storage_get_requests_total",
			Help: "Total number of GET /objects requests.",
		}),
		hits: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "storage_hits_total",
			Help: "Total number of successful object lookups.",
		}),
		misses: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "storage_misses_total",
			Help: "Total number of failed object lookups.",
		}),
		expiredDeletions: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "storage_expired_deletions_total",
			Help: "Total number of objects deleted because their TTL expired.",
		}),
		objectsTotal: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "storage_objects_total",
			Help: "Current number of non-expired objects stored in memory.",
		}),
		snapshotDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name:    "storage_snapshot_duration_seconds",
			Help:    "Duration of snapshot save operations in seconds.",
			Buckets: prometheus.DefBuckets,
		}),
		snapshotSizeBytes: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "storage_snapshot_size_bytes",
			Help: "Size of the last saved snapshot in bytes.",
		}),
	}

	reg.MustRegister(
		m.putRequests,
		m.getRequests,
		m.hits,
		m.misses,
		m.expiredDeletions,
		m.objectsTotal,
		m.snapshotDuration,
		m.snapshotSizeBytes,
	)

	return m
}

// IncPutRequests увеличивает счетчик PUT-запросов.
func (m *Metrics) IncPutRequests() {
	if m == nil {
		return
	}

	m.putRequests.Inc()
}

// IncGetRequests увеличивает счетчик GET-запросов.
func (m *Metrics) IncGetRequests() {
	if m == nil {
		return
	}

	m.getRequests.Inc()
}

// IncHits увеличивает счетчик успешных чтений.
func (m *Metrics) IncHits() {
	if m == nil {
		return
	}

	m.hits.Inc()
}

// IncMisses увеличивает счетчик неуспешных чтений.
func (m *Metrics) IncMisses() {
	if m == nil {
		return
	}

	m.misses.Inc()
}

// AddExpiredDeletions увеличивает счетчик удалений по TTL на n.
func (m *Metrics) AddExpiredDeletions(n int) {
	if m == nil || n <= 0 {
		return
	}

	m.expiredDeletions.Add(float64(n))
}

// SetObjectsTotal обновляет текущее количество живых объектов.
func (m *Metrics) SetObjectsTotal(n int) {
	if m == nil {
		return
	}

	m.objectsTotal.Set(float64(n))
}

// ObserveSnapshotDuration записывает длительность сохранения snapshot.
func (m *Metrics) ObserveSnapshotDuration(seconds float64) {
	if m == nil {
		return
	}

	m.snapshotDuration.Observe(seconds)
}

// SetSnapshotSizeBytes обновляет размер последнего snapshot.
func (m *Metrics) SetSnapshotSizeBytes(n int) {
	if m == nil || n < 0 {
		return
	}

	m.snapshotSizeBytes.Set(float64(n))
}
