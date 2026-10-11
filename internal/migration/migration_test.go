package migration

import (
	"testing"
	"time"
)

func TestCurrentAcademicPeriod(t *testing.T) {
	tests := []struct {
		name         string
		date         time.Time
		wantYear     int16
		wantSemester int16
	}{
		{
			name:         "before academic year starts",
			date:         time.Date(2026, time.April, 30, 12, 0, 0, 0, bangkokLocation),
			wantYear:     2568,
			wantSemester: 2,
		},
		{
			name:         "first semester",
			date:         time.Date(2026, time.May, 1, 0, 0, 0, 0, bangkokLocation),
			wantYear:     2569,
			wantSemester: 1,
		},
		{
			name:         "second semester",
			date:         time.Date(2026, time.November, 1, 0, 0, 0, 0, bangkokLocation),
			wantYear:     2569,
			wantSemester: 2,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			year, semester := CurrentAcademicPeriod(test.date)
			if year != test.wantYear || semester != test.wantSemester {
				t.Fatalf("CurrentAcademicPeriod() = (%d, %d), want (%d, %d)", year, semester, test.wantYear, test.wantSemester)
			}
		})
	}
}

func TestDefaultDivisions(t *testing.T) {
	divisions := DefaultDivisions()
	if len(divisions) != 4 {
		t.Fatalf("DefaultDivisions() returned %d divisions, want 4", len(divisions))
	}

	wantCodes := []string{"aca", "bud", "per", "gen"}
	for index, wantCode := range wantCodes {
		if got := divisions[index].Code; got != wantCode {
			t.Errorf("division[%d].Code = %q, want %q", index, got, wantCode)
		}
	}
}
