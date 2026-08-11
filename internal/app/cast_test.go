package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestCASTClientAuthenticationAndFullUpdate(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Header.Get("X-API-Key") != "secret" {
			t.Error("missing API-key header")
		}
		if r.Method == http.MethodPut {
			var update PolicyUpdate
			if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
				t.Fatal(err)
			}
			if len(update.AssignmentRules) == 0 || len(update.HPASettings) == 0 {
				t.Fatal("full update omitted managed fields")
			}
			w.WriteHeader(http.StatusOK)
			return
		}
		_, _ = w.Write([]byte(`{"id":"policy","name":"managed","applyType":"DEFERRED","recommendationPolicies":{"applyType":"DEFERRED"},"assignmentRules":[],"hpaSettings":{"enabled":true}}`))
	}))
	defer server.Close()
	client, err := NewCASTClient(server.URL, " secret\n")
	if err != nil {
		t.Fatal(err)
	}
	policy, err := client.GetPolicy(context.Background(), "cluster", "policy")
	if err != nil {
		t.Fatal(err)
	}
	update := PolicyUpdate{Name: policy.Name, ApplyType: policy.ApplyType, RecommendationPolicies: policy.RecommendationPolicies, AssignmentRules: policy.AssignmentRules, HPASettings: policy.HPASettings}
	if err := client.UpdatePolicy(context.Background(), "cluster", "policy", update); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("requests = %d", requests)
	}
}

func validPolicyResponse() string {
	return `{"id":"policy","name":"managed","applyType":"DEFERRED","recommendationPolicies":{"applyType":"DEFERRED"},"assignmentRules":[]}`
}

func TestCASTClientRetriesTransientResponses(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, validPolicyResponse())
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "secret")
	if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err != nil {
		t.Fatal(err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestCASTClientStopsAfterPersistentServerErrors(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "secret")
	if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err == nil || !strings.Contains(err.Error(), "HTTP 500") {
		t.Fatalf("error = %v", err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestCASTClientRetriesTransportFailures(t *testing.T) {
	requests := 0
	client, _ := NewCASTClient("https://api.invalid", "secret")
	client.http.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		requests++
		return nil, errors.New("network unavailable")
	})
	if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err == nil || !strings.Contains(err.Error(), "CAST request failed") {
		t.Fatalf("error = %v", err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestCASTClientDoesNotRetryPermanentHTTPFailures(t *testing.T) {
	for _, status := range []int{http.StatusForbidden, http.StatusNotFound} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			requests := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				requests++
				w.WriteHeader(status)
			}))
			defer server.Close()
			client, _ := NewCASTClient(server.URL, "secret")
			if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err == nil {
				t.Fatal("expected HTTP error")
			}
			if requests != 1 {
				t.Fatalf("requests = %d, want 1", requests)
			}
		})
	}
}

func TestCASTClientRetriesRateLimitAndStopsAfterThreeAttempts(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "secret")
	if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err == nil || !strings.Contains(err.Error(), "HTTP 429") {
		t.Fatalf("error = %v", err)
	}
	if requests != 3 {
		t.Fatalf("requests = %d, want 3", requests)
	}
}

func TestCASTClientDoesNotRetryPolicyUpdates(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPut {
			t.Fatalf("method = %s, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "secret")
	err := client.UpdatePolicy(context.Background(), "cluster", "policy", PolicyUpdate{
		Name: "managed", ApplyType: "IMMEDIATE", RecommendationPolicies: json.RawMessage(`{"applyType":"IMMEDIATE"}`), AssignmentRules: json.RawMessage(`[]`),
	})
	if err == nil || !strings.Contains(err.Error(), "HTTP 503") {
		t.Fatalf("error = %v", err)
	}
	if requests != 1 {
		t.Fatalf("update requests = %d, want 1", requests)
	}
}

func TestCASTClientDoesNotRetryPermanentErrorsOrExposeResponseBody(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `sensitive server detail`)
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "super-secret-key")
	_, err := client.GetPolicy(context.Background(), "cluster", "policy")
	if err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("error = %v", err)
	}
	if strings.Contains(err.Error(), "sensitive") || strings.Contains(err.Error(), "super-secret") {
		t.Fatalf("error leaks sensitive data: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1", requests)
	}
}

func TestCASTClientRejectsMalformedAndIncompleteResponses(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"malformed JSON", `{`, "decode CAST response"},
		{"missing required fields", `{"id":"policy"}`, "missing required fields"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, tc.body) }))
			defer server.Close()
			client, _ := NewCASTClient(server.URL, "secret")
			if _, err := client.GetPolicy(context.Background(), "cluster", "policy"); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestCASTClientHonorsCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()
	client, _ := NewCASTClient(server.URL, "secret")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := client.GetPolicy(ctx, "cluster", "policy")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context canceled", err)
	}
}

func TestNewCASTClientRejectsInvalidConfiguration(t *testing.T) {
	for _, tc := range []struct{ name, url, key string }{
		{"relative URL", "/api", "secret"},
		{"empty key", "https://api.cast.ai", "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewCASTClient(tc.url, tc.key); err == nil {
				t.Fatal("expected configuration error")
			}
		})
	}
}
