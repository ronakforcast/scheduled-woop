package app

import (
	"testing"
	"time"
)

func mustTime(t *testing.T, value string) time.Time {
	t.Helper()
	instant, err := time.Parse(time.RFC3339, value)
	if err != nil {
		t.Fatal(err)
	}
	return instant
}

func TestActiveProfileUsesInclusiveStartAndExclusiveEnd(t *testing.T) {
	schedule := baseConfig().Schedules[0]
	cases := []struct {
		name, instant, want string
	}{
		{"before start", "2026-08-03T07:59:00+02:00", "safe"},
		{"at start", "2026-08-03T08:00:00+02:00", "performance"},
		{"before end", "2026-08-03T17:59:00+02:00", "performance"},
		{"at end", "2026-08-03T18:00:00+02:00", "safe"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schedule.ActiveProfile(mustTime(t, tc.instant), "Europe/Prague").Name; got != tc.want {
				t.Fatalf("profile = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestActiveProfileOvernightWindowBelongsToStartDay(t *testing.T) {
	schedule := PolicySchedule{
		DefaultProfile: Profile{Name: "default", PolicyID: "default-id"},
		Windows: []Window{{
			Name: "monday-night", Days: []string{"Monday"}, Start: "22:00", End: "06:00",
			Profile: Profile{Name: "night", PolicyID: "night-id"},
		}},
	}
	cases := []struct {
		name, instant, want string
	}{
		{"monday before", "2026-08-03T21:59:00+02:00", "default"},
		{"monday start", "2026-08-03T22:00:00+02:00", "night"},
		{"tuesday continuation", "2026-08-04T05:59:00+02:00", "night"},
		{"tuesday end", "2026-08-04T06:00:00+02:00", "default"},
		{"sunday is not included", "2026-08-02T23:00:00+02:00", "default"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := schedule.ActiveProfile(mustTime(t, tc.instant), "Europe/Prague").Name; got != tc.want {
				t.Fatalf("profile = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestActiveProfileUsesConfiguredTimezoneAcrossDST(t *testing.T) {
	schedule := PolicySchedule{
		DefaultProfile: Profile{Name: "default", PolicyID: "default-id"},
		Windows: []Window{{
			Name: "business", Days: []string{"Monday"}, Start: "08:00", End: "09:00",
			Profile: Profile{Name: "business", PolicyID: "business-id"},
		}},
	}
	// Europe/Amsterdam changes from UTC+1 to UTC+2 in March. Both instants are 08:30 local.
	for _, instant := range []string{"2026-01-05T07:30:00Z", "2026-06-01T06:30:00Z"} {
		if got := schedule.ActiveProfile(mustTime(t, instant), "Europe/Amsterdam").Name; got != "business" {
			t.Fatalf("%s profile = %q, want business", instant, got)
		}
	}
}

func TestValidateRejectsOverlappingOvernightWindows(t *testing.T) {
	config := baseConfig()
	config.Schedules[0].Windows = []Window{
		{Name: "monday-night", Days: []string{"Monday"}, Start: "22:00", End: "06:00", Profile: Profile{Name: "one", PolicyID: "one-id"}},
		{Name: "tuesday-early", Days: []string{"Tuesday"}, Start: "05:00", End: "07:00", Profile: Profile{Name: "two", PolicyID: "two-id"}},
	}
	if err := config.Validate(); err == nil {
		t.Fatal("expected overnight overlap to be rejected")
	}
}
