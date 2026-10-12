package admin

import (
	"errors"
	"testing"
)

func TestValidateAcademicYear(t *testing.T) {
	tests := []struct {
		name      string
		year      int16
		semester  int16
		wantField string
	}{
		{name: "valid first semester", year: 2569, semester: 1},
		{name: "valid second semester", year: 2570, semester: 2},
		{name: "year too small", year: 2399, semester: 1, wantField: "year"},
		{name: "year too large", year: 3001, semester: 1, wantField: "year"},
		{name: "invalid semester", year: 2569, semester: 3, wantField: "current_semester"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAcademicYear(tt.year, tt.semester)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("validateAcademicYear() error = %v", err)
				}
				return
			}

			var validationErr *ValidationError
			if !errors.As(err, &validationErr) || validationErr.Field != tt.wantField {
				t.Fatalf("error = %v, want ValidationError field %q", err, tt.wantField)
			}
		})
	}
}
