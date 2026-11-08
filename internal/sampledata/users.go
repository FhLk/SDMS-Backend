// Package sampledata loads development-only users from sample-data/users.json.
package sampledata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const developmentOnly = "development-only"

// Dataset mirrors the checked-in development sample file. Division codes are
// resolved against the current database so UUIDs never need to be hard-coded.
type Dataset struct {
	Meta  Metadata     `json:"meta"`
	Users []SampleUser `json:"users"`
}

type Metadata struct {
	Environment     string            `json:"environment"`
	DefaultPassword string            `json:"default_password"`
	DivisionHeads   map[string]string `json:"division_heads"`
	UsageNote       string            `json:"usage_note"`
}

type SampleUser struct {
	Username  string `json:"username"`
	Password  string `json:"password"`
	Prefix    string `json:"prefix"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	IsExec    bool   `json:"is_exec"`
	IsAdmin   bool   `json:"is_admin"`
}

type Result struct {
	CreatedUsers      int
	UpdatedUsers      int
	DivisionHeads     int
	RepairedTopics    int
	RepairedAssignees int
}

// Read decodes and validates a development-only sample data file.
func Read(path string) (Dataset, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return Dataset{}, fmt.Errorf("read sample users: %w", err)
	}

	var dataset Dataset
	decoder := json.NewDecoder(strings.NewReader(string(content)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&dataset); err != nil {
		return Dataset{}, fmt.Errorf("decode sample users: %w", err)
	}
	dataset = normalize(dataset)
	if err := validate(dataset); err != nil {
		return Dataset{}, err
	}
	return dataset, nil
}

// Sync creates missing sample users, updates existing sample profiles, and
// assigns the configured division heads in one transaction.
// Existing passwords and activation state are preserved; passwords from the
// file are used only when a sample account is first created.
func Sync(ctx context.Context, db *gorm.DB, dataset Dataset, repairEmptyTopics bool) (Result, error) {
	dataset = normalize(dataset)
	if err := validate(dataset); err != nil {
		return Result{}, err
	}

	var result Result
	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		divisionUUIDs, err := loadDivisionUUIDs(tx)
		if err != nil {
			return err
		}

		userService := admin.NewUserService(tx)
		existingUsers, err := userService.ListUsers(ctx)
		if err != nil {
			return err
		}
		existingByUsername := make(map[string]model.User, len(existingUsers))
		for _, user := range existingUsers {
			existingByUsername[user.Username] = user
		}

		syncedByUsername := make(map[string]model.User, len(dataset.Users))
		for _, sample := range dataset.Users {
			user, exists := existingByUsername[sample.Username]
			if exists {
				prefix := sample.Prefix
				firstName := sample.FirstName
				lastName := sample.LastName
				isExec := sample.IsExec
				isAdmin := sample.IsAdmin
				user, err = userService.UpdateUser(ctx, user.UUID, admin.UpdateUserInput{
					Prefix:    &prefix,
					FirstName: &firstName,
					LastName:  &lastName,
					IsExec:    &isExec,
					IsAdmin:   &isAdmin,
				})
				if err != nil {
					return fmt.Errorf("update sample user %q: %w", sample.Username, err)
				}
				result.UpdatedUsers++
			} else {
				user, err = userService.CreateUser(ctx, admin.CreateUserInput{
					Username:  sample.Username,
					Password:  sample.Password,
					Prefix:    sample.Prefix,
					FirstName: sample.FirstName,
					LastName:  sample.LastName,
					IsExec:    sample.IsExec,
					IsAdmin:   sample.IsAdmin,
				})
				if err != nil {
					return fmt.Errorf("create sample user %q: %w", sample.Username, err)
				}
				result.CreatedUsers++
			}

			syncedByUsername[sample.Username] = user
		}

		headCodes := make([]string, 0, len(dataset.Meta.DivisionHeads))
		for code := range dataset.Meta.DivisionHeads {
			if _, exists := divisionUUIDs[code]; !exists {
				return fmt.Errorf("division code %q does not exist", code)
			}
			headCodes = append(headCodes, code)
		}
		sort.Strings(headCodes)
		if len(headCodes) > 0 {
			if err := tx.Model(&model.Division{}).
				Where("code IN ?", headCodes).
				Update("head_user_uuid", nil).Error; err != nil {
				return fmt.Errorf("clear sample division heads: %w", err)
			}
		}

		organizationService := admin.NewOrganizationService(tx)
		for _, code := range headCodes {
			username := dataset.Meta.DivisionHeads[code]
			user, exists := syncedByUsername[username]
			if !exists {
				return fmt.Errorf("division head %q for %q is not present in sample users", username, code)
			}
			if _, err := organizationService.SetDivisionHead(ctx, code, user.UUID); err != nil {
				return fmt.Errorf("set %q division head to %q: %w", code, username, err)
			}
			result.DivisionHeads++
		}

		if repairEmptyTopics {
			repairedTopics, repairedAssignees, err := repairEmptyOpenTopics(tx)
			if err != nil {
				return err
			}
			result.RepairedTopics = repairedTopics
			result.RepairedAssignees = repairedAssignees
		}
		return nil
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func normalize(dataset Dataset) Dataset {
	dataset.Users = append([]SampleUser(nil), dataset.Users...)
	for index := range dataset.Users {
		user := &dataset.Users[index]
		user.Username = strings.ToLower(strings.TrimSpace(user.Username))
	}

	heads := make(map[string]string, len(dataset.Meta.DivisionHeads))
	for code, username := range dataset.Meta.DivisionHeads {
		heads[strings.ToLower(strings.TrimSpace(code))] = strings.ToLower(strings.TrimSpace(username))
	}
	dataset.Meta.DivisionHeads = heads
	return dataset
}

func validate(dataset Dataset) error {
	if dataset.Meta.Environment != developmentOnly {
		return fmt.Errorf("sample data environment must be %q", developmentOnly)
	}
	if len(dataset.Users) == 0 {
		return errors.New("sample users must not be empty")
	}

	usernames := make(map[string]struct{}, len(dataset.Users))
	for index, user := range dataset.Users {
		if user.Username == "" {
			return fmt.Errorf("sample users[%d] username is required", index)
		}
		if _, duplicate := usernames[user.Username]; duplicate {
			return fmt.Errorf("sample username %q is duplicated", user.Username)
		}
		usernames[user.Username] = struct{}{}
		if strings.TrimSpace(user.Password) == "" {
			return fmt.Errorf("sample user %q password is required", user.Username)
		}
	}

	for code, username := range dataset.Meta.DivisionHeads {
		if code == "" || username == "" {
			return errors.New("division head code and username are required")
		}
		if _, exists := usernames[username]; !exists {
			return fmt.Errorf("division head %q for %q is not present in sample users", username, code)
		}
	}
	return nil
}

func loadDivisionUUIDs(db *gorm.DB) (map[string]uuid.UUID, error) {
	var divisions []model.Division
	if err := db.Order("sort_order, code").Find(&divisions).Error; err != nil {
		return nil, fmt.Errorf("list divisions for sample data: %w", err)
	}
	result := make(map[string]uuid.UUID, len(divisions))
	for _, division := range divisions {
		result[division.Code] = division.UUID
	}
	return result, nil
}

type missingAssignment struct {
	TopicUUID uuid.UUID
	UserUUID  uuid.UUID
}

// repairEmptyOpenTopics is intentionally narrow: it repairs only open topics
// that have never had an assignee or submission. Normal topics retain their
// original assignment snapshot when account state changes later.
func repairEmptyOpenTopics(db *gorm.DB) (int, int, error) {
	var missing []missingAssignment
	err := db.Table("topics AS t").
		Select("t.uuid AS topic_uuid, u.uuid AS user_uuid").
		Joins("CROSS JOIN accounts AS u").
		Where("t.is_open = ?", true).
		Where("u.deactivated_at IS NULL").
		Where("NOT EXISTS (SELECT 1 FROM topic_assignees AS existing WHERE existing.topic_uuid = t.uuid)").
		Where("NOT EXISTS (SELECT 1 FROM submissions AS submitted WHERE submitted.topic_uuid = t.uuid)").
		Order("t.uuid, u.uuid").
		Scan(&missing).Error
	if err != nil {
		return 0, 0, fmt.Errorf("list empty topic assignments: %w", err)
	}
	if len(missing) == 0 {
		return 0, 0, nil
	}

	assignments := make([]model.TopicAssignee, 0, len(missing))
	topics := make(map[uuid.UUID]struct{})
	for _, pair := range missing {
		assignments = append(assignments, model.TopicAssignee{
			TopicUUID: pair.TopicUUID,
			UserUUID:  pair.UserUUID,
		})
		topics[pair.TopicUUID] = struct{}{}
	}
	result := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&assignments)
	if result.Error != nil {
		return 0, 0, fmt.Errorf("repair empty topic assignments: %w", result.Error)
	}
	return len(topics), int(result.RowsAffected), nil
}
