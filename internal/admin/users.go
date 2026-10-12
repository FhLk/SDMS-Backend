package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrDivisionNotFound = errors.New("division not found")
)

// ValidationError ระบุ field ที่ไม่ผ่าน validation เพื่อให้ HTTP layer
// ตอบ 400 โดยไม่เปิดเผย error ภายในของฐานข้อมูล.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// CreateUserInput คือข้อมูลที่แอดมินใช้สร้างบุคลากรใหม่.
type CreateUserInput struct {
	Username      string
	Password      string
	FullName      string
	Position      string
	DivisionUUIDs []uuid.UUID
	IsExec        bool
	IsAdmin       bool
}

// UpdateUserInput ใช้ pointer เพื่อแยก field ที่ไม่ได้ส่งออกจากค่าศูนย์
// เช่น false หรือ slice ว่างที่ผู้เรียกตั้งใจใช้ล้างข้อมูลเดิม.
type UpdateUserInput struct {
	Username      *string
	FullName      *string
	Position      *string
	DivisionUUIDs *[]uuid.UUID
	IsExec        *bool
	IsAdmin       *bool
	IsActive      *bool
}

// UserService จัดการข้อมูลบุคลากร ฝ่าย และ session ที่เกี่ยวข้อง.
type UserService struct {
	db  *gorm.DB
	now func() time.Time
}

// NewUserService สร้าง service ที่ใช้ฐานข้อมูลเดียวกับ API.
func NewUserService(db *gorm.DB) *UserService {
	return &UserService{db: db, now: time.Now}
}

// ListUsers คืนบุคลากรทั้งหมด รวมบัญชีที่ปิดใช้งาน เพื่อให้แอดมินเปิดกลับได้.
func (s *UserService) ListUsers(ctx context.Context) ([]model.User, error) {
	users := make([]model.User, 0)
	err := s.db.WithContext(ctx).
		Preload("Divisions", func(db *gorm.DB) *gorm.DB {
			return db.Order("divisions.sort_order")
		}).
		Order("users.full_name").
		Order("users.username").
		Find(&users).Error
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	return users, nil
}

// CreateUser ตรวจข้อมูล สร้าง bcrypt hash และบันทึกผู้ใช้กับฝ่ายใน transaction เดียว.
func (s *UserService) CreateUser(ctx context.Context, input CreateUserInput) (model.User, error) {
	input = normalizeCreateUserInput(input)
	if err := validateCreateUserInput(input); err != nil {
		return model.User{}, err
	}

	divisionUUIDs, err := uniqueDivisionUUIDs(input.DivisionUUIDs)
	if err != nil {
		return model.User{}, err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return model.User{}, fmt.Errorf("hash user password: %w", err)
	}

	var created model.User
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := ensureDivisionsExist(tx, divisionUUIDs); err != nil {
			return err
		}

		user := model.User{
			Username:     input.Username,
			PasswordHash: string(passwordHash),
			FullName:     input.FullName,
			Position:     input.Position,
			IsExec:       input.IsExec,
			IsAdmin:      input.IsAdmin,
		}
		if err := tx.Create(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrDuplicatedKey) {
				return ErrUsernameExists
			}
			return fmt.Errorf("create user: %w", err)
		}
		if err := replaceUserDivisions(tx, user.UUID, divisionUUIDs); err != nil {
			return err
		}

		loaded, err := findUser(tx, user.UUID, false)
		if err != nil {
			return err
		}
		created = loaded
		return nil
	})
	if err != nil {
		return model.User{}, err
	}
	return created, nil
}

