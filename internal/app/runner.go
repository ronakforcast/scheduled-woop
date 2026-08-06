package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"reflect"
	"time"
)

type PolicyAPI interface {
	GetPolicy(context.Context, string, string) (Policy, error)
	UpdatePolicy(context.Context, string, string, PolicyUpdate) error
}

type Runner struct {
	Config Config
	CAST   PolicyAPI
	Log    *slog.Logger
	Now    func() time.Time
}

func (r *Runner) Run(ctx context.Context) error {
	if err := r.Reconcile(ctx); err != nil {
		r.Log.Error("reconciliation failed", "error", err)
	}
	ticker := time.NewTicker(r.Config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.Reconcile(ctx); err != nil {
				r.Log.Error("reconciliation failed", "error", err)
			}
		}
	}
}

func (r *Runner) Reconcile(ctx context.Context) error {
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	var reconcileErrors []error
	for index := range r.Config.Schedules {
		schedule := r.Config.Schedules[index]
		if err := r.reconcileSchedule(ctx, schedule, now); err != nil {
			r.Log.Error("schedule reconciliation failed", "schedule", schedule.Name, "managedPolicyId", schedule.ManagedPolicyID, "error", err)
			reconcileErrors = append(reconcileErrors, fmt.Errorf("schedule %s: %w", schedule.Name, err))
		}
	}
	return errors.Join(reconcileErrors...)
}

func (r *Runner) reconcileSchedule(ctx context.Context, schedule PolicySchedule, now time.Time) error {
	profile := schedule.ActiveProfile(now, r.Config.Timezone)
	source, err := r.CAST.GetPolicy(ctx, r.Config.ClusterID, profile.PolicyID)
	if err != nil {
		return fmt.Errorf("read profile %s: %w", profile.Name, err)
	}
	managed, err := r.CAST.GetPolicy(ctx, r.Config.ClusterID, schedule.ManagedPolicyID)
	if err != nil {
		return fmt.Errorf("read managed policy: %w", err)
	}

	recommendations, err := forceApplyType(source.RecommendationPolicies, r.Config.ApplyType)
	if err != nil {
		return err
	}
	desired := PolicyUpdate{Name: managed.Name, ApplyType: r.Config.ApplyType, RecommendationPolicies: recommendations, AssignmentRules: managed.AssignmentRules, HPASettings: managed.HPASettings}
	current := PolicyUpdate{Name: managed.Name, ApplyType: managed.ApplyType, RecommendationPolicies: managed.RecommendationPolicies, AssignmentRules: managed.AssignmentRules, HPASettings: managed.HPASettings}
	equal, err := equivalent(current, desired)
	if err != nil {
		return err
	}
	if equal {
		r.Log.Info("policy already converged", "profile", profile.Name, "managedPolicyId", managed.ID)
		return nil
	}
	if err := r.CAST.UpdatePolicy(ctx, r.Config.ClusterID, schedule.ManagedPolicyID, desired); err != nil {
		return fmt.Errorf("apply profile %s: %w", profile.Name, err)
	}
	verified, err := r.CAST.GetPolicy(ctx, r.Config.ClusterID, schedule.ManagedPolicyID)
	if err != nil {
		return fmt.Errorf("verify update: %w", err)
	}
	verifiedUpdate := PolicyUpdate{Name: verified.Name, ApplyType: verified.ApplyType, RecommendationPolicies: verified.RecommendationPolicies, AssignmentRules: verified.AssignmentRules, HPASettings: verified.HPASettings}
	ok, err := equivalent(verifiedUpdate, desired)
	if err != nil || !ok {
		return fmt.Errorf("CAST policy did not match after update")
	}
	r.Log.Info("profile applied", "schedule", schedule.Name, "profile", profile.Name, "managedPolicyId", managed.ID)
	return nil
}

func forceApplyType(raw json.RawMessage, applyType string) (json.RawMessage, error) {
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("recommendationPolicies: %w", err)
	}
	value["applyType"] = applyType
	return json.Marshal(value)
}

func equivalent(a, b PolicyUpdate) (bool, error) {
	left, err := canonical(a)
	if err != nil {
		return false, err
	}
	right, err := canonical(b)
	if err != nil {
		return false, err
	}
	return reflect.DeepEqual(left, right), nil
}

func canonical(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	err = json.Unmarshal(data, &result)
	return result, err
}
