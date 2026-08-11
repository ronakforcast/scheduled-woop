package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `clusterId: cluster
timezone: UTC
schedules:
  - name: production
    managedPolicyId: managed
    defaultProfile: {name: default, policyId: source}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if config.PollInterval != time.Minute {
		t.Fatalf("default interval = %s, want 1m", config.PollInterval)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := `clusterId: cluster
timezone: UTC
unexpected: true
schedules: []
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(path); err == nil || !strings.Contains(err.Error(), "field unexpected not found") {
		t.Fatalf("error = %v, want unknown-field error", err)
	}
}

func TestLoadConfigReportsMissingAndMalformedFiles(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadConfig(filepath.Join(t.TempDir(), "missing.yaml"))
		if err == nil || !strings.Contains(err.Error(), "read config") {
			t.Fatalf("error = %v, want read config error", err)
		}
	})
	t.Run("malformed YAML", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte("clusterId: [\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		_, err := LoadConfig(path)
		if err == nil || !strings.Contains(err.Error(), "parse config") {
			t.Fatalf("error = %v, want parse config error", err)
		}
	})
}

func TestValidateRejectsUnsafeOrAmbiguousConfiguration(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"missing cluster", func(c *Config) { c.ClusterID = " " }, "clusterId is required"},
		{"invalid timezone", func(c *Config) { c.Timezone = "Mars/Olympus" }, "timezone"},
		{"fast polling", func(c *Config) { c.PollInterval = 9 * time.Second }, "at least 10s"},
		{"invalid legacy global apply mode", func(c *Config) { c.ApplyType = "IMMEDIATE" }, "source policy"},
		{"no schedules", func(c *Config) { c.Schedules = nil }, "at least one schedule"},
		{"blank managed policy", func(c *Config) { c.Schedules[0].ManagedPolicyID = "" }, "managedPolicyId is required"},
		{"blank source policy", func(c *Config) { c.Schedules[0].DefaultProfile.PolicyID = "" }, "name and policyId are required"},
		{"invalid weekday", func(c *Config) { c.Schedules[0].Windows[0].Days = []string{"Funday"} }, "invalid day"},
		{"invalid clock", func(c *Config) { c.Schedules[0].Windows[0].Start = "8:00" }, "must be HH:MM"},
		{"full zero window", func(c *Config) { c.Schedules[0].Windows[0].End = c.Schedules[0].Windows[0].Start }, "must differ"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			config := baseConfig()
			tc.edit(&config)
			if err := config.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}
