package model

import (
	"strings"
	"unicode"

	"gorm.io/gorm"
)

// DisplayName returns the human-readable name without persisting a duplicate
// full_name column. Thai prefixes are conventionally attached to the first
// name, while Latin prefixes are separated by a space.
func (u User) DisplayName() string {
	if value := FormatPersonName(u.Prefix, u.FirstName, u.LastName); value != "" {
		return value
	}
	return strings.TrimSpace(u.FullName)
}

func FormatPersonName(prefix, firstName, lastName string) string {
	prefix = strings.TrimSpace(prefix)
	firstName = strings.TrimSpace(firstName)
	lastName = strings.TrimSpace(lastName)

	name := strings.Join(nonEmpty(firstName, lastName), " ")
	if prefix == "" {
		return name
	}
	if name == "" {
		return prefix
	}
	if asciiOnly(prefix) {
		return prefix + " " + name
	}
	return prefix + name
}

// BeforeSave keeps legacy internal callers working while the persisted schema
// remains normalized. A legacy FullName is preserved verbatim in first_name.
func (u *User) BeforeSave(_ *gorm.DB) error {
	u.Prefix = strings.TrimSpace(u.Prefix)
	u.FirstName = strings.TrimSpace(u.FirstName)
	u.LastName = strings.TrimSpace(u.LastName)
	if u.FirstName == "" && strings.TrimSpace(u.FullName) != "" {
		u.FirstName = strings.TrimSpace(u.FullName)
	}
	u.FullName = u.DisplayName()
	return nil
}

func (u *User) AfterFind(_ *gorm.DB) error {
	u.FullName = u.DisplayName()
	return nil
}

func nonEmpty(values ...string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func asciiOnly(value string) bool {
	for _, character := range value {
		if character > unicode.MaxASCII {
			return false
		}
	}
	return true
}
