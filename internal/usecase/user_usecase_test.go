package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/domain"
)

type fakeUserRepository struct {
	created *domain.User
	user    *domain.User
	err     error
}

func (f *fakeUserRepository) Create(_ context.Context, user *domain.User) error {
	if f.err != nil {
		return f.err
	}
	user.ID = 1
	f.created = user
	return nil
}

func (f *fakeUserRepository) FindByID(_ context.Context, _ uint) (*domain.User, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.user, nil
}

func (f *fakeUserRepository) List(_ context.Context, _, _ int) ([]domain.User, int64, error) {
	return nil, 0, f.err
}

func (f *fakeUserRepository) Update(_ context.Context, user *domain.User) error {
	f.user = user
	return f.err
}

func (f *fakeUserRepository) Delete(_ context.Context, _ uint) error { return f.err }

func TestCreateUserNormalizesInput(t *testing.T) {
	repository := &fakeUserRepository{}
	service := NewUserUsecase(repository)

	user, err := service.Create(context.Background(), CreateUserInput{
		Name:  "  Jane Doe  ",
		Email: " JANE@EXAMPLE.COM ",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if user.ID != 1 || user.Name != "Jane Doe" || user.Email != "jane@example.com" {
		t.Fatalf("Create() user = %#v", user)
	}
}

func TestCreateUserRejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name  string
		input CreateUserInput
	}{
		{name: "missing name", input: CreateUserInput{Email: "jane@example.com"}},
		{name: "invalid email", input: CreateUserInput{Name: "Jane", Email: "invalid"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := NewUserUsecase(&fakeUserRepository{})
			_, err := service.Create(context.Background(), tt.input)
			if !errors.Is(err, domain.ErrInvalidInput) {
				t.Fatalf("Create() error = %v, want ErrInvalidInput", err)
			}
		})
	}
}

func TestUpdateUserRequiresAField(t *testing.T) {
	service := NewUserUsecase(&fakeUserRepository{})
	_, err := service.Update(context.Background(), 1, UpdateUserInput{})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("Update() error = %v, want ErrInvalidInput", err)
	}
}

func TestListUserClampsPagination(t *testing.T) {
	service := NewUserUsecase(&fakeUserRepository{})
	_, _, page, pageSize, err := service.List(context.Background(), 0, 1000)
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if page != 1 || pageSize != maxPageSize {
		t.Fatalf("List() page = %d, pageSize = %d", page, pageSize)
	}
}
