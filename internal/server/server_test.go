package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kahook/internal/auth"
	"go.uber.org/zap"
)

// mockProducer satisfies the KafkaProducer interface for testing.
type mockProducer struct {
	produceErr error
	isHealthy  bool
}

func (m *mockProducer) Produce(_ context.Context, _ string, _, _ []byte, _ map[string]string) error {
	return m.produceErr
}

func (m *mockProducer) IsConnected() bool {
	return m.isHealthy
}

func (m *mockProducer) Close() {}

// setupTestServer builds a Server via NewServer so that all wiring is exercised.
func setupTestServer(a *auth.MultiAuth, producer KafkaProducer) *Server {
	return NewServer(ServerConfig{
		Port:     8080,
		Producer: producer,
		Auth:     a,
		Logger:   zap.NewNop(),
	})
}

// -------------------------------------------------------------------
// /health
// -------------------------------------------------------------------

func TestHealthHandler(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	srv.healthHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("healthHandler status = %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "healthy" {
		t.Errorf("healthHandler body status = %q, want %q", resp["status"], "healthy")
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/health", nil)
		w := httptest.NewRecorder()
		srv.healthHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("healthHandler %s = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

// -------------------------------------------------------------------
// /ready
// -------------------------------------------------------------------

func TestReadyHandler(t *testing.T) {
	tests := []struct {
		name       string
		isHealthy  bool
		wantStatus int
	}{
		{"healthy producer", true, http.StatusOK},
		{"unhealthy producer", false, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: tt.isHealthy})

			req := httptest.NewRequest(http.MethodGet, "/ready", nil)
			w := httptest.NewRecorder()
			srv.readyHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("readyHandler status = %d, want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

// -------------------------------------------------------------------
// /metrics
// -------------------------------------------------------------------

func TestMetricsHandler_Unauthenticated(t *testing.T) {
	// When auth is enabled, /metrics must return 401 without credentials.
	srv := setupTestServer(
		auth.NewMultiAuth(map[string]string{"admin": "secret"}, nil),
		&mockProducer{isHealthy: true},
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.metricsHandler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("metricsHandler without credentials = %d, want %d", w.Code, http.StatusUnauthorized)
	}

	// No Authorization header → default Basic challenge.
	wwwAuth := w.Header().Get("WWW-Authenticate")
	if !strings.HasPrefix(wwwAuth, "Basic") {
		t.Errorf("WWW-Authenticate = %q, want prefix %q", wwwAuth, "Basic")
	}
}

func TestMetricsHandler_Authenticated(t *testing.T) {
	srv := setupTestServer(
		auth.NewMultiAuth(map[string]string{"admin": "secret"}, nil),
		&mockProducer{isHealthy: true},
	)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.SetBasicAuth("admin", "secret")
	w := httptest.NewRecorder()
	srv.metricsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("metricsHandler authenticated = %d, want %d", w.Code, http.StatusOK)
	}

	var resp MetricsResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode metrics response: %v", err)
	}
	if resp.GoVersion == "" {
		t.Error("metrics GoVersion should not be empty")
	}
}

func TestMetricsHandler_NoneAuth(t *testing.T) {
	// With no auth configured the /metrics endpoint must be reachable without credentials.
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.metricsHandler(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("metricsHandler none-auth = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestMetricsHandler_MethodNotAllowed(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	req := httptest.NewRequest(http.MethodPost, "/metrics", nil)
	w := httptest.NewRecorder()
	srv.metricsHandler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("metricsHandler POST = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

// -------------------------------------------------------------------
// webhookHandler — method
// -------------------------------------------------------------------

func TestWebhookHandler_MethodNotAllowed(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch} {
		req := httptest.NewRequest(method, "/test-topic", nil)
		w := httptest.NewRecorder()
		srv.webhookHandler(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("webhookHandler %s = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}

// -------------------------------------------------------------------
// webhookHandler — authentication & WWW-Authenticate header
// -------------------------------------------------------------------

func TestWebhookHandler_Authentication(t *testing.T) {
	tests := []struct {
		name        string
		auth        *auth.MultiAuth
		setupReq    func(*http.Request)
		wantStatus  int
		wantWWWAuth string // non-empty → assert header prefix
	}{
		{
			name:       "no auth configured passes through",
			auth:       auth.NewMultiAuth(nil, nil),
			setupReq:   func(r *http.Request) {},
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "basic auth valid",
			auth:       auth.NewMultiAuth(map[string]string{"admin": "password"}, nil),
			setupReq:   func(r *http.Request) { r.SetBasicAuth("admin", "password") },
			wantStatus: http.StatusAccepted,
		},
		{
			name:        "basic auth invalid — WWW-Authenticate must be Basic",
			auth:        auth.NewMultiAuth(map[string]string{"admin": "password"}, nil),
			setupReq:    func(r *http.Request) { r.SetBasicAuth("admin", "wrong") },
			wantStatus:  http.StatusUnauthorized,
			wantWWWAuth: "Basic",
		},
		{
			name:       "bearer auth valid",
			auth:       auth.NewMultiAuth(nil, []string{"token123"}),
			setupReq:   func(r *http.Request) { r.Header.Set("Authorization", "Bearer token123") },
			wantStatus: http.StatusAccepted,
		},
		{
			name:        "bearer auth invalid — WWW-Authenticate must be Bearer",
			auth:        auth.NewMultiAuth(nil, []string{"token123"}),
			setupReq:    func(r *http.Request) { r.Header.Set("Authorization", "Bearer wrong") },
			wantStatus:  http.StatusUnauthorized,
			wantWWWAuth: "Bearer",
		},
		{
			name:        "no header with auth configured — default Basic challenge",
			auth:        auth.NewMultiAuth(map[string]string{"admin": "password"}, []string{"token123"}),
			setupReq:    func(r *http.Request) {},
			wantStatus:  http.StatusUnauthorized,
			wantWWWAuth: "Basic",
		},
		{
			name:       "multi-auth: basic creds when both configured",
			auth:       auth.NewMultiAuth(map[string]string{"admin": "password"}, []string{"token123"}),
			setupReq:   func(r *http.Request) { r.SetBasicAuth("admin", "password") },
			wantStatus: http.StatusAccepted,
		},
		{
			name:       "multi-auth: bearer token when both configured",
			auth:       auth.NewMultiAuth(map[string]string{"admin": "password"}, []string{"token123"}),
			setupReq:   func(r *http.Request) { r.Header.Set("Authorization", "Bearer token123") },
			wantStatus: http.StatusAccepted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupTestServer(tt.auth, &mockProducer{isHealthy: true})

			body := bytes.NewBufferString(`{"test": "data"}`)
			req := httptest.NewRequest(http.MethodPost, "/test-topic", body)
			tt.setupReq(req)
			w := httptest.NewRecorder()

			srv.webhookHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("webhookHandler status = %d, want %d", w.Code, tt.wantStatus)
			}

			if tt.wantWWWAuth != "" {
				got := w.Header().Get("WWW-Authenticate")
				if !strings.HasPrefix(got, tt.wantWWWAuth) {
					t.Errorf("WWW-Authenticate = %q, want prefix %q", got, tt.wantWWWAuth)
				}
			}
		})
	}
}

// -------------------------------------------------------------------
// webhookHandler — topic validation
// -------------------------------------------------------------------

func TestWebhookHandler_TopicValidation(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		wantStatus int
	}{
		{"valid topic", "/my-topic", http.StatusAccepted},
		{"valid topic with dots", "/my.nested.topic", http.StatusAccepted},
		{"empty path", "/", http.StatusBadRequest},
		{"reserved health", "/health", http.StatusBadRequest},
		{"reserved ready", "/ready", http.StatusBadRequest},
		{"reserved metrics", "/metrics", http.StatusBadRequest},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

			body := bytes.NewBufferString(`{"test": "data"}`)
			req := httptest.NewRequest(http.MethodPost, tt.path, body)
			w := httptest.NewRecorder()

			srv.webhookHandler(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("path=%s status = %d, want %d", tt.path, w.Code, tt.wantStatus)
			}
		})
	}
}

// -------------------------------------------------------------------
// webhookHandler — body validation
// -------------------------------------------------------------------

func TestWebhookHandler_EmptyBody(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	req := httptest.NewRequest(http.MethodPost, "/test-topic", nil)
	w := httptest.NewRecorder()

	srv.webhookHandler(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("empty body status = %d, want %d", w.Code, http.StatusBadRequest)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "empty_body" {
		t.Errorf("error type = %q, want %q", resp.Error, "empty_body")
	}
}

func TestWebhookHandler_BodyTooLarge(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	// Send a body that exceeds maxBodyBytes (1 MiB).
	oversized := strings.Repeat("x", maxBodyBytes+1)
	req := httptest.NewRequest(http.MethodPost, "/test-topic", strings.NewReader(oversized))
	w := httptest.NewRecorder()

	srv.webhookHandler(w, req)

	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized body status = %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
	}

	var resp ErrorResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Error != "body_too_large" {
		t.Errorf("error type = %q, want %q", resp.Error, "body_too_large")
	}
}

// -------------------------------------------------------------------
// webhookHandler — success
// -------------------------------------------------------------------

func TestWebhookHandler_Success(t *testing.T) {
	srv := setupTestServer(auth.NewMultiAuth(nil, nil), &mockProducer{isHealthy: true})

	body := bytes.NewBufferString(`{"event": "test", "data": {"id": 123}}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook-events", body)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Custom-Header", "custom-value")
	w := httptest.NewRecorder()

	srv.webhookHandler(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("success status = %d, want %d", w.Code, http.StatusAccepted)
	}

	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "accepted" {
		t.Errorf("status = %q, want %q", resp["status"], "accepted")
	}
	if resp["topic"] != "webhook-events" {
		t.Errorf("topic = %q, want %q", resp["topic"], "webhook-events")
	}
}

// -------------------------------------------------------------------
// NewServer — via ServerConfig (producer interface injection)
// -------------------------------------------------------------------

func TestNewServer_UsesInterface(t *testing.T) {
	// Verifies that NewServer accepts any KafkaProducer implementation,
	// proving the interface decoupling works end-to-end.
	mock := &mockProducer{isHealthy: true}
	srv := NewServer(ServerConfig{
		Port:     9090,
		Producer: mock,
		Auth:     auth.NewMultiAuth(nil, nil),
		Logger:   zap.NewNop(),
	})
	if srv == nil {
		t.Fatal("NewServer returned nil")
	}
}

// -------------------------------------------------------------------
// isInternalHeader
// -------------------------------------------------------------------

func TestIsInternalHeader(t *testing.T) {
	tests := []struct {
		header     string
		isInternal bool
	}{
		{"Authorization", true},
		{"authorization", true},
		{"AUTHORIZATION", true},
		{"Content-Type", true},
		{"Content-Length", true},
		{"Host", true},
		{"User-Agent", true},
		{"X-Custom-Header", false},
		{"X-GitHub-Event", false},
		{"Stripe-Signature", false},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			if got := isInternalHeader(tt.header); got != tt.isInternal {
				t.Errorf("isInternalHeader(%q) = %v, want %v", tt.header, got, tt.isInternal)
			}
		})
	}
}
