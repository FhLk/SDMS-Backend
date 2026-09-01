package postgres

import (
	"sdms/internal/modules/user/domain"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type UserModel struct {
	UID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Username     string    `gorm:"size:255;uniqueIndex;not null"`
	EmployeeCode string    `gorm:"size:100;uniqueIndex;not null"`
	Prefix       string    `gorm:"size:100;not null"`
	FirstName    string    `gorm:"size:255;not null"`
	LastName     string    `gorm:"size:255;not null"`
	Role         string    `gorm:"size:50;index;not null"`
	Status       string    `gorm:"size:50;index;not null"`
	PasswordHash string    `gorm:"size:255"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    gorm.DeletedAt `gorm:"index"`
}

func (UserModel) TableName() string {
	return "users"
}

func toDomain(model *UserModel) *domain.User {
	return &domain.User{
		UID:          model.UID,
		Username:     model.Username,
		EmployeeCode: model.EmployeeCode,
		Prefix:       model.Prefix,
		FirstName:    model.FirstName,
		LastName:     model.LastName,
		Role:         domain.Role(model.Role),
		Status:       domain.Status(model.Status),
		PasswordHash: model.PasswordHash,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
}

func toModel(user *domain.User) *UserModel {
	return &UserModel{
		UID:          user.UID,
		Username:     user.Username,
		EmployeeCode: user.EmployeeCode,
		Prefix:       user.Prefix,
		FirstName:    user.FirstName,
		LastName:     user.LastName,
		Role:         string(user.Role),
		Status:       string(user.Status),
		PasswordHash: user.PasswordHash,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}
