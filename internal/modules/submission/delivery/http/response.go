package http

import (
	"sort"
	"time"

	"sdms/internal/modules/submission/domain"
	"sdms/internal/modules/submission/usecase"

	"github.com/google/uuid"
)

type MissingRequiredFieldResponse struct {
	FieldUID uuid.UUID `json:"field_uid"`
	Label    string    `json:"label"`
	Type     string    `json:"type"`
}

type SubmissionResponse struct {
	UID                   uuid.UUID                      `json:"uid"`
	TopicUID              uuid.UUID                      `json:"topic_uid"`
	SubmittedBy           uuid.UUID                      `json:"submitted_by"`
	FormVersion           int                            `json:"form_version"`
	FormSnapshot          []domain.FormSnapshotField     `json:"form_snapshot"`
	CompletionStatus      string                         `json:"completion_status"`
	MissingRequiredFields []MissingRequiredFieldResponse `json:"missing_required_fields"`
	Values                []SubmissionValueResponse      `json:"values"`
	Files                 []SubmissionFileResponse       `json:"files"`
	CreatedAt             time.Time                      `json:"created_at"`
	UpdatedAt             time.Time                      `json:"updated_at"`
}

type SubmissionValueResponse struct {
	UID      uuid.UUID `json:"uid"`
	FieldUID uuid.UUID `json:"field_uid"`
	Value    any       `json:"value"`
}

