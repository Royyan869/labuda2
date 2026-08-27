package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
)

const (
	realtimeMetricsNamespace = "labuda_realtime"
	chatMetricsNamespace     = "labuda_chat"
)

// RealtimeMetrics holds metrics for realtime/WebSocket operations.
// Thread-safe: prometheus metrics are atomic.
type RealtimeMetrics struct {
	// ActiveConnections tracks current WebSocket connections
	ActiveConnections prometheus.Gauge

	// ActiveRooms tracks currently active rooms (rooms with >=1 connection)
	ActiveRooms prometheus.Gauge

	// BroadcastTotal counts total broadcasts sent
	BroadcastTotal prometheus.Counter

	// BroadcastDuration tracks broadcast latency in seconds
	BroadcastDuration prometheus.Histogram

	// ChatMessagesTotal counts total chat messages sent
	ChatMessagesTotal prometheus.Counter

	// ChatRateLimitedTotal counts rate-limited chat attempts
	ChatRateLimitedTotal prometheus.Counter
}

// NewRealtimeMetrics creates and registers realtime metrics.
func NewRealtimeMetrics() *RealtimeMetrics {
	buckets := []float64{0.001, 0.005, 0.01, 0.02, 0.05, 0.1, 0.2}

	return &RealtimeMetrics{
		ActiveConnections: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: realtimeMetricsNamespace,
			Name:      "active_connections",
			Help:      "Current number of active WebSocket connections.",
		}),
		ActiveRooms: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: realtimeMetricsNamespace,
			Name:      "active_rooms",
			Help:      "Current number of active rooms (rooms with at least one connection).",
		}),
		BroadcastTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: realtimeMetricsNamespace,
			Name:      "broadcast_total",
			Help:      "Total number of broadcasts sent to rooms.",
		}),
		BroadcastDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: realtimeMetricsNamespace,
			Name:      "broadcast_duration_seconds",
			Help:      "Duration of broadcast operations in seconds.",
			Buckets:   buckets,
		}),
		ChatMessagesTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: chatMetricsNamespace,
			Name:      "messages_total",
			Help:      "Total number of chat messages sent successfully.",
		}),
		ChatRateLimitedTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: chatMetricsNamespace,
			Name:      "rate_limited_total",
			Help:      "Total number of chat operations rejected due to rate limiting.",
		}),
	}
}

// Describe implements prometheus.Collector.Describe
func (rm *RealtimeMetrics) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(rm, ch)
}

// Collect implements prometheus.Collector.Collect
func (rm *RealtimeMetrics) Collect(ch chan<- prometheus.Metric) {
	ch <- rm.ActiveConnections
	ch <- rm.ActiveRooms
	ch <- rm.BroadcastTotal
	ch <- rm.BroadcastDuration
	ch <- rm.ChatMessagesTotal
	ch <- rm.ChatRateLimitedTotal
}

// IncrementActiveConnections increments the active connections gauge.
func (rm *RealtimeMetrics) IncrementActiveConnections() {
	rm.ActiveConnections.Inc()
}

// DecrementActiveConnections decrements the active connections gauge.
func (rm *RealtimeMetrics) DecrementActiveConnections() {
	rm.ActiveConnections.Dec()
}

// SetActiveRooms sets the active rooms gauge.
func (rm *RealtimeMetrics) SetActiveRooms(count int) {
	rm.ActiveRooms.Set(float64(count))
}

// RecordBroadcast records a broadcast operation.
func (rm *RealtimeMetrics) RecordBroadcast(durationSeconds float64) {
	rm.BroadcastTotal.Inc()
	rm.BroadcastDuration.Observe(durationSeconds)
}

// RecordChatMessage records a successfully sent chat message.
func (rm *RealtimeMetrics) RecordChatMessage() {
	rm.ChatMessagesTotal.Inc()
}

// RecordChatRateLimited records a rate-limited chat operation.
func (rm *RealtimeMetrics) RecordChatRateLimited() {
	rm.ChatRateLimitedTotal.Inc()
}


