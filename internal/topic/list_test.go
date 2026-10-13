package topic

import (
	"testing"
	"time"
)

func TestTopicStatusUsesBangkokCalendarDate(t *testing.T) {
	dueDate := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	onTime := time.Date(2026, time.October, 13, 16, 59, 0, 0, time.UTC) // 23:59 เวลาไทย
	late := time.Date(2026, time.October, 13, 17, 0, 0, 0, time.UTC)    // 00:00 วันถัดไปเวลาไทย
	today := time.Date(2026, time.October, 13, 23, 59, 0, 0, topicBangkokLocation)
	tomorrow := time.Date(2026, time.October, 14, 0, 0, 0, 0, topicBangkokLocation)

	tests := []struct {
		name        string
		assigned    bool
		submittedAt *time.Time
		open        bool
		now         time.Time
		want        *string
	}{
		{name: "not assigned", open: true, now: today},
		{name: "submitted on due date", assigned: true, submittedAt: &onTime, open: true, now: tomorrow, want: statusPointer(StatusSubmitted)},
		{name: "submitted after due date", assigned: true, submittedAt: &late, open: true, now: tomorrow, want: statusPointer(StatusLate)},
		{name: "pending through end of due date", assigned: true, open: true, now: today, want: statusPointer(StatusPending)},
		{name: "overdue after due date", assigned: true, open: true, now: tomorrow, want: statusPointer(StatusOverdue)},
		{name: "closed without submission", assigned: true, open: false, now: today, want: statusPointer(StatusNotSubmitted)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := topicStatus(tt.assigned, tt.submittedAt, dueDate, tt.open, tt.now)
			if tt.want == nil {
				if got != nil {
					t.Fatalf("topicStatus() = %q, want nil", *got)
				}
				return
			}
			if got == nil || *got != *tt.want {
				t.Fatalf("topicStatus() = %v, want %q", got, *tt.want)
			}
		})
	}
}

func statusPointer(value string) *string { return &value }
