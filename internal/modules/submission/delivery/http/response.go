package http

import (
	"sdms/internal/modules/submission/domain"
	"sort"
	"time"

	"github.com/google/uuid"
)

type SubmissionResponse struct {
	UID         uuid.UUID                 `json:"uid"`
	TopicUID    uuid.UUID                 `json:"topic_uid"`
	SubmittedBy uuid.UUID                 `json:"submitted_by"`
	Values      []SubmissionValueResponse `json:"values"`
	Files       []SubmissionFileResponse  `json:"files"`
	CreatedAt   time.Time                 `json:"created_at"`
	UpdatedAt   time.Time                 `json:"updated_at"`
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
	UID           uuid.UUID                        `json:"uid"`
	TopicUID      uuid.UUID                        `json:"topic_uid"`
	SubmittedBy   uuid.UUID                        `json:"submitted_by"`
	PreviewValues []SubmissionPreviewValueResponse `json:"preview_values"`
	CreatedAt     time.Time                        `json:"created_at"`
	UpdatedAt     time.Time                        `json:"updated_at"`
}

type SubmissionPreviewValueResponse struct {
	FieldUID uuid.UUID `json:"field_uid"`
	Label    string    `json:"label"`
	Type     string    `json:"type"`
	Position int       `json:"position"`
	Value    any       `json:"value"`
}

func newSubmissionResponse(
	submission domain.Submission,
) SubmissionResponse {
	values := make(
		[]SubmissionValueResponse,
		0,
		len(submission.Values),
	)

	for _, value := range submission.Values {
		values = append(
			values,
			newSubmissionValueResponse(value),
		)
	}

	files := make([]SubmissionFileResponse, 0, len(submission.Files))
	for _, file := range submission.Files {
		files = append(files, newSubmissionFileResponse(file))
	}

	return SubmissionResponse{
		UID:         submission.UID,
		TopicUID:    submission.TopicUID,
		SubmittedBy: submission.SubmittedBy,
		Values:      values,
		Files:       files,
		CreatedAt:   submission.CreatedAt,
		UpdatedAt:   submission.UpdatedAt,
	}
}

func newSubmissionValueResponse(
	value domain.SubmissionValue,
) SubmissionValueResponse {
	var result any

	switch {
	case value.TextValue != nil:
		result = *value.TextValue

	case value.NumberValue != nil:
		result = *value.NumberValue

	case value.DateValue != nil:
		result = value.DateValue.Format("2006-01-02")

	default:
		result = nil
	}

	return SubmissionValueResponse{
		UID:      value.UID,
		FieldUID: value.FieldUID,
		Value:    result,
	}
}

func newSubmissionListResponse(
	submissions []domain.Submission,
) []SubmissionListResponse {
	response := make(
		[]SubmissionListResponse,
		0,
		len(submissions),
	)

	for _, submission := range submissions {
		values := append(
			[]domain.SubmissionValue(nil),
			submission.Values...,
		)
		sort.SliceStable(values, func(i, j int) bool {
			return values[i].FieldPosition < values[j].FieldPosition
		})

		previewValues := make(
			[]SubmissionPreviewValueResponse,
			0,
			3,
		)

		for _, value := range values {
			if !value.FieldIsPreview {
				continue
			}

			previewValues = append(
				previewValues,
				newSubmissionPreviewValueResponse(value),
			)

			if len(previewValues) == 3 {
				break
			}
		}

		response = append(
			response,
			SubmissionListResponse{
				UID:           submission.UID,
				TopicUID:      submission.TopicUID,
				SubmittedBy:   submission.SubmittedBy,
				PreviewValues: previewValues,
				CreatedAt:     submission.CreatedAt,
				UpdatedAt:     submission.UpdatedAt,
			},
		)
	}

	return response
}

func newSubmissionPreviewValueResponse(
	value domain.SubmissionValue,
) SubmissionPreviewValueResponse {
	valueResponse := newSubmissionValueResponse(value)

	return SubmissionPreviewValueResponse{
		FieldUID: value.FieldUID,
		Label:    value.FieldLabel,
		Type:     value.FieldType,
		Position: value.FieldPosition,
		Value:    valueResponse.Value,
	}
}

func newSubmissionFileResponse(file domain.SubmissionFile) SubmissionFileResponse {
	return SubmissionFileResponse{
		UID:              file.UID,
		SubmissionUID:    file.SubmissionUID,
		FieldUID:         file.FieldUID,
		OriginalFilename: file.OriginalFilename,
		ContentType:      file.ContentType,
		Size:             file.Size,
		ViewURL:          "/api/v1/submission-files/" + file.UID.String() + "/view",
		DownloadURL:      "/api/v1/submission-files/" + file.UID.String() + "/download",
		CreatedAt:        file.CreatedAt,
		UpdatedAt:        file.UpdatedAt,
	}
}
