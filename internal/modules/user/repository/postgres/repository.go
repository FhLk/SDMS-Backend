package postgres

import (
	"context"
	"errors"

	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type userRepository struct {
	db *gorm.DB
}

func NewUserRepository(db *gorm.DB) *userRepository {
	return &userRepository{
		db: db,
	}
}

// ============================================================
// Create
// ============================================================

func (r *userRepository) Create(
	ctx context.Context,
	user *domain.User,
) error {
	model := toModel(user)

	if err := r.db.
		WithContext(ctx).
		Create(model).
		Error; err != nil {
		return err
	}

	// sync generated values from database back to domain
	user.UID = model.UID
	user.CreatedAt = model.CreatedAt
	user.UpdatedAt = model.UpdatedAt

	return nil
}

// ============================================================
// List
// ============================================================

func (r *userRepository) List(
	ctx context.Context,
) ([]domain.User, error) {
	var models []UserModel

	if err := r.db.
		WithContext(ctx).
		Order("created_at DESC").
		Find(&models).
		Error; err != nil {
		return nil, err
	}

	users := make([]domain.User, 0, len(models))

	for i := range models {
		user := toDomain(&models[i])
		users = append(users, *user)
	}

	return users, nil
}

// ============================================================
// FindByID
// ============================================================

func (r *userRepository) FindByID(
	ctx context.Context,
	id uuid.UUID,
) (*domain.User, error) {
	var model UserModel

	err := r.db.
		WithContext(ctx).
		Where("uid = ?", id).
		First(&model).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return toDomain(&model), nil
}

// ============================================================
// FindByUsername
// ============================================================

func (r *userRepository) FindByUsername(
	ctx context.Context,
	username string,
) (*domain.User, error) {
	var model UserModel

	err := r.db.
		WithContext(ctx).
		Where("username = ?", username).
		First(&model).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return toDomain(&model), nil
}

// ============================================================
// FindByEmployeeCode
// ============================================================

func (r *userRepository) FindByEmployeeCode(
	ctx context.Context,
	employeeCode string,
) (*domain.User, error) {
	var model UserModel

	err := r.db.
		WithContext(ctx).
		Where("employee_code = ?", employeeCode).
		First(&model).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}

	if err != nil {
		return nil, err
	}

	return toDomain(&model), nil
}

// ============================================================
// Update
// ============================================================

func (r *userRepository) Update(
	ctx context.Context,
	user *domain.User,
) error {
	model := toModel(user)

	result := r.db.
		WithContext(ctx).
		Model(&UserModel{}).
		Where("uid = ?", user.UID).
		Select("*").
		Omit(
			"uid",
			"created_at",
			"deleted_at",
		).
		Updates(model)

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	// ดึงข้อมูลกลับมาอีกครั้งเพื่อให้ domain
	// ได้ UpdatedAt และค่าที่ database จัดการให้
	var updatedModel UserModel

	err := r.db.
		WithContext(ctx).
		Where("uid = ?", user.UID).
		First(&updatedModel).
		Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domain.ErrUserNotFound
	}

	if err != nil {
		return err
	}

	updatedUser := toDomain(&updatedModel)

	*user = *updatedUser

	return nil
}

// ============================================================
// Delete
// ============================================================

func (r *userRepository) Delete(
	ctx context.Context,
	id uuid.UUID,
) error {
	result := r.db.
		WithContext(ctx).
		Where("uid = ?", id).
		Delete(&UserModel{})

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrUserNotFound
	}

	return nil
}
