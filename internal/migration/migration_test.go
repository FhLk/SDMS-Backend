package migration

import (
	"testing"
	"time"
)

func TestCurrentAcademicPeriod(t *testing.T) {
	tests := []struct {
		name         string
		date         string
		wantYear     int16
		wantSemester int16
	}{
		{name: "ก่อนเปิดปีการศึกษา", date: "2026-04-30T12:00:00+07:00", wantYear: 2568, wantSemester: 2},
		{name: "ภาคเรียนหนึ่ง", date: "2026-05-01T00:00:00+07:00", wantYear: 2569, wantSemester: 1},
		{name: "ภาคเรียนสอง", date: "2026-11-01T00:00:00+07:00", wantYear: 2569, wantSemester: 2},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := time.Parse(time.RFC3339, test.date)
			if err != nil {
				t.Fatalf("parse test date: %v", err)
			}

			year, semester := CurrentAcademicPeriod(value)
			if year != test.wantYear || semester != test.wantSemester {
				t.Fatalf("CurrentAcademicPeriod() = (%d, %d), want (%d, %d)", year, semester, test.wantYear, test.wantSemester)
			}
		})
	}
}

func TestSchemaModelsContainsCurrentTables(t *testing.T) {
	if got, want := len(SchemaModels()), 10; got != want {
		t.Fatalf("len(SchemaModels()) = %d, want %d", got, want)
	}
}
