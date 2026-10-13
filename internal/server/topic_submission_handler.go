package server

import (
	"encoding/json"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/apperror"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/FhLk/SDMS-Backend/internal/submission"
	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type topicSubmissionUserResponse struct {
	UUID     uuid.UUID `json:"uuid"`
	Username string    `json:"username"`
	FullName string    `json:"full_name"`
	Position string    `json:"position"`
}

type topicSubmissionRowResponse struct {
	User           topicSubmissionUserResponse `json:"user"`
	SubmissionUUID *uuid.UUID                  `json:"submission_uuid"`
	Values         json.RawMessage             `json:"values"`
	Files          []submissionFileResponse    `json:"files"`
	Status         string                      `json:"status"`
	SubmittedAt    *time.Time                  `json:"submitted_at"`
	UpdatedAt      *time.Time                  `json:"updated_at"`
}

type topicSubmissionTableResponse struct {
	Topic topicResponse                `json:"topic"`
	Rows  []topicSubmissionRowResponse `json:"rows"`
}

// listTopicTable คืนตารางคำตอบตาม scope ที่ service คำนวณจาก profile และ policy กลาง.
func (h submissionHandler) listTopicTable(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Submission service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	table, err := h.service.ListTopicTable(c.Context(), topicUUID, profile, c.Query("status"))
	if err != nil {
		return mapSubmissionError(err)
	}
	response, err := newTopicSubmissionTableResponse(table)
	if err != nil {
		return apperror.Wrap(fiber.StatusInternalServerError, "INTERNAL_SERVER_ERROR", "Internal server error", err)
	}
	return c.JSON(response)
}

// listPendingNames ใช้สำหรับคัดลอกรายชื่อคนที่ยังไม่ส่ง และ service จะตรวจ
// สิทธิ์แก้หัวข้อก่อน query รายชื่อทุกครั้ง.
func (h submissionHandler) listPendingNames(c fiber.Ctx) error {
	if h.service == nil {
		return apperror.New(fiber.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "Submission service unavailable")
	}
	profile, ok := authenticatedProfile(c)
	if !ok {
		return apperror.New(fiber.StatusUnauthorized, "UNAUTHORIZED", "Authentication required")
	}
	topicUUID, err := parseTopicUUID(c)
	if err != nil {
		return err
	}
	users, err := h.service.ListPendingNames(c.Context(), topicUUID, profile)
	if err != nil {
		return mapSubmissionError(err)
	}
	response := make([]topicSubmissionUserResponse, 0, len(users))
	for _, user := range users {
		response = append(response, newTopicSubmissionUserResponse(user))
	}
	return c.JSON(response)
}

func newTopicSubmissionTableResponse(table submission.TopicTable) (topicSubmissionTableResponse, error) {
	topicValue, err := newTopicResponse(table.Topic)
	if err != nil {
		return topicSubmissionTableResponse{}, err
	}
	rows := make([]topicSubmissionRowResponse, 0, len(table.Rows))
	for _, row := range table.Rows {
		files := make([]submissionFileResponse, 0, len(row.Files))
		for _, file := range row.Files {
			files = append(files, newSubmissionFileResponse(file))
		}
		rows = append(rows, topicSubmissionRowResponse{
			User:           newTopicSubmissionUserResponse(row.User),
			SubmissionUUID: row.SubmissionUUID,
			Values:         json.RawMessage(row.Values),
			Files:          files,
			Status:         row.Status,
			SubmittedAt:    row.SubmittedAt,
			UpdatedAt:      row.UpdatedAt,
		})
	}
	return topicSubmissionTableResponse{Topic: topicValue, Rows: rows}, nil
}

func newTopicSubmissionUserResponse(user model.User) topicSubmissionUserResponse {
	return topicSubmissionUserResponse{
		UUID:     user.UUID,
		Username: user.Username,
		FullName: user.FullName,
		Position: user.Position,
	}
}

func newSubmissionFileResponse(file model.File) submissionFileResponse {
	return submissionFileResponse{
		UUID:         file.UUID,
		FieldUUID:    file.FieldUUID,
		OriginalName: file.OriginalName,
		MimeType:     file.MimeType,
		SizeBytes:    file.SizeBytes,
		Position:     file.Position,
	}
}
