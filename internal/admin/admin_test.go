package admin

import (
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	valid := CreateInput{
		Username:  "admin",
		Password:  "sdms-admin!",
		FirstName: "System",
		LastName:  "Administrator",
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
		{name: "mixed name formats", input: CreateInput{Username: "admin", Password: "sdms-admin!", FirstName: "System", FullName: "System Administrator"}},
		{name: "missing first name", input: withFirstName(valid, "")},
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

func withFirstName(input CreateInput, firstName string) CreateInput {
	input.FirstName = firstName
	return input
}