// UpdateUser แก้เฉพาะ field ที่ส่งมาและคงข้อมูลอื่นไว้ตาม semantics ของ PATCH.
// การปิดบัญชีจะยกเลิก session และเอาผู้ใช้ออกจากหัวข้อเปิดที่ยังไม่ได้ส่ง
// แต่ไม่ลบ submission เดิมหรือ assignment ที่มี submission แล้ว.
func (s *UserService) UpdateUser(ctx context.Context, userUUID uuid.UUID, input UpdateUserInput) (model.User, error) {
	input = normalizeUpdateUserInput(input)
	if err := validateUpdateUserInput(input); err != nil {
		return model.User{}, err
	}

	var updated model.User
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findUser(tx, userUUID, true); err != nil {
			return err
		}

		updates := make(map[string]any)
		if input.Username != nil {
			updates["username"] = *input.Username
		}
		if input.FullName != nil {
			updates["full_name"] = *input.FullName
		}
		if input.Position != nil {
			updates["position"] = *input.Position
		}
		if input.IsExec != nil {
			updates["is_exec"] = *input.IsExec
		}
		if input.IsAdmin != nil {
			updates["is_admin"] = *input.IsAdmin
		}
		if input.IsActive != nil {
			if *input.IsActive {
				updates["deactivated_at"] = nil
			} else {
				updates["deactivated_at"] = s.now()
			}
		}

		if len(updates) > 0 {
			if err := tx.Model(&model.User{}).Where("uuid = ?", userUUID).Updates(updates).Error; err != nil {
				if errors.Is(err, gorm.ErrDuplicatedKey) {
					return ErrUsernameExists
				}
				return fmt.Errorf("update user: %w", err)
			}
		}

		if input.DivisionUUIDs != nil {
			divisionUUIDs, err := uniqueDivisionUUIDs(*input.DivisionUUIDs)
			if err != nil {
				return err
			}
			if err := ensureDivisionsExist(tx, divisionUUIDs); err != nil {
				return err
			}
			if err := replaceUserDivisions(tx, userUUID, divisionUUIDs); err != nil {
				return err
			}
		}

		if input.IsActive != nil && !*input.IsActive {
			if err := revokeUserSessions(tx, userUUID); err != nil {
				return err
			}
			if err := removeOpenUnsubmittedAssignments(tx, userUUID); err != nil {
				return err
			}
		}

		loaded, err := findUser(tx, userUUID, false)
		if err != nil {
			return err
		}
		updated = loaded
		return nil
	})
	if err != nil {
		return model.User{}, err
	}
	return updated, nil
}

// ResetPassword เปลี่ยน bcrypt hash และลบ session ทุกตัวของผู้ใช้ใน transaction เดียว.
func (s *UserService) ResetPassword(ctx context.Context, userUUID uuid.UUID, password string) error {
	if err := validatePassword(password); err != nil {
		return &ValidationError{Field: "password", Message: err.Error()}
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash user password: %w", err)
	}

	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := findUser(tx, userUUID, true); err != nil {
			return err
		}
		if err := tx.Model(&model.User{}).
			Where("uuid = ?", userUUID).
			Update("password_hash", string(passwordHash)).Error; err != nil {
			return fmt.Errorf("update user password: %w", err)
		}
		return revokeUserSessions(tx, userUUID)
	})
}

func normalizeCreateUserInput(input CreateUserInput) CreateUserInput {
	input.Username = strings.ToLower(strings.TrimSpace(input.Username))
	input.FullName = strings.TrimSpace(input.FullName)
	input.Position = strings.TrimSpace(input.Position)
	return input
}

func normalizeUpdateUserInput(input UpdateUserInput) UpdateUserInput {
	if input.Username != nil {
		value := strings.ToLower(strings.TrimSpace(*input.Username))
		input.Username = &value
	}
	if input.FullName != nil {
		value := strings.TrimSpace(*input.FullName)
		input.FullName = &value
	}
	if input.Position != nil {
		value := strings.TrimSpace(*input.Position)
		input.Position = &value
	}
	return input
}

func validateCreateUserInput(input CreateUserInput) error {
	if err := validateUsername(input.Username); err != nil {
		return &ValidationError{Field: "username", Message: err.Error()}
	}
	if err := validatePassword(input.Password); err != nil {
		return &ValidationError{Field: "password", Message: err.Error()}
	}
	if err := validateRequiredText("full_name", input.FullName, 120); err != nil {
		return err
	}
	return validateRequiredText("position", input.Position, 120)
}

func validateUpdateUserInput(input UpdateUserInput) error {
	if input.Username == nil && input.FullName == nil && input.Position == nil &&
		input.DivisionUUIDs == nil && input.IsExec == nil && input.IsAdmin == nil && input.IsActive == nil {
		return &ValidationError{Field: "request", Message: "at least one field is required"}
	}
	if input.Username != nil {
		if err := validateUsername(*input.Username); err != nil {
			return &ValidationError{Field: "username", Message: err.Error()}
		}
	}
	if input.FullName != nil {
		if err := validateRequiredText("full_name", *input.FullName, 120); err != nil {
			return err
		}
	}
	if input.Position != nil {
		if err := validateRequiredText("position", *input.Position, 120); err != nil {
			return err
		}
	}
	return nil
}

