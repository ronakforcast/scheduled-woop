package app

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ClusterID       string           `yaml:"clusterId"`
	Timezone        string           `yaml:"timezone"`
	PollInterval    time.Duration    `yaml:"-"`
	PollIntervalRaw string           `yaml:"pollInterval"`
	ApplyType       string           `yaml:"applyType"`
	Schedules       []PolicySchedule `yaml:"schedules"`
}

type PolicySchedule struct {
	Name            string   `yaml:"name"`
	ManagedPolicyID string   `yaml:"managedPolicyId"`
	DefaultProfile  Profile  `yaml:"defaultProfile"`
	Windows         []Window `yaml:"windows"`
}

type Profile struct {
	Name     string `yaml:"name"`
	PolicyID string `yaml:"policyId"`
}

type Window struct {
	Name    string   `yaml:"name"`
	Days    []string `yaml:"days"`
	Start   string   `yaml:"start"`
	End     string   `yaml:"end"`
	Profile Profile  `yaml:"profile"`
}

func LoadConfig(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	var config Config
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)
	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	if config.PollIntervalRaw == "" {
		config.PollInterval = time.Minute
	} else if config.PollInterval, err = time.ParseDuration(config.PollIntervalRaw); err != nil {
		return Config{}, fmt.Errorf("pollInterval: %w", err)
	}
	if config.ApplyType == "" {
		config.ApplyType = "DEFERRED"
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	if strings.TrimSpace(c.ClusterID) == "" {
		return errors.New("clusterId is required")
	}
	if _, err := time.LoadLocation(c.Timezone); err != nil {
		return fmt.Errorf("timezone %q: %w", c.Timezone, err)
	}
	if c.PollInterval < 10*time.Second {
		return errors.New("pollInterval must be at least 10s")
	}
	if c.ApplyType != "DEFERRED" {
		return errors.New("only DEFERRED applyType is supported")
	}
	if len(c.Schedules) == 0 {
		return errors.New("at least one schedule is required")
	}
	seenSchedules := map[string]bool{}
	seenPolicies := map[string]string{}
	for index := range c.Schedules {
		schedule := &c.Schedules[index]
		if strings.TrimSpace(schedule.Name) == "" || seenSchedules[schedule.Name] {
			return fmt.Errorf("schedule %d must have a unique name", index)
		}
		seenSchedules[schedule.Name] = true
		if strings.TrimSpace(schedule.ManagedPolicyID) == "" {
			return fmt.Errorf("schedule %s managedPolicyId is required", schedule.Name)
		}
		if owner := seenPolicies[schedule.ManagedPolicyID]; owner != "" {
			return fmt.Errorf("schedules %s and %s target the same managedPolicyId", owner, schedule.Name)
		}
		seenPolicies[schedule.ManagedPolicyID] = schedule.Name
		if err := validateProfile(schedule.DefaultProfile, "schedule "+schedule.Name+" defaultProfile"); err != nil {
			return err
		}
		if err := validateWindows(schedule.Name, schedule.Windows); err != nil {
			return err
		}
	}
	for _, schedule := range c.Schedules {
		profiles := []Profile{schedule.DefaultProfile}
		for _, window := range schedule.Windows {
			profiles = append(profiles, window.Profile)
		}
		for _, profile := range profiles {
			if owner := seenPolicies[profile.PolicyID]; owner != "" {
				return fmt.Errorf("policy %s cannot be both schedule %s's managed policy and schedule %s's source profile", profile.PolicyID, owner, schedule.Name)
			}
		}
	}
	return nil
}

func validateWindows(scheduleName string, windows []Window) error {
	seen := map[string]bool{}
	for i, window := range windows {
		if strings.TrimSpace(window.Name) == "" || seen[window.Name] {
			return fmt.Errorf("schedule %s window %d must have a unique name", scheduleName, i)
		}
		seen[window.Name] = true
		if err := validateProfile(window.Profile, "window "+window.Name); err != nil {
			return err
		}
		if len(window.Days) == 0 {
			return fmt.Errorf("window %s needs at least one day", window.Name)
		}
		if _, err := parseClock(window.Start); err != nil {
			return fmt.Errorf("window %s start: %w", window.Name, err)
		}
		if _, err := parseClock(window.End); err != nil {
			return fmt.Errorf("window %s end: %w", window.Name, err)
		}
		if window.Start == window.End {
			return fmt.Errorf("window %s start and end must differ", window.Name)
		}
		for _, day := range window.Days {
			if _, ok := weekdays[strings.ToLower(day)]; !ok {
				return fmt.Errorf("window %s has invalid day %q", window.Name, day)
			}
		}
	}
	return validateNoOverlaps(windows)
}

func validateProfile(profile Profile, field string) error {
	if strings.TrimSpace(profile.Name) == "" || strings.TrimSpace(profile.PolicyID) == "" {
		return fmt.Errorf("%s name and policyId are required", field)
	}
	return nil
}
