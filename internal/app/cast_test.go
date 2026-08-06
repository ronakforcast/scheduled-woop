package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
