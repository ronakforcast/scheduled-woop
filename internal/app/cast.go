package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Policy struct {
	ID                     string          `json:"id"`
	Name                   string          `json:"name"`
	ApplyType              string          `json:"applyType"`
	RecommendationPolicies json.RawMessage `json:"recommendationPolicies"`
	AssignmentRules        json.RawMessage `json:"assignmentRules"`
	HPASettings            json.RawMessage `json:"hpaSettings,omitempty"`
}

type PolicyUpdate struct {
	Name                   string          `json:"name"`
	ApplyType              string          `json:"applyType"`
	RecommendationPolicies json.RawMessage `json:"recommendationPolicies"`
	AssignmentRules        json.RawMessage `json:"assignmentRules"`
	HPASettings            json.RawMessage `json:"hpaSettings,omitempty"`
}

type CASTClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewCASTClient(baseURL, apiKey string) (*CASTClient, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	apiKey = strings.TrimSpace(apiKey)
	parsed, err := url.Parse(baseURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, errors.New("invalid CAST API URL")
	}
	if apiKey == "" {
		return nil, errors.New("CAST API key is empty")
	}
	return &CASTClient{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}, nil
}

func (c *CASTClient) GetPolicy(ctx context.Context, clusterID, policyID string) (Policy, error) {
	var result Policy
	path := fmt.Sprintf("/v1/workload-autoscaling/clusters/%s/policies/%s", url.PathEscape(clusterID), url.PathEscape(policyID))
	if err := c.request(ctx, http.MethodGet, path, nil, &result); err != nil {
		return result, err
	}
	if result.ID == "" || result.Name == "" || len(result.RecommendationPolicies) == 0 || len(result.AssignmentRules) == 0 {
		return Policy{}, errors.New("CAST policy response is missing required fields")
	}
	return result, nil
}

func (c *CASTClient) UpdatePolicy(ctx context.Context, clusterID, policyID string, update PolicyUpdate) error {
	path := fmt.Sprintf("/v1/workload-autoscaling/clusters/%s/policies/%s", url.PathEscape(clusterID), url.PathEscape(policyID))
	return c.request(ctx, http.MethodPut, path, update, nil)
}

func (c *CASTClient) request(ctx context.Context, method, path string, input, output any) error {
	var payload []byte
	var err error
	if input != nil {
		payload, err = json.Marshal(input)
		if err != nil {
			return err
		}
	}
	maxAttempts := 1
	if method == http.MethodGet {
		maxAttempts = 3
	}
	for attempt := 0; attempt < maxAttempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("X-API-Key", c.apiKey)
		req.Header.Set("Accept", "application/json")
		if input != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		response, err := c.http.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if attempt+1 < maxAttempts {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
				continue
			}
			return fmt.Errorf("CAST request failed: %w", err)
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4<<20))
		_ = response.Body.Close()
		if readErr != nil {
			return readErr
		}
		if response.StatusCode == 429 || response.StatusCode >= 500 {
			if attempt+1 < maxAttempts {
				time.Sleep(time.Duration(attempt+1) * 250 * time.Millisecond)
				continue
			}
		}
		if response.StatusCode < 200 || response.StatusCode >= 300 {
			return fmt.Errorf("CAST API returned HTTP %d", response.StatusCode)
		}
		if output != nil {
			if err := json.Unmarshal(body, output); err != nil {
				return fmt.Errorf("decode CAST response: %w", err)
			}
		}
		return nil
	}
	return errors.New("CAST request retries exhausted")
}
