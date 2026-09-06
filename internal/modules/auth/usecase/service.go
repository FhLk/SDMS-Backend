package usecase

import (
	"context"
	"errors"
	"strings"

	authdomain "sdms/internal/modules/auth/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (*userdomain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*userdomain.User, error)
	Update(ctx context.Context, user *userdomain.User) error
}

type TokenIssuer interface {
	Generate(userID uuid.UUID) (string, error)
}

type Service struct {
	users  UserRepository
	tokens TokenIssuer
}

func NewService(users UserRepository, tokens TokenIssuer) *Service {
	return &Service{users: users, tokens: tokens}
}

func (s *Service) Login(ctx context.Context, username, password string) (*userdomain.User, string, error) {
	username = strings.TrimSpace(username)
	if username == "" || password == "" {
		return nil, "", authdomain.ErrInvalidCredentials
	}

	user, err := s.users.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			return nil, "", authdomain.ErrInvalidCredentials
		}
		return nil, "", err
	}

	if user.Status != userdomain.StatusActive {
		return nil, "", authdomain.ErrInactiveUser
	}
	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return nil, "", authdomain.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.UID)
	if err != nil {
		return nil, "", err
	}
	return user, token, nil
}

func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	if userID == uuid.Nil {
		return authdomain.ErrInvalidCredentials
	}
	if currentPassword == "" {
		return authdomain.ErrInvalidCredentials
	}
	if strings.TrimSpace(newPassword) == "" {
		return authdomain.ErrPasswordRequired
	}
	if len(newPassword) < 8 {
		return authdomain.ErrPasswordTooShort
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			return authdomain.ErrInvalidCredentials
		}
		return err
	}
	if user.PasswordHash == "" || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)) != nil {
		return authdomain.ErrInvalidCredentials
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.PasswordHash = string(hash)
	return s.users.Update(ctx, user)
}
