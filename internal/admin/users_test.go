package admin

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestValidateCreateUserInput(t *testing.T) {
	valid := CreateUserInput{
		Username: "teacher.one",
		Password: "safe-password",
		FullName: "ครูตัวอย่าง",
		Position: "ครู",
	}
	if err := validateCreateUserInput(valid); err != nil {
		t.Fatalf("validateCreateUserInput(valid) error = %v", err)
	}

	tests := []struct {
		name  string
		input CreateUserInput
		field string
	}{
		{name: "invalid username", input: createInputWithUsername(valid, "INVALID"), field: "username"},
		{name: "short password", input: createInputWithPassword(valid, "short"), field: "password"},
		{name: "long password", input: createInputWithPassword(valid, strings.Repeat("x", 73)), field: "password"},
		{name: "missing full name", input: createInputWithFullName(valid, ""), field: "full_name"},
		{name: "missing position", input: createInputWithPosition(valid, ""), field: "position"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCreateUserInput(tt.input)
			var validationErr *ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("error = %v, want ValidationError", err)
			}
			if validationErr.Field != tt.field {
				t.Fatalf("field = %q, want %q", validationErr.Field, tt.field)
			}
		})
	}
}

func TestNormalizeUserInput(t *testing.T) {
	created := normalizeCreateUserInput(CreateUserInput{
		Username: " Teacher.One ",
		FullName: " ครู ตัวอย่าง ",
		Position: " ครู ",
	})
	if created.Username != "teacher.one" || created.FullName != "ครู ตัวอย่าง" || created.Position != "ครู" {
		t.Fatalf("normalizeCreateUserInput() = %+v", created)
	}

	username := " Teacher.Two "
	fullName := " ครู สอง "
	position := " ครูผู้ช่วย "
	updated := normalizeUpdateUserInput(UpdateUserInput{
		Username: &username,
		FullName: &fullName,
		Position: &position,
	})
	if *updated.Username != "teacher.two" || *updated.FullName != "ครู สอง" || *updated.Position != "ครูผู้ช่วย" {
		t.Fatalf("normalizeUpdateUserInput() = %+v", updated)
	}
}

func TestValidateUpdateUserInputRequiresAField(t *testing.T) {
	err := validateUpdateUserInput(UpdateUserInput{})
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "request" {
		t.Fatalf("error = %v, want request ValidationError", err)
	}

	// false เป็นค่าที่ตั้งใจส่งได้ pointer จึงต้องนับว่า request มี field.
	falseValue := false
	if err := validateUpdateUserInput(UpdateUserInput{IsActive: &falseValue}); err != nil {
		t.Fatalf("validateUpdateUserInput(false field) error = %v", err)
	}
}

func TestUniqueDivisionUUIDs(t *testing.T) {
	first := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f840001")
	second := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f840002")
	got, err := uniqueDivisionUUIDs([]uuid.UUID{first, second, first})
	if err != nil {
		t.Fatalf("uniqueDivisionUUIDs() error = %v", err)
	}
	if len(got) != 2 || got[0] != first || got[1] != second {
		t.Fatalf("uniqueDivisionUUIDs() = %v, want first and second", got)
	}

	if _, err := uniqueDivisionUUIDs([]uuid.UUID{uuid.Nil}); err == nil {
		t.Fatal("uniqueDivisionUUIDs(nil UUID) error = nil, want validation error")
	}
}

func createInputWithUsername(input CreateUserInput, value string) CreateUserInput {
	input.Username = value
	return input
}

func createInputWithPassword(input CreateUserInput, value string) CreateUserInput {
	input.Password = value
	return input
}

func createInputWithFullName(input CreateUserInput, value string) CreateUserInput {
	input.FullName = value
	return input
}

func createInputWithPosition(input CreateUserInput, value string) CreateUserInput {
	input.Position = value
	return input
}
