package http

import (
	"time"

	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type UserResponse struct {
	UID          uuid.UUID `json:"uid"`
	Username     string    `json:"username"`
	EmployeeCode string    `json:"employee_code"`
	Prefix       string    `json:"prefix"`
	FirstName    string    `json:"first_name"`
	LastName     string    `json:"last_name"`
	Role         string    `json:"role"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type ErrorResponse struct {
	Message string `json:"message"`
}

type MessageResponse struct {
	Message string `json:"message"`
}

func newUserResponse(user *domain.User) UserResponse {
	return UserResponse{
		UID:          user.UID,
		Username:     user.Username,
		EmployeeCode: user.EmployeeCode,
		Prefix:       user.Prefix,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Role:         string(user.Role),
		Status:       string(user.Status),
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}

func newUserListResponse(users []domain.User) []UserResponse {
	responses := make([]UserResponse, 0, len(users))

	for i := range users {
		responses = append(responses, newUserResponse(&users[i]))
	}

	return responses
}
