package autoconfig

import (
	"testing"
	"time"
)

func TestRSSCheckScheduleEmptySpecAllowsAnyTime(t *testing.T) {
	schedule, err := ParseRSSCheckSchedule("", time.UTC)
	if err != nil {
		t.Fatalf("ParseRSSCheckSchedule() error = %v", err)
	}

	now := time.Date(2026, time.September, 11, 3, 0, 0, 0, time.UTC)
	if !schedule.Allows(now) {
		t.Fatal("empty schedule should allow any time")
	}
}

func TestRSSCheckScheduleAllowsConfiguredWindow(t *testing.T) {
	location := time.FixedZone("test", 2*60*60)
	schedule, err := ParseRSSCheckSchedule("Tue@09:00-23:59,Wed@09:00-23:59", location)
	if err != nil {
		t.Fatalf("ParseRSSCheckSchedule() error = %v", err)
	}

	allowed := time.Date(2026, time.September, 9, 17, 30, 0, 0, location)
	if !schedule.Allows(allowed) {
		t.Fatal("schedule should allow Wednesday afternoon")
	}

	blocked := time.Date(2026, time.September, 9, 3, 0, 0, 0, location)
	if schedule.Allows(blocked) {
		t.Fatal("schedule should block Wednesday early morning")
	}
}

func TestRSSCheckScheduleSupportsCrossMidnightWindow(t *testing.T) {
	schedule, err := ParseRSSCheckSchedule("Tue@22:00-02:00", time.UTC)
	if err != nil {
		t.Fatalf("ParseRSSCheckSchedule() error = %v", err)
	}

	tuesdayNight := time.Date(2026, time.September, 8, 23, 30, 0, 0, time.UTC)
	if !schedule.Allows(tuesdayNight) {
		t.Fatal("schedule should allow Tuesday late night")
	}

	wednesdayEarly := time.Date(2026, time.September, 9, 1, 30, 0, 0, time.UTC)
	if !schedule.Allows(wednesdayEarly) {
		t.Fatal("schedule should allow spillover after midnight")
	}

	wednesdayLate := time.Date(2026, time.September, 9, 3, 0, 0, 0, time.UTC)
	if schedule.Allows(wednesdayLate) {
		t.Fatal("schedule should block after cross-midnight window ends")
	}
}

func TestRSSCheckScheduleRejectsInvalidSpec(t *testing.T) {
	if _, err := ParseRSSCheckSchedule("Tuesday 09:00-23:59", time.UTC); err == nil {
		t.Fatal("ParseRSSCheckSchedule() expected error")
	}
}
