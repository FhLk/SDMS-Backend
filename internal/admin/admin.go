// Package admin มี use case สำหรับสร้างผู้ดูแลระบบคนแรก.
package admin

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	usernamePattern   = regexp.MustCompile(`^[a-z0-9._-]{3,32}$`)
	ErrUsernameExists = errors.New("username already exists")
)

// CreateInput คือข้อมูลที่จำเป็นสำหรับสร้างบัญชีแอดมิน.
type CreateInput struct {
	Username string
	Password string
	FullName string
	Position string
}

// Create ตรวจข้อมูล, hash รหัสผ่านด้วย bcrypt และบันทึกบัญชีแอดมิน.
func Create(ctx context.Context, db *gorm.DB, input CreateInput) (*model.User, error) {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.FullName = strings.TrimSpace(input.FullName)
	input.Position = strings.TrimSpace(input.Position)

	if err := validate(input); err != nil {
		return nil, err
	}

	var count int64
	if err := db.WithContext(ctx).Model(&model.User{}).
		Where("username = ?", input.Username).
		Count(&count).Error; err != nil {
		return nil, fmt.Errorf("check existing username: %w", err)
	}
	if count > 0 {
		return nil, ErrUsernameExists
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := model.User{
		Username:     input.Username,
		PasswordHash: string(passwordHash),
		FullName:     input.FullName,
		Position:     input.Position,
		IsAdmin:      true,
	}
	if err := db.WithContext(ctx).Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return nil, ErrUsernameExists
		}
		return nil, fmt.Errorf("create admin: %w", err)
	}

	return &user, nil
}

func validate(input CreateInput) error {
	if err := validateUsername(input.Username); err != nil {
		return err
	}
	if err := validatePassword(input.Password); err != nil {
		return err
	}
	if input.FullName == "" {
		return errors.New("full name is required")
	}
	if input.Position == "" {
		return errors.New("position is required")
	}
	return nil
}

func validateUsername(username string) error {
	if !usernamePattern.MatchString(username) {
		return errors.New("username must be 3-32 characters using lowercase letters, numbers, dot, underscore, or hyphen")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return errors.New("password must contain at least 8 characters")
	}
	// bcrypt รับข้อมูลได้สูงสุด 72 bytes; ตรวจล่วงหน้าเพื่อให้ข้อความชัดเจน.
	if len([]byte(password)) > 72 {
		return errors.New("password must not exceed 72 bytes")
	}
	return nil
}
