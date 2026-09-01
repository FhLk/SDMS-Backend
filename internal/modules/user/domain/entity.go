package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleDirector Role = "DIRECTOR"
	RoleTeacher  Role = "TEARCHER"
)

type Status string

const (
	StatusActive   Status = "ACTIVE"
	StatusInactive Status = "INACTIVE"
)

type User struct {
	UID          uuid.UUID
	Username     string
	EmployeeCode string
	Prefix       string
	FirstName    string
	LastName     string
	Role         Role
	Status       Status
	PasswordHash string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
