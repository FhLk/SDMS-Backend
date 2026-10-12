// Package auth จัดการ login, server-side session และข้อมูลผู้ใช้ปัจจุบัน.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrInvalidCredentials intentionally covers missing users, wrong passwords,
	// and deactivated users so callers cannot enumerate account state.
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUnauthenticated    = errors.New("authentication required")
	dummyPasswordHash     = mustDummyPasswordHash()
)

// LoginInput contains credentials and audit metadata for a new session.
type LoginInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
}

// LoginResult returns the opaque token once; only its hash is stored in PostgreSQL.
type LoginResult struct {
	Token     string
	ExpiresAt time.Time
}

// Profile contains the user and organization data returned by GET /api/me.
type Profile struct {
	User          model.User
	Divisions     []model.Division
	HeadDivisions []model.Division
}

// Service owns authentication rules and persistent session operations.
type Service struct {
	db         *gorm.DB
	sessionTTL time.Duration
	now        func() time.Time
	random     io.Reader
}

// New creates an authentication service backed by PostgreSQL through GORM.
func New(db *gorm.DB, sessionTTL time.Duration) *Service {
	return &Service{
		db:         db,
		sessionTTL: sessionTTL,
		now:        time.Now,
		random:     rand.Reader,
	}
}

// Login verifies bcrypt credentials and creates a new server-side session.
func (s *Service) Login(ctx context.Context, input LoginInput) (LoginResult, error) {
	username := strings.ToLower(strings.TrimSpace(input.Username))

	var user model.User
	query := s.db.WithContext(ctx).Where("username = ?", username).Limit(1).Find(&user)
	if query.Error != nil {
		return LoginResult{}, fmt.Errorf("find login user: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		// Run bcrypt even when the user is absent to reduce username timing leaks.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash, []byte(input.Password))
		return LoginResult{}, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(input.Password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	if user.DeactivatedAt != nil {
		return LoginResult{}, ErrInvalidCredentials
	}

	token, tokenHash, err := newSessionToken(s.random)
	if err != nil {
		return LoginResult{}, fmt.Errorf("generate session token: %w", err)
	}
	now := s.now()
	expiresAt := now.Add(s.sessionTTL)
	session := model.Session{
		TokenHash:  tokenHash,
		UserUUID:   user.UUID,
		ExpiresAt:  expiresAt,
		LastSeenAt: now,
		IP:         truncate(input.IP, 45),
		UserAgent:  truncate(input.UserAgent, 300),
		CreatedAt:  now,
	}
	if err := s.db.WithContext(ctx).Create(&session).Error; err != nil {
		return LoginResult{}, fmt.Errorf("create session: %w", err)
	}

	return LoginResult{Token: token, ExpiresAt: expiresAt}, nil
}

// Logout revokes the supplied token. Missing or already-revoked tokens are successful.
func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.db.WithContext(ctx).
		Where("token_hash = ?", hashToken(token)).
		Delete(&model.Session{}).Error; err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// CurrentUser validates a session and returns the user's divisions and headed divisions.
func (s *Service) CurrentUser(ctx context.Context, token string) (Profile, error) {
	if token == "" {
		return Profile{}, ErrUnauthenticated
	}

	var session model.Session
	query := s.db.WithContext(ctx).
		Preload("User.Divisions", func(db *gorm.DB) *gorm.DB {
			return db.Order("divisions.sort_order")
		}).
		Where("token_hash = ?", hashToken(token)).
		Limit(1).
		Find(&session)
	if query.Error != nil {
		return Profile{}, fmt.Errorf("find session: %w", query.Error)
	}
	if query.RowsAffected == 0 {
		return Profile{}, ErrUnauthenticated
	}

	now := s.now()
	if !session.ExpiresAt.After(now) || session.User.DeactivatedAt != nil {
		// Expired and deactivated sessions are removed eagerly when encountered.
		_ = s.db.WithContext(ctx).Where("token_hash = ?", session.TokenHash).Delete(&model.Session{}).Error
		return Profile{}, ErrUnauthenticated
	}

	var headDivisions []model.Division
	if err := s.db.WithContext(ctx).
		Where("head_user_uuid = ?", session.UserUUID).
		Order("sort_order").
		Find(&headDivisions).Error; err != nil {
		return Profile{}, fmt.Errorf("find headed divisions: %w", err)
	}

	// Avoid a database write on every request while keeping LastSeenAt useful.
	if now.Sub(session.LastSeenAt) >= 5*time.Minute {
		if err := s.db.WithContext(ctx).Model(&model.Session{}).
			Where("token_hash = ?", session.TokenHash).
			UpdateColumn("last_seen_at", now).Error; err != nil {
			return Profile{}, fmt.Errorf("update session activity: %w", err)
		}
	}

	return Profile{
		User:          session.User,
		Divisions:     session.User.Divisions,
		HeadDivisions: headDivisions,
	}, nil
}

func newSessionToken(random io.Reader) (token string, tokenHash string, err error) {
	bytes := make([]byte, 32)
	if _, err := io.ReadFull(random, bytes); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(bytes)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func truncate(value string, maximum int) string {
	runes := []rune(value)
	if len(runes) <= maximum {
		return value
	}
	return string(runes[:maximum])
}

func mustDummyPasswordHash() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("timing-only-password"), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	return hash
}
