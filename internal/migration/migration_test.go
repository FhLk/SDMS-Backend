package migration

import (
	"sync"
	"testing"
	"time"

	"gorm.io/gorm/schema"
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

func TestSchemaModelsCoverEveryTable(t *testing.T) {
	want := []string{
		"users", "divisions", "user_divisions", "academic_years", "topics", "topic_fields",
		"submissions", "files", "topic_assignees", "sessions", "checkin_user_links",
	}

	models := SchemaModels()
	if len(models) != len(want) {
		t.Fatalf("SchemaModels() returned %d models, want %d", len(models), len(want))
	}
	cache := &sync.Map{}
	for index, value := range models {
		parsed, err := schema.Parse(value, cache, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parse model %T: %v", value, err)
		}
		if parsed.Table != want[index] {
			t.Errorf("model[%d] table = %q, want %q", index, parsed.Table, want[index])
		}
	}
}
