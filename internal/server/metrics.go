package server

import (
	"runtime"
	"sync/atomic"
	"time"
)

// Metrics holds atomic counters and start time for the server.
type Metrics struct {
	StartTime        time.Time
	RequestsTotal    atomic.Int64
	RequestsSuccess  atomic.Int64
	RequestsError    atomic.Int64
	MessagesProduced atomic.Int64
}

func NewMetrics() *Metrics {
	return &Metrics{
		StartTime: time.Now(),
	}
}

func (m *Metrics) IncrementRequests() {
	m.RequestsTotal.Add(1)
}

func (m *Metrics) IncrementSuccess() {
	m.RequestsSuccess.Add(1)
}

func (m *Metrics) IncrementError() {
	m.RequestsError.Add(1)
}

func (m *Metrics) IncrementMessages() {
	m.MessagesProduced.Add(1)
}

// MetricsResponse is the JSON-serialisable snapshot returned by /metrics.
type MetricsResponse struct {
	Uptime           string `json:"uptime"`
	RequestsTotal    int64  `json:"requests_total"`
	RequestsSuccess  int64  `json:"requests_success"`
	RequestsError    int64  `json:"requests_error"`
	MessagesProduced int64  `json:"messages_produced"`
	GoVersion        string `json:"go_version"`
	Goroutines       int    `json:"goroutines"`
}

// newMetricsSnapshot builds a point-in-time snapshot from the live Metrics.
func newMetricsSnapshot(m *Metrics) MetricsResponse {
	return MetricsResponse{
		Uptime:           time.Since(m.StartTime).String(),
		RequestsTotal:    m.RequestsTotal.Load(),
		RequestsSuccess:  m.RequestsSuccess.Load(),
		RequestsError:    m.RequestsError.Load(),
		MessagesProduced: m.MessagesProduced.Load(),
		GoVersion:        runtime.Version(),
		Goroutines:       runtime.NumGoroutine(),
	}
}
