package http

import (
	"testing"
	"time"

	"sdms/internal/modules/submission/domain"

	"github.com/google/uuid"
)

func TestNewSubmissionListResponseIncludesOnlySelectedPreviewValuesInFieldPositionOrder(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()
	nameFieldUID := uuid.New()
	amountFieldUID := uuid.New()
	noteFieldUID := uuid.New()

	name := "โครงการพัฒนาห้องเรียน"
	amount := 25000.0
	note := "ข้อมูลนี้ไม่ควรแสดงใน preview"

	response := newSubmissionListResponse([]domain.Submission{
		{
			UID:         submissionUID,
			TopicUID:    topicUID,
			SubmittedBy: teacherUID,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Values: []domain.SubmissionValue{
				{
					FieldUID:       noteFieldUID,
					FieldLabel:     "หมายเหตุ",
					FieldType:      "text",
					FieldIsPreview: false,
					FieldPosition:  2,
					TextValue:      &note,
				},
				{
					FieldUID:       amountFieldUID,
					FieldLabel:     "งบประมาณ",
					FieldType:      "number",
					FieldIsPreview: true,
					FieldPosition:  1,
					NumberValue:    &amount,
				},
				{
					FieldUID:       nameFieldUID,
					FieldLabel:     "ชื่อโครงการ",
					FieldType:      "text",
					FieldIsPreview: true,
					FieldPosition:  0,
					TextValue:      &name,
				},
			},
		},
	})

	if len(response) != 1 {
		t.Fatalf("expected 1 submission, got %d", len(response))
	}

	preview := response[0].PreviewValues
	if len(preview) != 2 {
		t.Fatalf("expected 2 preview values, got %d", len(preview))
	}

	if preview[0].FieldUID != nameFieldUID || preview[0].Label != "ชื่อโครงการ" || preview[0].Value != name {
		t.Fatalf("unexpected first preview value: %+v", preview[0])
	}

	if preview[1].FieldUID != amountFieldUID || preview[1].Label != "งบประมาณ" || preview[1].Value != amount {
		t.Fatalf("unexpected second preview value: %+v", preview[1])
	}
}
