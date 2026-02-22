package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/kahook/internal/auth"
)

// maxBodyBytes is the maximum request body size accepted by the webhook handler (1 MiB).
const maxBodyBytes = 1 << 20 // 1 MiB

// produceTimeout is the per-request deadline for Kafka produce calls.
const produceTimeout = 10 * time.Second

// internalHeaders is the set of hop-by-hop / framework headers that are NOT
// forwarded to Kafka as message headers. Using a package-level map avoids
// allocating a new slice on every call to isInternalHeader.
var internalHeaders = map[string]bool{
	"authorization":   true,
	"content-type":    true,
	"content-length":  true,
	"host":            true,
	"user-agent":      true,
	"accept":          true,
	"accept-encoding": true,
	"connection":      true,
}

// KafkaProducer is the interface the server requires from a Kafka producer.
// Using an interface keeps the server decoupled from the concrete implementation
// and makes it straightforward to inject mocks in tests.
type KafkaProducer interface {
	Produce(ctx context.Context, topic string, key, value []byte, headers map[string]string) error
	IsConnected() bool
	Close()
}

// Server is the HTTP server that bridges incoming webhooks to Kafka.
type Server struct {
	httpServer *http.Server
	producer   KafkaProducer
	auth       *auth.MultiAuth
	logger     *zap.Logger
	metrics    *Metrics
}

// ServerConfig holds all dependencies and configuration needed to build a Server.
type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	Producer     KafkaProducer
	Auth         *auth.MultiAuth
	Logger       *zap.Logger
}

// ErrorResponse is the JSON body returned on errors.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// NewServer constructs and configures the HTTP server.
func NewServer(cfg ServerConfig) *Server {
	s := &Server{
		producer: cfg.Producer,
		auth:     cfg.Auth,
		logger:   cfg.Logger,
		metrics:  NewMetrics(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.healthHandler)
	mux.HandleFunc("/ready", s.readyHandler)
	mux.HandleFunc("/metrics", s.metricsHandler)
	mux.HandleFunc("/", s.webhookHandler)

	handler := RequestIDMiddleware(s.loggingMiddleware(mux))

	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	return s
}

// Start begins listening for HTTP requests. It blocks until the server stops.
func (s *Server) Start() error {
	s.logger.Info("starting server", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully drains in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down server")
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		s.metrics.IncrementRequests()

		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		if wrapped.statusCode >= 400 {
			s.metrics.IncrementError()
		} else {
			s.metrics.IncrementSuccess()
		}

		requestID := w.Header().Get(RequestIDHeader)

		s.logger.Info("request",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", wrapped.statusCode),
			zap.Duration("duration", time.Since(start)),
			zap.String("remote_addr", r.RemoteAddr),
			zap.String("request_id", requestID),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (s *Server) healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is allowed")
		return
	}
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
}

func (s *Server) readyHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is allowed")
		return
	}

	if s.producer == nil || !s.producer.IsConnected() {
		s.writeError(w, http.StatusServiceUnavailable, "not_ready", "kafka producer not available")
		return
	}

	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

// metricsHandler exposes internal runtime counters.
// It requires authentication to avoid leaking operational data publicly.
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only GET is allowed")
		return
	}

	if !s.auth.Authenticate(r) {
		s.writeUnauthorized(w, r)
		return
	}

	response := newMetricsSnapshot(s.metrics)
	s.writeJSON(w, http.StatusOK, response)
}

func (s *Server) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, http.StatusMethodNotAllowed, "method_not_allowed", "only POST is allowed")
		return
	}

	if !s.auth.Authenticate(r) {
		s.writeUnauthorized(w, r)
		return
	}

	topic := strings.Trim(r.URL.Path, "/")
	if topic == "" || strings.Contains(topic, "/") {
		s.writeError(w, http.StatusBadRequest, "invalid_topic", "topic name must be a single path segment")
		return
	}

	if topic == "health" || topic == "ready" || topic == "metrics" {
		s.writeError(w, http.StatusBadRequest, "reserved_topic", "cannot use reserved topic name")
		return
	}

	// Limit body size to prevent unbounded memory allocation from malicious senders.
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			s.writeError(w, http.StatusRequestEntityTooLarge, "body_too_large",
				fmt.Sprintf("request body exceeds maximum size of %d bytes", maxBodyBytes))
			return
		}
		s.writeError(w, http.StatusBadRequest, "read_error", "failed to read request body")
		return
	}
	defer r.Body.Close()

	if len(body) == 0 {
		s.writeError(w, http.StatusBadRequest, "empty_body", "request body cannot be empty")
		return
	}

	headers := make(map[string]string)
	for k, v := range r.Header {
		if !isInternalHeader(k) {
			headers[k] = v[0]
		}
	}

	webhookKey := r.Header.Get("X-Webhook-Key")
	var key []byte
	if webhookKey != "" {
		key = []byte(webhookKey)
	}

	// Apply a per-request produce timeout so a hung Kafka broker doesn't block
	// the HTTP handler indefinitely.
	produceCtx, cancel := context.WithTimeout(r.Context(), produceTimeout)
	defer cancel()

	if err := s.producer.Produce(produceCtx, topic, key, body, headers); err != nil {
		s.logger.Error("failed to produce message",
			zap.String("topic", topic),
			zap.Error(err),
		)
		s.writeError(w, http.StatusInternalServerError, "produce_error", "failed to send message to kafka")
		return
	}

	s.metrics.IncrementMessages()

	requestID := w.Header().Get(RequestIDHeader)

	s.logger.Info("webhook received",
		zap.String("topic", topic),
		zap.Int("size", len(body)),
		zap.String("remote_addr", r.RemoteAddr),
		zap.String("request_id", requestID),
	)

	s.writeJSON(w, http.StatusAccepted, map[string]string{
		"status":     "accepted",
		"topic":      topic,
		"request_id": requestID,
	})
}

// writeUnauthorized sends a 401 with the correct WWW-Authenticate header (RFC 7235).
// It inspects the request's Authorization header to determine which challenge to send.
func (s *Server) writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	authHeader := r.Header.Get("Authorization")
	parts := strings.SplitN(authHeader, " ", 2)

	scheme := ""
	if len(parts) >= 1 {
		scheme = strings.ToLower(parts[0])
	}

	switch scheme {
	case "bearer":
		w.Header().Set("WWW-Authenticate", `Bearer realm="kahook"`)
	default:
		// Default to Basic challenge — covers "basic", empty header, and unknown schemes.
		w.Header().Set("WWW-Authenticate", `Basic realm="kahook"`)
	}
	s.writeError(w, http.StatusUnauthorized, "unauthorized", "invalid or missing credentials")
}

func (s *Server) writeError(w http.ResponseWriter, code int, errorType, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errorType,
		Message: message,
	})
}

func (s *Server) writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func isInternalHeader(key string) bool {
	return internalHeaders[strings.ToLower(key)]
}
