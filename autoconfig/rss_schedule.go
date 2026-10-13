package autoconfig

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultRSSCheckTimezone = "Europe/Madrid"
	EnvRSSCheckWindows      = "RSS_CHECK_WINDOWS"
	EnvRSSCheckTimezone     = "RSS_CHECK_TIMEZONE"
)

type RSSCheckSchedule struct {
	Spec     string
	Location *time.Location
	windows  []rssCheckWindow
}

type rssCheckWindow struct {
	day         time.Weekday
	startMinute int
	endMinute   int
}

func FromRSSCheckScheduleEnv() (RSSCheckSchedule, error) {
	spec := os.Getenv(EnvRSSCheckWindows)
	if strings.TrimSpace(spec) == "" {
		return ParseRSSCheckSchedule(spec, time.Local)
	}

	timezone := os.Getenv(EnvRSSCheckTimezone)
	if timezone == "" {
		timezone = DefaultRSSCheckTimezone
	}

	location, err := time.LoadLocation(timezone)
	if err != nil {
		return RSSCheckSchedule{}, fmt.Errorf("%s: %w", EnvRSSCheckTimezone, err)
	}

	return ParseRSSCheckSchedule(spec, location)
}

func ParseRSSCheckSchedule(spec string, location *time.Location) (RSSCheckSchedule, error) {
	spec = strings.TrimSpace(spec)
	if location == nil {
		location = time.Local
	}

	if spec == "" {
		return RSSCheckSchedule{Spec: spec, Location: location}, nil
	}

	var windows []rssCheckWindow
	for _, rawWindow := range strings.Split(spec, ",") {
		rawWindow = strings.TrimSpace(rawWindow)
		if rawWindow == "" {
			continue
		}

		dayPart, rangePart, ok := strings.Cut(rawWindow, "@")
		if !ok {
			return RSSCheckSchedule{}, fmt.Errorf("invalid RSS check window %q: expected Day@HH:MM-HH:MM", rawWindow)
		}

		startPart, endPart, ok := strings.Cut(strings.TrimSpace(rangePart), "-")
		if !ok {
			return RSSCheckSchedule{}, fmt.Errorf("invalid RSS check window %q: expected Day@HH:MM-HH:MM", rawWindow)
		}

		startMinute, err := parseClockMinute(startPart)
		if err != nil {
			return RSSCheckSchedule{}, fmt.Errorf("invalid RSS check window %q: %w", rawWindow, err)
		}
		endMinute, err := parseClockMinute(endPart)
		if err != nil {
			return RSSCheckSchedule{}, fmt.Errorf("invalid RSS check window %q: %w", rawWindow, err)
		}

		days, err := parseWeekdayList(dayPart)
		if err != nil {
			return RSSCheckSchedule{}, fmt.Errorf("invalid RSS check window %q: %w", rawWindow, err)
		}
		for _, day := range days {
			windows = append(windows, rssCheckWindow{
				day:         day,
				startMinute: startMinute,
				endMinute:   endMinute,
			})
		}
	}

	return RSSCheckSchedule{Spec: spec, Location: location, windows: windows}, nil
}

func (s RSSCheckSchedule) Allows(now time.Time) bool {
	if len(s.windows) == 0 {
		return true
	}

	localNow := now.In(s.Location)
	currentDay := localNow.Weekday()
	currentMinute := localNow.Hour()*60 + localNow.Minute()

	for _, window := range s.windows {
		if window.allows(currentDay, currentMinute) {
			return true
		}
	}
	return false
}

func (w rssCheckWindow) allows(day time.Weekday, minute int) bool {
	if w.startMinute <= w.endMinute {
		return day == w.day && minute >= w.startMinute && minute <= w.endMinute
	}

	nextDay := (w.day + 1) % 7
	return (day == w.day && minute >= w.startMinute) || (day == nextDay && minute <= w.endMinute)
}

func parseClockMinute(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	hourPart, minutePart, ok := strings.Cut(raw, ":")
	if !ok {
		return 0, fmt.Errorf("invalid clock %q: expected HH:MM", raw)
	}

	hour, err := strconv.Atoi(hourPart)
	if err != nil || hour < 0 || hour > 23 {
		return 0, fmt.Errorf("invalid hour %q", hourPart)
	}
	minute, err := strconv.Atoi(minutePart)
	if err != nil || minute < 0 || minute > 59 {
		return 0, fmt.Errorf("invalid minute %q", minutePart)
	}
	return hour*60 + minute, nil
}

func parseWeekdayList(raw string) ([]time.Weekday, error) {
	raw = strings.TrimSpace(raw)
	if raw == "*" {
		return []time.Weekday{
			time.Sunday,
			time.Monday,
			time.Tuesday,
			time.Wednesday,
			time.Thursday,
			time.Friday,
			time.Saturday,
		}, nil
	}

	var days []time.Weekday
	for _, rawDay := range strings.Split(raw, "+") {
		day, err := parseWeekday(rawDay)
		if err != nil {
			return nil, err
		}
		days = append(days, day)
	}
	return days, nil
}

func parseWeekday(raw string) (time.Weekday, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sun", "sunday", "dom", "domingo", "0", "7":
		return time.Sunday, nil
	case "mon", "monday", "lun", "lunes", "1":
		return time.Monday, nil
	case "tue", "tuesday", "mar", "martes", "2":
		return time.Tuesday, nil
	case "wed", "wednesday", "mie", "miercoles", "3":
		return time.Wednesday, nil
	case "thu", "thursday", "jue", "jueves", "4":
		return time.Thursday, nil
	case "fri", "friday", "vie", "viernes", "5":
		return time.Friday, nil
	case "sat", "saturday", "sab", "sabado", "6":
		return time.Saturday, nil
	default:
		return 0, fmt.Errorf("unknown weekday %q", raw)
	}
}