func validateRequiredText(field, value string, maximum int) error {
	if value == "" {
		return &ValidationError{Field: field, Message: strings.ReplaceAll(field, "_", " ") + " is required"}
	}
	if utf8.RuneCountInString(value) > maximum {
		return &ValidationError{Field: field, Message: fmt.Sprintf("%s must not exceed %d characters", strings.ReplaceAll(field, "_", " "), maximum)}
	}
	return nil
}

func uniqueDivisionUUIDs(values []uuid.UUID) ([]uuid.UUID, error) {
	result := make([]uuid.UUID, 0, len(values))
	seen := make(map[uuid.UUID]struct{}, len(values))
	for _, value := range values {
		if value == uuid.Nil {
			return nil, &ValidationError{Field: "division_uuids", Message: "division UUID must not be empty"}
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result, nil
}

func ensureDivisionsExist(db *gorm.DB, divisionUUIDs []uuid.UUID) error {
	if len(divisionUUIDs) == 0 {
		return nil
	}

	var count int64
	if err := db.Model(&model.Division{}).Where("uuid IN ?", divisionUUIDs).Count(&count).Error; err != nil {
		return fmt.Errorf("validate user divisions: %w", err)
	}
	if count != int64(len(divisionUUIDs)) {
		return ErrDivisionNotFound
	}
	return nil
}

func replaceUserDivisions(db *gorm.DB, userUUID uuid.UUID, divisionUUIDs []uuid.UUID) error {
	if err := db.Where("user_uuid = ?", userUUID).Delete(&model.UserDivision{}).Error; err != nil {
		return fmt.Errorf("clear user divisions: %w", err)
	}
	if len(divisionUUIDs) == 0 {
		return nil
	}

	memberships := make([]model.UserDivision, 0, len(divisionUUIDs))
	for _, divisionUUID := range divisionUUIDs {
		memberships = append(memberships, model.UserDivision{
			UserUUID:     userUUID,
			DivisionUUID: divisionUUID,
		})
	}
	if err := db.Create(&memberships).Error; err != nil {
		return fmt.Errorf("save user divisions: %w", err)
	}
	return nil
}

func findUser(db *gorm.DB, userUUID uuid.UUID, lock bool) (model.User, error) {
	query := db
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var user model.User
	if err := query.Where("uuid = ?", userUUID).Take(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.User{}, ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("find user: %w", err)
	}

	if err := db.Table("divisions").
		Select("divisions.*").
		Joins("JOIN user_divisions ON user_divisions.division_uuid = divisions.uuid").
		Where("user_divisions.user_uuid = ?", userUUID).
		Order("divisions.sort_order").
		Find(&user.Divisions).Error; err != nil {
		return model.User{}, fmt.Errorf("find user divisions: %w", err)
	}
	return user, nil
}

func revokeUserSessions(db *gorm.DB, userUUID uuid.UUID) error {
	if err := db.Where("user_uuid = ?", userUUID).Delete(&model.Session{}).Error; err != nil {
		return fmt.Errorf("revoke user sessions: %w", err)
	}
	return nil
}

func removeOpenUnsubmittedAssignments(db *gorm.DB, userUUID uuid.UUID) error {
	// ใช้ correlated NOT EXISTS เพื่อเก็บ assignment ที่มีคำตอบแล้วไว้
	// จึงไม่ทำให้ประวัติและตัวเลขย้อนหลังของ submission เดิมหายไป.
	result := db.Exec(`
		DELETE FROM topic_assignees AS ta
		USING topics AS t
		WHERE ta.topic_uuid = t.uuid
		  AND ta.user_uuid = ?
		  AND t.is_open = TRUE
		  AND NOT EXISTS (
			SELECT 1
			FROM submissions AS s
			WHERE s.topic_uuid = ta.topic_uuid
			  AND s.user_uuid = ta.user_uuid
		  )
	`, userUUID)
	if result.Error != nil {
		return fmt.Errorf("remove open topic assignments: %w", result.Error)
	}
	return nil
}
