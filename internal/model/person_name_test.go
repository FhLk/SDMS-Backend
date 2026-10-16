package model

import "testing"

func TestFormatPersonName(t *testing.T) {
	tests := []struct {
		name      string
		prefix    string
		firstName string
		lastName  string
		want      string
	}{
		{name: "Thai prefix", prefix: "นาง", firstName: "สมหญิง", lastName: "ใจดี", want: "นางสมหญิง ใจดี"},
		{name: "Latin prefix", prefix: "Mr.", firstName: "John", lastName: "Smith", want: "Mr. John Smith"},
		{name: "without prefix", firstName: "สมชาย", lastName: "ใจดี", want: "สมชาย ใจดี"},
		{name: "trim fields", prefix: " นาง ", firstName: " สมหญิง ", lastName: " ใจดี ", want: "นางสมหญิง ใจดี"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatPersonName(tt.prefix, tt.firstName, tt.lastName); got != tt.want {
				t.Fatalf("FormatPersonName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestUserBeforeSavePreservesLegacyFullName(t *testing.T) {
	user := User{FullName: "  ผู้ดูแลระบบ  "}
	if err := user.BeforeSave(nil); err != nil {
		t.Fatalf("BeforeSave() error = %v", err)
	}
	if user.FirstName != "ผู้ดูแลระบบ" || user.Prefix != "" || user.LastName != "" || user.DisplayName() != "ผู้ดูแลระบบ" {
		t.Fatalf("user name = %+v, want legacy value stored in first_name", user)
	}
}
