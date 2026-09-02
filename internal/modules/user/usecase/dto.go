package usecase

import "sdms/internal/modules/user/domain"

type CreateUserInput struct {
	Username     string
	EmployeeCode string
	Prefix       string
	FirstName    string
	LastName     string
	Role         domain.Role
}

type UpdateUserInput struct {
	Username     string
	EmployeeCode string
	Prefix       string
	FirstName    string
	LastName     string
	Role         domain.Role
}

type UpdateUserStatusInput struct {
	Status domain.Status
}
