package usecase

import (
	"context"
	"errors"
	"testing"

	authdomain "sdms/internal/modules/auth/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type authUserRepoStub struct {
	findByUsernameFn func(context.Context, string) (*userdomain.User, error)
	findByIDFn       func(context.Context, uuid.UUID) (*userdomain.User, error)
	updateFn         func(context.Context, *userdomain.User) error
}

func (s *authUserRepoStub) FindByUsername(ctx context.Context, username string) (*userdomain.User, error) {
	if s.findByUsernameFn != nil {
		return s.findByUsernameFn(ctx, username)
	}
	return nil, userdomain.ErrUserNotFound
}

func (s *authUserRepoStub) FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error) {
	if s.findByIDFn != nil {
		return s.findByIDFn(ctx, id)
	}
	return nil, userdomain.ErrUserNotFound
}

func (s *authUserRepoStub) Update(ctx context.Context, user *userdomain.User) error {
	if s.updateFn != nil {
		return s.updateFn(ctx, user)
	}
	return nil
}

type tokenIssuerStub struct {
	token string
	err   error
}

func (s tokenIssuerStub) Generate(uuid.UUID) (string, error) {
	return s.token, s.err
}

func TestLoginSuccess(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	user := &userdomain.User{
		UID:          uuid.New(),
		Username:     "teacher01",
		Role:         userdomain.RoleTeacher,
		Status:       userdomain.StatusActive,
		PasswordHash: string(hash),
	}

	service := NewService(&authUserRepoStub{
		findByUsernameFn: func(_ context.Context, username string) (*userdomain.User, error) {
			if username != "teacher01" {
				t.Fatalf("username = %q", username)
			}
			return user, nil
		},
	}, tokenIssuerStub{token: "signed-token"})

	gotUser, token, err := service.Login(context.Background(), " teacher01 ", "password123")
	if err != nil {
		t.Fatalf("Login() error = %v", err)
	}
	if gotUser != user || token != "signed-token" {
		t.Fatalf("Login() = %+v, %q", gotUser, token)
	}
}

func TestLoginRejectsInvalidPassword(t *testing.T) {
	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.MinCost)
	service := NewService(&authUserRepoStub{
		findByUsernameFn: func(context.Context, string) (*userdomain.User, error) {
			return &userdomain.User{UID: uuid.New(), Status: userdomain.StatusActive, PasswordHash: string(hash)}, nil
		},
	}, tokenIssuerStub{token: "unused"})

	_, _, err := service.Login(context.Background(), "teacher01", "wrong-password")
	if !errors.Is(err, authdomain.ErrInvalidCredentials) {
		t.Fatalf("Login() error = %v, want ErrInvalidCredentials", err)
	}
}

func TestChangePassword(t *testing.T) {
	oldHash, _ := bcrypt.GenerateFromPassword([]byte("oldpassword"), bcrypt.MinCost)
	user := &userdomain.User{UID: uuid.New(), Status: userdomain.StatusActive, PasswordHash: string(oldHash)}
	updated := false

	service := NewService(&authUserRepoStub{
		findByIDFn: func(context.Context, uuid.UUID) (*userdomain.User, error) { return user, nil },
		updateFn: func(_ context.Context, got *userdomain.User) error {
			updated = true
			if bcrypt.CompareHashAndPassword([]byte(got.PasswordHash), []byte("newpassword")) != nil {
				t.Fatal("new password was not hashed correctly")
			}
			return nil
		},
	}, tokenIssuerStub{})

	if err := service.ChangePassword(context.Background(), user.UID, "oldpassword", "newpassword"); err != nil {
		t.Fatalf("ChangePassword() error = %v", err)
	}
	if !updated {
		t.Fatal("expected repository update")
	}
}