type SubmissionFileResponse struct {
	UID              uuid.UUID `json:"uid"`
	SubmissionUID    uuid.UUID `json:"submission_uid"`
	FieldUID         uuid.UUID `json:"field_uid"`
	OriginalFilename string    `json:"original_filename"`
	ContentType      string    `json:"content_type"`
	Size             int64     `json:"size"`
	ViewURL          string    `json:"view_url"`
	DownloadURL      string    `json:"download_url"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type SubmissionListResponse struct {
	UID                   uuid.UUID                        `json:"uid"`
	TopicUID              uuid.UUID                        `json:"topic_uid"`
	SubmittedBy           uuid.UUID                        `json:"submitted_by"`
	FormVersion           int                              `json:"form_version"`
	CompletionStatus      string                           `json:"completion_status"`
	MissingRequiredFields []MissingRequiredFieldResponse   `json:"missing_required_fields"`
	PreviewValues         []SubmissionPreviewValueResponse `json:"preview_values"`
	CreatedAt             time.Time                        `json:"created_at"`
	UpdatedAt             time.Time                        `json:"updated_at"`
}

type SubmissionPreviewValueResponse struct {
	FieldUID uuid.UUID `json:"field_uid"`
	Label    string    `json:"label"`
	Type     string    `json:"type"`
	Position int       `json:"position"`
	Value    any       `json:"value"`
}

type TopicSubmissionStatusResponse struct {
	TeacherUID            uuid.UUID                      `json:"teacher_uid"`
	EmployeeCode          string                         `json:"employee_code"`
	Prefix                string                         `json:"prefix"`
	FirstName             string                         `json:"first_name"`
	LastName              string                         `json:"last_name"`
	Status                string                         `json:"status"`
	LatestSubmissionUID   *uuid.UUID                     `json:"latest_submission_uid,omitempty"`
	LatestSubmissionAt    *time.Time                     `json:"latest_submission_at,omitempty"`
	MissingRequiredFields []MissingRequiredFieldResponse `json:"missing_required_fields"`
}

func newSubmissionResponse(submission domain.Submission) SubmissionResponse {
	values := make([]SubmissionValueResponse, 0, len(submission.Values))
	for _, value := range submission.Values {
		values = append(values, newSubmissionValueResponse(value))
	}
	files := make([]SubmissionFileResponse, 0, len(submission.Files))
	for _, file := range submission.Files {
		files = append(files, newSubmissionFileResponse(file))
	}
	return SubmissionResponse{
		UID: submission.UID, TopicUID: submission.TopicUID, SubmittedBy: submission.SubmittedBy,
		FormVersion: submission.FormVersion, FormSnapshot: submission.FormSnapshot,
		CompletionStatus:      string(submission.CompletionStatus),
		MissingRequiredFields: newMissingRequiredFields(submission.MissingRequiredFields),
		Values:                values, Files: files, CreatedAt: submission.CreatedAt, UpdatedAt: submission.UpdatedAt,
	}
}

func newSubmissionValueResponse(value domain.SubmissionValue) SubmissionValueResponse {
	var result any
	switch {
	case value.TextValue != nil:
		result = *value.TextValue
	case value.NumberValue != nil:
		result = *value.NumberValue
	case value.DateValue != nil:
		result = value.DateValue.Format("2006-01-02")
	}
	return SubmissionValueResponse{UID: value.UID, FieldUID: value.FieldUID, Value: result}
}

func newSubmissionListResponse(submissions []domain.Submission) []SubmissionListResponse {
	response := make([]SubmissionListResponse, 0, len(submissions))
	for _, submission := range submissions {
		values := append([]domain.SubmissionValue(nil), submission.Values...)
		sort.SliceStable(values, func(i, j int) bool { return values[i].FieldPosition < values[j].FieldPosition })
		previewValues := make([]SubmissionPreviewValueResponse, 0, len(values))
		for _, value := range values {
			if value.FieldIsPreview {
				previewValues = append(previewValues, newSubmissionPreviewValueResponse(value))
			}
		}
		response = append(response, SubmissionListResponse{
			UID: submission.UID, TopicUID: submission.TopicUID, SubmittedBy: submission.SubmittedBy,
			FormVersion:           submission.FormVersion,
			CompletionStatus:      string(submission.CompletionStatus),
			MissingRequiredFields: newMissingRequiredFields(submission.MissingRequiredFields), PreviewValues: previewValues,
			CreatedAt: submission.CreatedAt, UpdatedAt: submission.UpdatedAt,
		})
	}
	return response
}

func newSubmissionPreviewValueResponse(value domain.SubmissionValue) SubmissionPreviewValueResponse {
	valueResponse := newSubmissionValueResponse(value)
	return SubmissionPreviewValueResponse{
		FieldUID: value.FieldUID, Label: value.FieldLabel, Type: value.FieldType,
		Position: value.FieldPosition, Value: valueResponse.Value,
	}
}

func newSubmissionFileResponse(file domain.SubmissionFile) SubmissionFileResponse {
	return SubmissionFileResponse{
		UID: file.UID, SubmissionUID: file.SubmissionUID, FieldUID: file.FieldUID,
		OriginalFilename: file.OriginalFilename, ContentType: file.ContentType, Size: file.Size,
		ViewURL:     "/api/v1/submission-files/" + file.UID.String() + "/view",
		DownloadURL: "/api/v1/submission-files/" + file.UID.String() + "/download",
		CreatedAt:   file.CreatedAt, UpdatedAt: file.UpdatedAt,
	}
}

func newMissingRequiredFields(fields []domain.MissingRequiredField) []MissingRequiredFieldResponse {
	result := make([]MissingRequiredFieldResponse, 0, len(fields))
	for _, field := range fields {
		result = append(result, MissingRequiredFieldResponse{FieldUID: field.FieldUID, Label: field.Label, Type: field.Type})
	}
	return result
}

func newTopicSubmissionStatusResponse(items []usecase.TopicSubmissionStatus) []TopicSubmissionStatusResponse {
	result := make([]TopicSubmissionStatusResponse, 0, len(items))
	for _, item := range items {
		result = append(result, TopicSubmissionStatusResponse{
			TeacherUID: item.Teacher.UID, EmployeeCode: item.Teacher.EmployeeCode, Prefix: item.Teacher.Prefix,
			FirstName: item.Teacher.FirstName, LastName: item.Teacher.LastName, Status: item.Status,
			LatestSubmissionUID: item.LatestSubmissionUID, LatestSubmissionAt: item.LatestSubmissionAt,
			MissingRequiredFields: newMissingRequiredFields(item.MissingRequiredFields),
		})
	}
	return result
}
