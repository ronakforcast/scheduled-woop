package app

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func baseConfig() Config {
	return Config{
		ClusterID: "cluster", Timezone: "Europe/Prague", PollInterval: time.Minute, ApplyType: "DEFERRED",
		Schedules: []PolicySchedule{{
			Name: "application-one", ManagedPolicyID: "managed",
			DefaultProfile: Profile{Name: "safe", PolicyID: "safe-id"},
			Windows:        []Window{{Name: "business", Days: []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}, Start: "08:00", End: "18:00", Profile: Profile{Name: "performance", PolicyID: "performance-id"}}},
		}},
	}
}

func TestActiveProfileOrdinaryAndOvernight(t *testing.T) {
	config := baseConfig()
	cases := []struct{ instant, want string }{
		{"2026-08-03T09:00:00+02:00", "performance"},
		{"2026-08-03T19:00:00+02:00", "safe"},
		{"2026-08-08T10:00:00+02:00", "safe"},
	}
	for _, item := range cases {
		instant, _ := time.Parse(time.RFC3339, item.instant)
		if got := config.Schedules[0].ActiveProfile(instant, config.Timezone).Name; got != item.want {
			t.Errorf("%s: got %s, want %s", item.instant, got, item.want)
		}
	}
	config.Schedules[0].Windows = []Window{{Name: "night", Days: []string{"Monday"}, Start: "18:00", End: "08:00", Profile: Profile{Name: "night", PolicyID: "night-id"}}}
	instant, _ := time.Parse(time.RFC3339, "2026-08-04T07:00:00+02:00")
	if got := config.Schedules[0].ActiveProfile(instant, config.Timezone).Name; got != "night" {
		t.Fatalf("overnight profile = %s", got)
	}
}

func TestValidationRejectsOverlapsAndImmediate(t *testing.T) {
	config := baseConfig()
	config.Schedules[0].Windows = append(config.Schedules[0].Windows, config.Schedules[0].Windows[0])
	config.Schedules[0].Windows[1].Name = "duplicate-time"
	if err := config.Validate(); err == nil {
		t.Fatal("expected overlap error")
	}
	config = baseConfig()
	config.ApplyType = "IMMEDIATE"
	if err := config.Validate(); err == nil {
		t.Fatal("expected immediate rejection")
	}
}

func TestLoadConfigWithMultipleSchedules(t *testing.T) {
	content := `clusterId: cluster
timezone: Europe/Prague
pollInterval: 30s
applyType: DEFERRED
schedules:
  - name: one
    managedPolicyId: managed-a
    defaultProfile: {name: default-a, policyId: source-a}
    windows: []
  - name: two
    managedPolicyId: managed-d
    defaultProfile: {name: default-d, policyId: source-d}
    windows: []
`
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(config.Schedules) != 2 || config.Schedules[1].ManagedPolicyID != "managed-d" {
		t.Fatalf("config = %#v", config)
	}
	config.Schedules[1].ManagedPolicyID = "managed-a"
	if err := config.Validate(); err == nil {
		t.Fatal("duplicate managed policy should fail")
	}
	config.Schedules[1].ManagedPolicyID = "managed-d"
	config.Schedules[1].DefaultProfile.PolicyID = "managed-a"
	if err := config.Validate(); err == nil {
		t.Fatal("managed policy used as a source should fail")
	}
}

type fakeCAST struct {
	policies map[string]Policy
	updates  []PolicyUpdate
	failGet  map[string]error
}

func (f *fakeCAST) GetPolicy(_ context.Context, _, id string) (Policy, error) {
	if err := f.failGet[id]; err != nil {
		return Policy{}, err
	}
	return f.policies[id], nil
}

func TestReconcileMultipleSchedulesAndIsolatesFailures(t *testing.T) {
	config := baseConfig()
	config.Schedules = append(config.Schedules, PolicySchedule{
		Name: "application-two", ManagedPolicyID: "managed-two",
		DefaultProfile: Profile{Name: "batch-default", PolicyID: "batch-id"},
	})
	fake := &fakeCAST{policies: map[string]Policy{
		"safe-id":     {ID: "safe-id", Name: "safe", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.2}}`), AssignmentRules: json.RawMessage(`[]`)},
		"batch-id":    {ID: "batch-id", Name: "batch", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.3}}`), AssignmentRules: json.RawMessage(`[]`)},
		"managed":     {ID: "managed", Name: "one", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.1}}`), AssignmentRules: json.RawMessage(`[]`)},
		"managed-two": {ID: "managed-two", Name: "two", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.1}}`), AssignmentRules: json.RawMessage(`[]`)},
	}}
	runner := Runner{Config: config, CAST: fake, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time {
		instant, _ := time.Parse(time.RFC3339, "2026-08-03T19:00:00+02:00")
		return instant
	}}
	if err := runner.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.updates) != 2 {
		t.Fatalf("updates = %d, want 2", len(fake.updates))
	}

	fake.updates = nil
	fake.policies["managed"] = Policy{ID: "managed", Name: "one", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.1}}`), AssignmentRules: json.RawMessage(`[]`)}
	fake.policies["managed-two"] = Policy{ID: "managed-two", Name: "two", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.1}}`), AssignmentRules: json.RawMessage(`[]`)}
	fake.failGet = map[string]error{"safe-id": errors.New("source unavailable")}
	if err := runner.Reconcile(context.Background()); err == nil {
		t.Fatal("expected aggregated error")
	}
	if len(fake.updates) != 1 || fake.updates[0].Name != "two" {
		t.Fatalf("second schedule did not reconcile: %#v", fake.updates)
	}
}
func (f *fakeCAST) UpdatePolicy(_ context.Context, _, id string, update PolicyUpdate) error {
	f.updates = append(f.updates, update)
	p := f.policies[id]
	p.ApplyType = update.ApplyType
	p.RecommendationPolicies = update.RecommendationPolicies
	p.AssignmentRules = update.AssignmentRules
	f.policies[id] = p
	return nil
}

func TestReconcileCopiesSettingsAndPreservesManagedIdentityAndRules(t *testing.T) {
	rules := json.RawMessage(`[{"namespace":{"names":["payments"]}}]`)
	hpa := json.RawMessage(`{"enabled":true}`)
	fake := &fakeCAST{policies: map[string]Policy{
		"safe-id": {ID: "safe-id", Name: "source", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.2}}`), AssignmentRules: json.RawMessage(`[]`)},
		"managed": {ID: "managed", Name: "managed-name", ApplyType: "DEFERRED", RecommendationPolicies: json.RawMessage(`{"applyType":"DEFERRED","cpu":{"overhead":0.1}}`), AssignmentRules: rules, HPASettings: hpa},
	}}
	runner := Runner{Config: baseConfig(), CAST: fake, Log: slog.New(slog.NewTextHandler(io.Discard, nil)), Now: func() time.Time {
		instant, _ := time.Parse(time.RFC3339, "2026-08-03T19:00:00+02:00")
		return instant
	}}
	if err := runner.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.updates) != 1 {
		t.Fatalf("updates = %d", len(fake.updates))
	}
	if fake.updates[0].Name != "managed-name" || string(fake.updates[0].AssignmentRules) != string(rules) {
		t.Fatalf("managed fields changed: %#v", fake.updates[0])
	}
	if string(fake.updates[0].HPASettings) != string(hpa) {
		t.Fatal("managed HPA settings changed")
	}
	if err := runner.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(fake.updates) != 1 {
		t.Fatal("converged policy was written again")
	}
}
