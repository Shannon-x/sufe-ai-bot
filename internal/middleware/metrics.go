package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Message metrics
	messagesReceived = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_messages_received_total",
		Help: "Total number of messages received",
	}, []string{"chat_type"})

	messagesProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_messages_processed_total",
		Help: "Total number of messages processed",
	}, []string{"status"})

	// Command metrics
	commandsExecuted = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_commands_executed_total",
		Help: "Total number of commands executed",
	}, []string{"command"})

	// AI metrics
	aiRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "telegram_bot_ai_request_duration_seconds",
		Help:    "Duration of AI requests",
		Buckets: prometheus.DefBuckets,
	}, []string{"model", "status"})

	aiRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_ai_requests_total",
		Help: "Total number of AI requests",
	}, []string{"model", "status"})

	// Cache metrics
	cacheHits = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telegram_bot_cache_hits_total",
		Help: "Total number of cache hits",
	})

	cacheMisses = promauto.NewCounter(prometheus.CounterOpts{
		Name: "telegram_bot_cache_misses_total",
		Help: "Total number of cache misses",
	})

	// Rate limit metrics
	rateLimitExceeded = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_rate_limit_exceeded_total",
		Help: "Total number of rate limit exceeded events",
	}, []string{"user_id"})

	// Storage metrics
	storageOperations = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "telegram_bot_storage_operations_total",
		Help: "Total number of storage operations",
	}, []string{"operation", "status"})

	storageOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "telegram_bot_storage_operation_duration_seconds",
		Help:    "Duration of storage operations",
		Buckets: prometheus.DefBuckets,
	}, []string{"operation"})

	// Active users gauge
	activeUsers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telegram_bot_active_users",
		Help: "Number of active users",
	})

	// Active chats gauge
	activeChats = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "telegram_bot_active_chats",
		Help: "Number of active chats",
	})
)

// Metrics provides methods to record metrics
type Metrics struct{}

// NewMetrics creates a new metrics instance
func NewMetrics() *Metrics {
	return &Metrics{}
}

// RecordMessageReceived records a received message
func (m *Metrics) RecordMessageReceived(chatType string) {
	messagesReceived.WithLabelValues(chatType).Inc()
}

// RecordMessageProcessed records a processed message
func (m *Metrics) RecordMessageProcessed(status string) {
	messagesProcessed.WithLabelValues(status).Inc()
}

// RecordCommandExecuted records an executed command
func (m *Metrics) RecordCommandExecuted(command string) {
	commandsExecuted.WithLabelValues(command).Inc()
}

// RecordAIRequest records an AI request
func (m *Metrics) RecordAIRequest(model, status string, duration time.Duration) {
	aiRequestDuration.WithLabelValues(model, status).Observe(duration.Seconds())
	aiRequestsTotal.WithLabelValues(model, status).Inc()
}

// RecordCacheHit records a cache hit
func (m *Metrics) RecordCacheHit() {
	cacheHits.Inc()
}

// RecordCacheMiss records a cache miss
func (m *Metrics) RecordCacheMiss() {
	cacheMisses.Inc()
}

// RecordRateLimitExceeded records a rate limit exceeded event
func (m *Metrics) RecordRateLimitExceeded(userID string) {
	rateLimitExceeded.WithLabelValues(userID).Inc()
}

// RecordStorageOperation records a storage operation
func (m *Metrics) RecordStorageOperation(operation, status string, duration time.Duration) {
	storageOperations.WithLabelValues(operation, status).Inc()
	storageOperationDuration.WithLabelValues(operation).Observe(duration.Seconds())
}

// SetActiveUsers sets the number of active users
func (m *Metrics) SetActiveUsers(count float64) {
	activeUsers.Set(count)
}

// SetActiveChats sets the number of active chats
func (m *Metrics) SetActiveChats(count float64) {
	activeChats.Set(count)
}

// StartMetricsServer starts the metrics HTTP server
func StartMetricsServer(port int, path string) error {
	router := mux.NewRouter()
	router.Handle(path, promhttp.Handler())
	
	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	addr := fmt.Sprintf(":%d", port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return server.ListenAndServe()
}