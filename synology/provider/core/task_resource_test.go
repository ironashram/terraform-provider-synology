package core

import (
	"testing"

	"github.com/synology-community/go-synology/pkg/api/core"
)

func TestParseTaskScheduleSpecDaily(t *testing.T) {
	got, err := parseTaskScheduleSpec("17 3 * * *")
	if err != nil {
		t.Fatalf("parseTaskScheduleSpec returned error: %v", err)
	}

	if got.Minute != 17 || got.Hour != 3 {
		t.Fatalf("unexpected schedule time: %+v", got)
	}
	if got.WeekDay != "0,1,2,3,4,5,6" {
		t.Fatalf("unexpected weekday set: %q", got.WeekDay)
	}
}

func TestParseTaskScheduleSpecWeekly(t *testing.T) {
	got, err := parseTaskScheduleSpec("17 3 * * 1,3,5")
	if err != nil {
		t.Fatalf("parseTaskScheduleSpec returned error: %v", err)
	}

	if got.WeekDay != "1,3,5" {
		t.Fatalf("unexpected weekday set: %q", got.WeekDay)
	}
}

func TestParseTaskScheduleSpecRejectsUnsupportedMonthly(t *testing.T) {
	if _, err := parseTaskScheduleSpec("0 0 1 * *"); err == nil {
		t.Fatal("expected monthly schedule to be rejected")
	}
}

func TestRenderTaskScheduleDaily(t *testing.T) {
	got, err := renderTaskSchedule(core.TaskSchedule{
		DateType:   0,
		Hour:       3,
		Minute:     17,
		RepeatDate: 1001,
		WeekDay:    "0,1,2,3,4,5,6",
	})
	if err != nil {
		t.Fatalf("renderTaskSchedule returned error: %v", err)
	}

	if got != "17 3 * * *" {
		t.Fatalf("unexpected rendered schedule: %q", got)
	}
}

func TestTaskScheduleMatchesSpec(t *testing.T) {
	if !taskScheduleMatchesSpec("17 3 * * *", core.TaskSchedule{
		DateType:   0,
		Hour:       3,
		Minute:     17,
		RepeatDate: 1001,
		WeekDay:    "0,1,2,3,4,5,6",
	}) {
		t.Fatal("expected schedules to match")
	}
}

// dsmMinuteIntervalSchedule is the schedule object DSM itself returned for a task
// created with repeat_min=15, captured from SYNO.Core.TaskScheduler get.
func dsmMinuteIntervalSchedule() core.TaskSchedule {
	lastWorkHour := int64(23)
	return core.TaskSchedule{
		DateType:     0,
		Hour:         0,
		Minute:       0,
		LastWorkHour: &lastWorkHour,
		MonthlyWeek:  []string{},
		RepeatDate:   1001,
		RepeatHour:   0,
		RepeatMin:    15,
		WeekDay:      "0,1,2,3,4,5,6",
	}
}

func TestParseTaskScheduleSpecMinuteInterval(t *testing.T) {
	got, err := parseTaskScheduleSpec("*/15 * * * *")
	if err != nil {
		t.Fatalf("parseTaskScheduleSpec returned error: %v", err)
	}

	if got.RepeatMin != 15 {
		t.Fatalf("expected repeat_min 15, got %d", got.RepeatMin)
	}
	if got.RepeatHour != 0 || got.Hour != 0 || got.Minute != 0 {
		t.Fatalf("expected interval task to start at 00:00 with no hour repeat: %+v", got)
	}
	if got.LastWorkHour == nil || *got.LastWorkHour != 23 {
		t.Fatalf("expected last_work_hour 23, got %v", got.LastWorkHour)
	}
	if got.RepeatDate != 1001 || got.DateType != 0 {
		t.Fatalf("unexpected date fields: %+v", got)
	}
	if got.WeekDay != "0,1,2,3,4,5,6" {
		t.Fatalf("unexpected weekday set: %q", got.WeekDay)
	}
}

func TestParseTaskScheduleSpecRejectsUnsupportedMinuteInterval(t *testing.T) {
	if _, err := parseTaskScheduleSpec("*/7 * * * *"); err == nil {
		t.Fatal("expected an interval outside DSM's repeat_min_store_config to be rejected")
	}
}

func TestParseTaskScheduleSpecRejectsMinuteIntervalWithFixedHour(t *testing.T) {
	if _, err := parseTaskScheduleSpec("*/15 3 * * *"); err == nil {
		t.Fatal("expected a minute interval combined with a fixed hour to be rejected")
	}
}

func TestParseTaskScheduleSpecHourInterval(t *testing.T) {
	got, err := parseTaskScheduleSpec("30 */6 * * *")
	if err != nil {
		t.Fatalf("parseTaskScheduleSpec returned error: %v", err)
	}

	if got.RepeatHour != 6 || got.Minute != 30 || got.Hour != 0 || got.RepeatMin != 0 {
		t.Fatalf("unexpected hour-interval schedule: %+v", got)
	}
}

func TestRenderTaskScheduleMinuteInterval(t *testing.T) {
	got, err := renderTaskSchedule(dsmMinuteIntervalSchedule())
	if err != nil {
		t.Fatalf("renderTaskSchedule returned error: %v", err)
	}

	if got != "*/15 * * * *" {
		t.Fatalf("unexpected rendered schedule: %q", got)
	}
}

// Guards the drift-detection path: if this fails, terraform reports a permanent
// diff on a task whose live schedule is already correct.
func TestTaskScheduleMatchesSpecMinuteInterval(t *testing.T) {
	if !taskScheduleMatchesSpec("*/15 * * * *", dsmMinuteIntervalSchedule()) {
		t.Fatal("expected DSM's own repeat_min schedule to match the spec that produced it")
	}
}
