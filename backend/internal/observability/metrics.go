// Package observability contains low-level, opt-in telemetry boundaries.
package observability

import (
	"fmt"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// Metrics records bounded HTTP request counters and duration summaries. It is
// intentionally dependency-free so the daemon can expose useful development
// metrics before an OTLP collector is configured.
type Metrics struct {
	requests      atomic.Uint64
	serverError   atomic.Uint64
	durationNS    atomic.Uint64
	byMethod      [len(metricMethods)]atomic.Uint64
	byStatusClass [len(metricStatusClasses)]atomic.Uint64
}

var metricMethods = [...]string{"GET", "POST", "PUT", "PATCH", "DELETE", "OTHER"}
var metricStatusClasses = [...]string{"1xx", "2xx", "3xx", "4xx", "5xx"}

// NewMetrics returns an empty request metrics collector.
func NewMetrics() *Metrics { return &Metrics{} }

// Middleware records method-independent request totals and status classes.
// Request paths are deliberately not labels, preventing unbounded cardinality.
func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		rw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		m.requests.Add(1)
		m.durationNS.Add(uint64(time.Since(started).Nanoseconds()))
		m.byMethod[methodIndex(r.Method)].Add(1)
		m.byStatusClass[statusClassIndex(rw.status)].Add(1)
		if rw.status >= 500 {
			m.serverError.Add(1)
		}
	})
}

// Handler exposes the stable Prometheus text exposition format.
func (m *Metrics) Handler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	requests := m.requests.Load()
	seconds := float64(m.durationNS.Load()) / float64(time.Second)
	fmt.Fprintf(w, "# HELP porter_http_requests_total Total HTTP requests handled by Porter.\n# TYPE porter_http_requests_total counter\nporter_http_requests_total %d\n", requests)
	fmt.Fprintln(w, "# HELP porter_http_requests_by_method_total Total HTTP requests by bounded method label.")
	fmt.Fprintln(w, "# TYPE porter_http_requests_by_method_total counter")
	for i, method := range metricMethods {
		fmt.Fprintf(w, "porter_http_requests_by_method_total{method=\"%s\"} %d\n", method, m.byMethod[i].Load())
	}
	fmt.Fprintln(w, "# HELP porter_http_responses_by_status_class_total Total HTTP responses by bounded status class label.")
	fmt.Fprintln(w, "# TYPE porter_http_responses_by_status_class_total counter")
	for i, statusClass := range metricStatusClasses {
		fmt.Fprintf(w, "porter_http_responses_by_status_class_total{status_class=\"%s\"} %d\n", statusClass, m.byStatusClass[i].Load())
	}
	fmt.Fprintf(w, "# HELP porter_http_server_errors_total Total HTTP responses with status 500 or higher.\n# TYPE porter_http_server_errors_total counter\nporter_http_server_errors_total %d\n", m.serverError.Load())
	fmt.Fprintf(w, "# HELP porter_http_request_duration_seconds_sum Sum of HTTP request durations in seconds.\n# TYPE porter_http_request_duration_seconds_sum counter\nporter_http_request_duration_seconds_sum %s\n", strconv.FormatFloat(seconds, 'f', 6, 64))
	fmt.Fprintf(w, "# HELP porter_http_request_duration_seconds_count Count of HTTP request durations.\n# TYPE porter_http_request_duration_seconds_count counter\nporter_http_request_duration_seconds_count %d\n", requests)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func methodIndex(method string) int {
	for i := 0; i < len(metricMethods)-1; i++ {
		if metricMethods[i] == method {
			return i
		}
	}
	return len(metricMethods) - 1
}

func statusClassIndex(status int) int {
	if status < 100 || status >= 600 {
		return len(metricStatusClasses) - 1
	}
	index := status/100 - 1
	if index < 0 {
		return 0
	}
	if index >= len(metricStatusClasses) {
		return len(metricStatusClasses) - 1
	}
	return index
}
