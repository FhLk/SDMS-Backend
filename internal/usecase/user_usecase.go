package usecase

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/FhLk/SDMS-Backend/internal/domain"
)

const (
	defaultPageSize = 20
	maxPageSize     = 100
)

type CreateUserInput struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UpdateUserInput struct {
	Name  *string `json:"name"`
	Email *string `json:"email"`
}

type UserUsecase struct {
	repository domain.UserRepository
}

func NewUserUsecase(repository domain.UserRepository) *UserUsecase {
	return &UserUsecase{repository: repository}
}

func (u *UserUsecase) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	name := strings.TrimSpace(input.Name)
	email := normalizeEmail(input.Email)
	if err := validateName(name); err != nil {
		return nil, err
	}
	if err := validateEmail(email); err != nil {
		return nil, err
	}

	user := &domain.User{Name: name, Email: email}
	if err := u.repository.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *UserUsecase) GetByID(ctx context.Context, id uint) (*domain.User, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be greater than zero", domain.ErrInvalidInput)
	}
	return u.repository.FindByID(ctx, id)
}

func (u *UserUsecase) List(ctx context.Context, page, pageSize int) ([]domain.User, int64, int, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	users, total, err := u.repository.List(ctx, (page-1)*pageSize, pageSize)
	return users, total, page, pageSize, err
}

func (u *UserUsecase) Update(ctx context.Context, id uint, input UpdateUserInput) (*domain.User, error) {
	if id == 0 {
		return nil, fmt.Errorf("%w: id must be greater than zero", domain.ErrInvalidInput)
	}
	if input.Name == nil && input.Email == nil {
		return nil, fmt.Errorf("%w: at least one field is required", domain.ErrInvalidInput)
	}

	user, err := u.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if err := validateName(name); err != nil {
			return nil, err
		}
		user.Name = name
	}
	if input.Email != nil {
		email := normalizeEmail(*input.Email)
		if err := validateEmail(email); err != nil {
			return nil, err
		}
		user.Email = email
	}

	if err := u.repository.Update(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (u *UserUsecase) Delete(ctx context.Context, id uint) error {
	if id == 0 {
		return fmt.Errorf("%w: id must be greater than zero", domain.ErrInvalidInput)
	}
	return u.repository.Delete(ctx, id)
}

func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is required", domain.ErrInvalidInput)
	}
	if len([]rune(name)) > 100 {
		return fmt.Errorf("%w: name must be at most 100 characters", domain.ErrInvalidInput)
	}
	return nil
}

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("%w: email is required", domain.ErrInvalidInput)
	}
	address, err := mail.ParseAddress(email)
	if err != nil || !strings.EqualFold(address.Address, email) {
		return fmt.Errorf("%w: email is invalid", domain.ErrInvalidInput)
	}
	if len(email) > 254 {
		return fmt.Errorf("%w: email must be at most 254 characters", domain.ErrInvalidInput)
	}
	return nil
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
