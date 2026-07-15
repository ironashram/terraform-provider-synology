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
