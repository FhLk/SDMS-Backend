package admin

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	valid := CreateInput{
		Username: "admin",
		Password: "strong-password",
		FullName: "System Administrator",
		Position: "Administrator",
	}
	if err := validate(valid); err != nil {
		t.Fatalf("validate(valid) error = %v", err)
	}

	tests := []struct {
		name  string
		input CreateInput
	}{
		{name: "invalid username", input: withUsername(valid, "A")},
		{name: "short password", input: withPassword(valid, "short")},
		{name: "bcrypt password limit", input: withPassword(valid, strings.Repeat("x", 73))},
		{name: "missing full name", input: withFullName(valid, "")},
		{name: "missing position", input: withPosition(valid, "")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validate(test.input); err == nil {
				t.Fatal("validate() error = nil, want a validation error")
			}
		})
	}
}

func withUsername(input CreateInput, username string) CreateInput {
	input.Username = username
	return input
}

func withPassword(input CreateInput, password string) CreateInput {
	input.Password = password
	return input
}

func withFullName(input CreateInput, fullName string) CreateInput {
	input.FullName = fullName
	return input
}

func withPosition(input CreateInput, position string) CreateInput {
	input.Position = position
	return input
}
