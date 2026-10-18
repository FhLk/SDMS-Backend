package sampledata

import (
	"testing"

	"github.com/google/uuid"
)

func TestValidateRejectsNonDevelopmentDataset(t *testing.T) {
	dataset := Dataset{
		Meta:  Metadata{Environment: "production"},
		Users: []SampleUser{{Username: "teacher.one", Password: "safe-password"}},
	}
	if err := validate(dataset); err == nil {
		t.Fatal("validate() error = nil, want non-development dataset rejection")
	}
}

func TestValidateRejectsDuplicateUsername(t *testing.T) {
	dataset := Dataset{
		Meta: Metadata{Environment: developmentOnly},
		Users: []SampleUser{
			{Username: "teacher.one", Password: "safe-password"},
			{Username: "teacher.one", Password: "safe-password"},
		},
	}
	if err := validate(dataset); err == nil {
		t.Fatal("validate() error = nil, want duplicate username rejection")
	}
}

func TestResolveDivisionUUIDsRejectsUnknownCode(t *testing.T) {
	if _, err := resolveDivisionUUIDs([]string{"missing"}, map[string]uuid.UUID{}); err == nil {
		t.Fatal("resolveDivisionUUIDs() error = nil, want unknown division rejection")
	}
}
