package app

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

var weekdays = map[string]time.Weekday{
	"sunday": time.Sunday, "monday": time.Monday, "tuesday": time.Tuesday,
	"wednesday": time.Wednesday, "thursday": time.Thursday, "friday": time.Friday, "saturday": time.Saturday,
}

func (s PolicySchedule) ActiveProfile(now time.Time, timezone string) Profile {
	location, _ := time.LoadLocation(timezone)
	local := now.In(location)
	minute := local.Hour()*60 + local.Minute()
	for _, window := range s.Windows {
		start, _ := parseClock(window.Start)
		end, _ := parseClock(window.End)
		if start < end {
			if hasDay(window.Days, local.Weekday()) && minute >= start && minute < end {
				return window.Profile
			}
			continue
		}
		if hasDay(window.Days, local.Weekday()) && minute >= start {
			return window.Profile
		}
		previous := local.AddDate(0, 0, -1).Weekday()
		if hasDay(window.Days, previous) && minute < end {
			return window.Profile
		}
	}
	return s.DefaultProfile
}

func parseClock(value string) (int, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 2 || len(parts[0]) != 2 || len(parts[1]) != 2 {
		return 0, fmt.Errorf("%q must be HH:MM", value)
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || hour < 0 || hour > 23 || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("%q is not a valid local time", value)
	}
	return hour*60 + minute, nil
}

func hasDay(values []string, wanted time.Weekday) bool {
	for _, value := range values {
		if weekdays[strings.ToLower(value)] == wanted {
			return true
		}
	}
	return false
}

func validateNoOverlaps(windows []Window) error {
	occupied := make([]string, 7*24*60)
	for _, window := range windows {
		start, _ := parseClock(window.Start)
		end, _ := parseClock(window.End)
		for _, dayName := range window.Days {
			day := int(weekdays[strings.ToLower(dayName)])
			length := end - start
			if length <= 0 {
				length += 24 * 60
			}
			for offset := 0; offset < length; offset++ {
				index := (day*24*60 + start + offset) % len(occupied)
				if occupied[index] != "" {
					return fmt.Errorf("windows %s and %s overlap", occupied[index], window.Name)
				}
				occupied[index] = window.Name
			}
		}
	}
	return nil
}
