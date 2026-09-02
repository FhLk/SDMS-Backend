package usecase

import (
	"context"
	"errors"
	"strings"

	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type UserService struct {
	repo domain.UserRepository
}

func NewUserService(repo domain.UserRepository) *UserService {
	return &UserService{
		repo: repo,
	}
}

// ============================================================
// Create
// ============================================================

func (s *UserService) Create(ctx context.Context, input CreateUserInput) (*domain.User, error) {
	input.Username = strings.TrimSpace(input.Username)
	input.EmployeeCode = strings.TrimSpace(input.EmployeeCode)
	input.Prefix = strings.TrimSpace(input.Prefix)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Role = domain.Role(strings.TrimSpace(string(input.Role)))

	if err := validateCreateInput(input); err != nil {
		return nil, err
	}

	if err := s.ensureUsernameAvailable(
		ctx,
		input.Username,
		uuid.Nil,
	); err != nil {
		return nil, err
	}

	if err := s.ensureEmployeeCodeAvailable(
		ctx,
		input.EmployeeCode,
		uuid.Nil,
	); err != nil {
		return nil, err
	}

	user := &domain.User{
		UID:          uuid.New(),
		Username:     input.Username,
		EmployeeCode: input.EmployeeCode,
		Prefix:       input.Prefix,
		FirstName:    input.FirstName,
		LastName:     input.LastName,
		Role:         input.Role,
		Status:       domain.StatusActive,
	}

	if err := s.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ============================================================
// List
// ============================================================

func (s *UserService) List(ctx context.Context) ([]domain.User, error) {
	users, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}

	return users, nil
}

// ============================================================
// GetByID
// ============================================================

func (s *UserService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, domain.ErrInvalidUserID
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// ============================================================
// GetByUsername
// ============================================================

func (s *UserService) GetByUsername(ctx context.Context, username string) (*domain.User, error) {
	username = strings.TrimSpace(username)

	if username == "" {
		return nil, domain.ErrUsernameRequired
	}

	user, err := s.repo.FindByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// ============================================================
// Update
// ============================================================

func (s *UserService) Update(ctx context.Context, id uuid.UUID, input UpdateUserInput) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, domain.ErrInvalidUserID
	}

	input.Username = strings.TrimSpace(input.Username)
	input.EmployeeCode = strings.TrimSpace(input.EmployeeCode)
	input.Prefix = strings.TrimSpace(input.Prefix)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Role = domain.Role(strings.TrimSpace(string(input.Role)))

	if err := validateUpdateInput(input); err != nil {
		return nil, err
	}

	// ต้องหา user เดิมก่อน
	existingUser, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// username เปลี่ยนหรือไม่
	if input.Username != existingUser.Username {
		if err := s.ensureUsernameAvailable(
			ctx,
			input.Username,
			id,
		); err != nil {
			return nil, err
		}
	}

	// employee code เปลี่ยนหรือไม่
	if input.EmployeeCode != existingUser.EmployeeCode {
		if err := s.ensureEmployeeCodeAvailable(ctx, input.EmployeeCode, id); err != nil {
			return nil, err
		}
	}

	existingUser.Username = input.Username
	existingUser.EmployeeCode = input.EmployeeCode
	existingUser.Prefix = input.Prefix
	existingUser.FirstName = input.FirstName
	existingUser.LastName = input.LastName
	existingUser.Role = input.Role

	if err := s.repo.Update(ctx, existingUser); err != nil {
		return nil, err
	}

	return existingUser, nil
}

// ============================================================
// Update Status
// ============================================================

func (s *UserService) UpdateStatus(ctx context.Context, id uuid.UUID, input UpdateUserStatusInput) (*domain.User, error) {
	if id == uuid.Nil {
		return nil, domain.ErrInvalidUserID
	}

	input.Status = domain.Status(strings.TrimSpace(string(input.Status)))

	if !isValidStatus(input.Status) {
		return nil, domain.ErrInvalidStatus
	}

	user, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	user.Status = input.Status

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

// ============================================================
// Delete
// ============================================================

func (s *UserService) Delete(ctx context.Context, id uuid.UUID) error {
	if id == uuid.Nil {
		return domain.ErrInvalidUserID
	}

	_, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	return nil
}

// ============================================================
// Private business helpers
// ============================================================

func (s *UserService) ensureUsernameAvailable(ctx context.Context, username string, currentUserID uuid.UUID) error {
	user, err := s.repo.FindByUsername(ctx, username)

	if errors.Is(err, domain.ErrUserNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	// ใช้ตอน update:
	// ถ้า username ที่เจอเป็นของตัวเอง ถือว่าใช้ได้
	if currentUserID != uuid.Nil &&
		user.UID == currentUserID {
		return nil
	}

	return domain.ErrUsernameAlreadyExists
}

func (s *UserService) ensureEmployeeCodeAvailable(ctx context.Context, employeeCode string, currentUserID uuid.UUID) error {
	user, err := s.repo.FindByEmployeeCode(
		ctx,
		employeeCode,
	)

	if errors.Is(err, domain.ErrUserNotFound) {
		return nil
	}

	if err != nil {
		return err
	}

	if currentUserID != uuid.Nil &&
		user.UID == currentUserID {
		return nil
	}

	return domain.ErrEmployeeCodeAlreadyExists
}

// ============================================================
// Validation
// ============================================================

func validateCreateInput(input CreateUserInput) error {
	if input.Username == "" {
		return domain.ErrUsernameRequired
	}

	if input.EmployeeCode == "" {
		return domain.ErrEmployeeCodeRequired
	}

	if input.Prefix == "" {
		return domain.ErrPrefixRequired
	}

	if input.FirstName == "" {
		return domain.ErrFirstNameRequired
	}

	if input.LastName == "" {
		return domain.ErrLastNameRequired
	}

	if !isValidRole(input.Role) {
		return domain.ErrInvalidRole
	}

	return nil
}

func validateUpdateInput(input UpdateUserInput) error {
	if input.Username == "" {
		return domain.ErrUsernameRequired
	}

	if input.EmployeeCode == "" {
		return domain.ErrEmployeeCodeRequired
	}

	if input.Prefix == "" {
		return domain.ErrPrefixRequired
	}

	if input.FirstName == "" {
		return domain.ErrFirstNameRequired
	}

	if input.LastName == "" {
		return domain.ErrLastNameRequired
	}

	if !isValidRole(input.Role) {
		return domain.ErrInvalidRole
	}

	return nil
}

func isValidRole(role domain.Role) bool {
	switch role {
	case domain.RoleDirector,
		domain.RoleTeacher:
		return true

	default:
		return false
	}
}

func isValidStatus(status domain.Status) bool {
	switch status {
	case domain.StatusActive,
		domain.StatusInactive:
		return true

	default:
		return false
	}
}
