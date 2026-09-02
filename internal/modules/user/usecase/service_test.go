package usecase

import (
	"context"
	"errors"
	"testing"

	"sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type fakeUserRepository struct {
	createFn             func(context.Context, *domain.User) error
	findByIDFn           func(context.Context, uuid.UUID) (*domain.User, error)
	findByUsernameFn     func(context.Context, string) (*domain.User, error)
	findByEmployeeCodeFn func(context.Context, string) (*domain.User, error)
	listFn               func(context.Context) ([]domain.User, error)
	updateFn             func(context.Context, *domain.User) error
	deleteFn             func(context.Context, uuid.UUID) error
}

func (f *fakeUserRepository) Create(ctx context.Context, user *domain.User) error {
	if f.createFn != nil {
		return f.createFn(ctx, user)
	}
	return nil
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	if f.findByIDFn != nil {
		return f.findByIDFn(ctx, id)
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	if f.findByUsernameFn != nil {
		return f.findByUsernameFn(ctx, username)
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepository) FindByEmployeeCode(ctx context.Context, employeeCode string) (*domain.User, error) {
	if f.findByEmployeeCodeFn != nil {
		return f.findByEmployeeCodeFn(ctx, employeeCode)
	}
	return nil, domain.ErrUserNotFound
}

func (f *fakeUserRepository) List(ctx context.Context) ([]domain.User, error) {
	if f.listFn != nil {
		return f.listFn(ctx)
	}
	return []domain.User{}, nil
}

func (f *fakeUserRepository) Update(ctx context.Context, user *domain.User) error {
	if f.updateFn != nil {
		return f.updateFn(ctx, user)
	}
	return nil
}

func (f *fakeUserRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if f.deleteFn != nil {
		return f.deleteFn(ctx, id)
	}
	return nil
}

func TestUserServiceCreateTrimsAndPersistsAllFields(t *testing.T) {
	if domain.RoleTeacher != "TEACHER" {
		t.Fatalf("RoleTeacher = %q, want %q", domain.RoleTeacher, "TEACHER")
	}

	var created *domain.User
	repository := &fakeUserRepository{
		createFn: func(_ context.Context, user *domain.User) error {
			created = user
			return nil
		},
	}
	service := NewUserService(repository)

	user, err := service.Create(context.Background(), CreateUserInput{
		Username:     "  somchai  ",
		EmployeeCode: "  EMP-001  ",
		Prefix:       "  นาย  ",
		FirstName:    "  สมชาย  ",
		LastName:     "  ใจดี  ",
		Role:         domain.RoleTeacher,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created == nil || user != created {
		t.Fatal("repository did not receive the created user")
	}
	if user.UID == uuid.Nil {
		t.Error("UID was not generated")
	}
	if user.Username != "somchai" || user.EmployeeCode != "EMP-001" {
		t.Errorf("identifiers were not trimmed: %+v", user)
	}
	if user.Prefix != "นาย" || user.FirstName != "สมชาย" || user.LastName != "ใจดี" {
		t.Errorf("name fields were not persisted correctly: %+v", user)
	}
	if user.Role != domain.RoleTeacher {
		t.Errorf("Role = %q, want %q", user.Role, domain.RoleTeacher)
	}
	if user.Status != domain.StatusActive {
		t.Errorf("Status = %q, want %q", user.Status, domain.StatusActive)
	}
}

func TestUserServiceCreateRequiresPrefix(t *testing.T) {
	service := NewUserService(&fakeUserRepository{})

	_, err := service.Create(context.Background(), CreateUserInput{
		Username:     "somchai",
		EmployeeCode: "EMP-001",
		Prefix:       " ",
		FirstName:    "สมชาย",
		LastName:     "ใจดี",
		Role:         domain.RoleTeacher,
	})
	if !errors.Is(err, domain.ErrPrefixRequired) {
		t.Fatalf("Create() error = %v, want %v", err, domain.ErrPrefixRequired)
	}
}

func TestUserServiceUpdateUsesPathIDAndUpdatesPrefix(t *testing.T) {
	id := uuid.New()
	existing := &domain.User{
		UID:          id,
		Username:     "somchai",
		EmployeeCode: "EMP-001",
		Prefix:       "นาย",
		FirstName:    "สมชาย",
		LastName:     "ใจดี",
		Role:         domain.RoleTeacher,
		Status:       domain.StatusActive,
	}
	findCalled := false
	updateCalled := false
	repository := &fakeUserRepository{
		findByIDFn: func(_ context.Context, gotID uuid.UUID) (*domain.User, error) {
			findCalled = true
			if gotID != id {
				t.Errorf("FindByID() id = %s, want %s", gotID, id)
			}
			return existing, nil
		},
		updateFn: func(_ context.Context, user *domain.User) error {
			updateCalled = true
			return nil
		},
	}
	service := NewUserService(repository)

	user, err := service.Update(context.Background(), id, UpdateUserInput{
		Username:     "somchai",
		EmployeeCode: "EMP-001",
		Prefix:       " ดร. ",
		FirstName:    " สมชาย ",
		LastName:     " ใจดี ",
		Role:         domain.RoleDirector,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if !findCalled || !updateCalled {
		t.Fatal("Update() did not load and persist the user")
	}
	if user.Prefix != "ดร." || user.Role != domain.RoleDirector {
		t.Errorf("updated user = %+v", user)
	}
}

func TestUserServiceUpdateStatusUsesPathID(t *testing.T) {
	id := uuid.New()
	existing := &domain.User{UID: id, Status: domain.StatusActive}
	repository := &fakeUserRepository{
		findByIDFn: func(_ context.Context, gotID uuid.UUID) (*domain.User, error) {
			if gotID != id {
				t.Errorf("FindByID() id = %s, want %s", gotID, id)
			}
			return existing, nil
		},
	}
	service := NewUserService(repository)

	user, err := service.UpdateStatus(context.Background(), id, UpdateUserStatusInput{
		Status: " INACTIVE ",
	})
	if err != nil {
		t.Fatalf("UpdateStatus() error = %v", err)
	}
	if user.Status != domain.StatusInactive {
		t.Errorf("Status = %q, want %q", user.Status, domain.StatusInactive)
	}
}
