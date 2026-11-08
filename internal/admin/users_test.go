package admin

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateCreateUserInput(t *testing.T) {
	valid := CreateUserInput{
		Username:  "teacher.one",
		Password:  "safe-password",
		FirstName: "ครู",
		LastName:  "ตัวอย่าง",
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
		{name: "mixed name formats", input: CreateUserInput{Username: "teacher.one", Password: "safe-password", FirstName: "ครู", FullName: "ครู ตัวอย่าง"}, field: "full_name"},
		{name: "missing first name", input: createInputWithFirstName(valid, ""), field: "first_name"},
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
		Username:  " Teacher.One ",
		Prefix:    " นาย ",
		FirstName: " ครู ",
		LastName:  " ตัวอย่าง ",
	})
	if created.Username != "teacher.one" || created.Prefix != "นาย" || created.FirstName != "ครู" || created.LastName != "ตัวอย่าง" {
		t.Fatalf("normalizeCreateUserInput() = %+v", created)
	}
	legacy := normalizeCreateUserInput(CreateUserInput{FullName: " ผู้ดูแลระบบ "})
	if legacy.FirstName != "ผู้ดูแลระบบ" || legacy.FullName != "" {
		t.Fatalf("legacy normalizeCreateUserInput() = %+v, want first_name fallback", legacy)
	}

	username := " Teacher.Two "
	firstName := " ครู "
	lastName := " สอง "
	updated := normalizeUpdateUserInput(UpdateUserInput{
		Username:  &username,
		FirstName: &firstName,
		LastName:  &lastName,
	})
	if *updated.Username != "teacher.two" || *updated.FirstName != "ครู" || *updated.LastName != "สอง" {
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

func createInputWithUsername(input CreateUserInput, value string) CreateUserInput {
	input.Username = value
	return input
}

func createInputWithPassword(input CreateUserInput, value string) CreateUserInput {
	input.Password = value
	return input
}

func createInputWithFirstName(input CreateUserInput, value string) CreateUserInput {
	input.FirstName = value
	return input
}
