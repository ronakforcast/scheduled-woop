package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestObservabilityHealthAndReadiness(t *testing.T) {
	metrics := NewMetrics()
	handler := metrics.Handler()

	request := func(path string) *httptest.ResponseRecorder {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		return recorder
	}
	if got := request("/healthz").Code; got != http.StatusOK {
		t.Fatalf("health status = %d, want 200", got)
	}
	if got := request("/readyz").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("initial readiness = %d, want 503", got)
	}
	metrics.SetReady(true)
	if got := request("/readyz").Code; got != http.StatusOK {
		t.Fatalf("ready status = %d, want 200", got)
	}
	metrics.SetReady(false)
	if got := request("/readyz").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("stopped readiness = %d, want 503", got)
	}
}

func TestObservabilityMetricsUsePrometheusTextFormat(t *testing.T) {
	metrics := NewMetrics()
	metrics.RecordReconcile(nil, time.Unix(1_700_000_000, 0))
	metrics.RecordReconcile(assertionError("failed"), time.Unix(1_700_000_100, 0))
	metrics.RecordPolicyUpdate()
	recorder := httptest.NewRecorder()
	metrics.Handler().ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`scheduled_woop_reconciliations_total{result="success"} 1`,
		`scheduled_woop_reconciliations_total{result="failure"} 1`,
		`scheduled_woop_policy_updates_total 1`,
		`scheduled_woop_last_success_unixtime 1700000000`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metrics missing %q:\n%s", expected, body)
		}
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "text/plain") {
		t.Fatalf("content type = %q", contentType)
	}
}

type assertionError string

func (e assertionError) Error() string { return string(e) }
