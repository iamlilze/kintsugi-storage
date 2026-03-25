package observability

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	putRequests       prometheus.Counter
	getRequests       prometheus.Counter
	hits              prometheus.Counter
	misses            prometheus.Counter
	expiredDeletions  prometheus.Counter
	objectsTotal      prometheus.Gauge
	snapshotDuration  prometheus.Histogram
	snapshotSizeBytes prometheus.Gauge
}

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
			Help: "Current number of non-expired objects stored.",
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

func (m *Metrics) IncPutRequests() { m.putRequests.Inc() }
func (m *Metrics) IncGetRequests() { m.getRequests.Inc() }
func (m *Metrics) IncHits()        { m.hits.Inc() }
func (m *Metrics) IncMisses()      { m.misses.Inc() }

func (m *Metrics) AddExpiredDeletions(n int) {
	if n > 0 {
		m.expiredDeletions.Add(float64(n))
	}
}

func (m *Metrics) SetObjectsTotal(n int) { m.objectsTotal.Set(float64(n)) }

func (m *Metrics) ObserveSnapshotDuration(seconds float64) {
	m.snapshotDuration.Observe(seconds)
}

func (m *Metrics) SetSnapshotSizeBytes(n int) {
	if n >= 0 {
		m.snapshotSizeBytes.Set(float64(n))
	}
}
