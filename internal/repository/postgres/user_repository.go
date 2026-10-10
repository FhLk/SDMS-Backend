package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/domain"
	"gorm.io/gorm"
)

type userModel struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:100;not null"`
	Email     string    `gorm:"size:254;not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

func (userModel) TableName() string { return "users" }

type UserRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&userModel{})
}

func (r *UserRepository) Create(ctx context.Context, user *domain.User) error {
	model := fromDomain(user)
	if err := r.db.WithContext(ctx).Create(&model).Error; err != nil {
		return translateError(err)
	}
	*user = toDomain(model)
	return nil
}

func (r *UserRepository) FindByID(ctx context.Context, id uint) (*domain.User, error) {
	var model userModel
	if err := r.db.WithContext(ctx).First(&model, id).Error; err != nil {
		return nil, translateError(err)
	}
	user := toDomain(model)
	return &user, nil
}

func (r *UserRepository) List(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&userModel{}).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count users: %w", err)
	}

	var models []userModel
	if err := r.db.WithContext(ctx).
		Order("id ASC").
		Offset(offset).
		Limit(limit).
		Find(&models).Error; err != nil {
		return nil, 0, fmt.Errorf("list users: %w", err)
	}

	users := make([]domain.User, 0, len(models))
	for _, model := range models {
		users = append(users, toDomain(model))
	}
	return users, total, nil
}

func (r *UserRepository) Update(ctx context.Context, user *domain.User) error {
	model := fromDomain(user)
	result := r.db.WithContext(ctx).Model(&userModel{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{"name": model.Name, "email": model.Email})
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}

	if err := r.db.WithContext(ctx).First(&model, user.ID).Error; err != nil {
		return translateError(err)
	}
	*user = toDomain(model)
	return nil
}

func (r *UserRepository) Delete(ctx context.Context, id uint) error {
	result := r.db.WithContext(ctx).Delete(&userModel{}, id)
	if result.Error != nil {
		return translateError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func fromDomain(user *domain.User) userModel {
	return userModel{
		ID:        user.ID,
		Name:      user.Name,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}
}

func toDomain(model userModel) domain.User {
	return domain.User{
		ID:        model.ID,
		Name:      model.Name,
		Email:     model.Email,
		CreatedAt: model.CreatedAt,
		UpdatedAt: model.UpdatedAt,
	}
}

func translateError(err error) error {
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return domain.ErrNotFound
	case errors.Is(err, gorm.ErrDuplicatedKey):
		return fmt.Errorf("%w: email already exists", domain.ErrConflict)
	default:
		return err
	}
}
