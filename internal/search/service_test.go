package search

import (
	"errors"
	"strings"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

func TestNormalizeInputAppliesDefaultsAndNormalizesDivision(t *testing.T) {
	input := Input{
		Profile:  auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: uuid.New()}}},
		Query:    "  ภาษาไทย  ",
		Division: " ACA ",
	}
	got, err := normalizeInput(input)
	if err != nil {
		t.Fatalf("normalizeInput() error = %v", err)
	}
	if got.Query != "ภาษาไทย" || got.Division != "aca" || got.Page != 1 || got.PageSize != DefaultPageSize {
		t.Fatalf("normalized input = %+v", got)
	}
}

func TestNormalizeInputRejectsInvalidFilters(t *testing.T) {
	profile := auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: uuid.New()}}}
	invalidYear := int16(2399)
	invalidSemester := int16(3)
	nilUUID := uuid.Nil
	tests := []struct {
		name      string
		input     Input
		wantField string
	}{
		{name: "missing profile", input: Input{}, wantField: "profile"},
		{name: "long query", input: Input{Profile: profile, Query: strings.Repeat("ก", 201)}, wantField: "q"},
		{name: "invalid division", input: Input{Profile: profile, Division: "aca/th"}, wantField: "division"},
		{name: "invalid year", input: Input{Profile: profile, Year: &invalidYear}, wantField: "year"},
		{name: "invalid semester", input: Input{Profile: profile, Semester: &invalidSemester}, wantField: "semester"},
		{name: "nil submitter", input: Input{Profile: profile, By: &nilUUID}, wantField: "by"},
		{name: "negative page", input: Input{Profile: profile, Page: -1}, wantField: "page"},
		{name: "large page size", input: Input{Profile: profile, PageSize: MaxPageSize + 1}, wantField: "page_size"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := normalizeInput(tt.input)
			if tt.wantField == "profile" {
				if !errors.Is(err, ErrInvalidProfile) {
					t.Fatalf("error = %v, want ErrInvalidProfile", err)
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

func TestEscapeLikeTreatsWildcardsAsText(t *testing.T) {
	if got, want := escapeLike(`100%_ครบ\แล้ว`), `100\%\_ครบ\\แล้ว`; got != want {
		t.Fatalf("escapeLike() = %q, want %q", got, want)
	}
}
