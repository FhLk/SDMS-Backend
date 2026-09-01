package usecase

import (
	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type CreateUserInput struct {
	Username     string
	EmployeeCode string
	FirstName    string
	LastName     string
	Role         domain.Role
}

type UpdateUserInput struct {
	UID          uuid.UUID
	Username     string
	EmployeeCode string
	FirstName    string
	LastName     string
	Role         domain.Role
}

type UpdateUserStatusInput struct {
	UID    uuid.UUID
	Status domain.Status
}
