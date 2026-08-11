package app

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type Metrics struct {
	ready               atomic.Bool
	reconcileSuccess    atomic.Uint64
	reconcileFailure    atomic.Uint64
	policyUpdates       atomic.Uint64
	lastSuccessUnixTime atomic.Int64
}

func NewMetrics() *Metrics { return &Metrics{} }

func (m *Metrics) SetReady(ready bool) { m.ready.Store(ready) }

func (m *Metrics) RecordReconcile(err error, now time.Time) {
	if err != nil {
		m.reconcileFailure.Add(1)
		return
	}
	m.reconcileSuccess.Add(1)
	m.lastSuccessUnixTime.Store(now.Unix())
}

func (m *Metrics) RecordPolicyUpdate() { m.policyUpdates.Add(1) }

func (m *Metrics) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		if !m.ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready\n"))
			return
		}
		_, _ = w.Write([]byte("ready\n"))
	})
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprintf(w, "# TYPE scheduled_woop_reconciliations_total counter\n")
		_, _ = fmt.Fprintf(w, "scheduled_woop_reconciliations_total{result=\"success\"} %d\n", m.reconcileSuccess.Load())
		_, _ = fmt.Fprintf(w, "scheduled_woop_reconciliations_total{result=\"failure\"} %d\n", m.reconcileFailure.Load())
		_, _ = fmt.Fprintf(w, "# TYPE scheduled_woop_policy_updates_total counter\n")
		_, _ = fmt.Fprintf(w, "scheduled_woop_policy_updates_total %d\n", m.policyUpdates.Load())
		_, _ = fmt.Fprintf(w, "# TYPE scheduled_woop_last_success_unixtime gauge\n")
		_, _ = fmt.Fprintf(w, "scheduled_woop_last_success_unixtime %d\n", m.lastSuccessUnixTime.Load())
	})
	return mux
}
