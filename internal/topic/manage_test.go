package topic

import (
	"errors"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

func TestValidateUpdateInputAcceptsPartialMetadataAndFullForm(t *testing.T) {
	title := " หัวข้อฉบับแก้ไข "
	dueDate := " 2026-11-30 "
	fieldUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb50001")
	fields := []UpdateFieldInput{
		{UUID: fieldUUID, Type: " TEXT ", Label: " ชื่อกิจกรรม "},
		{Type: model.FieldChoice, Label: "ระดับชั้น", Options: []string{" ม.1 ", "ม.2"}},
	}
	input := normalizeUpdateInput(UpdateInput{Title: &title, DueDate: &dueDate, Fields: &fields})

	parsedDueDate, err := validateUpdateInput(input)
	if err != nil {
		t.Fatalf("validateUpdateInput() error = %v", err)
	}
	if *input.Title != "หัวข้อฉบับแก้ไข" || parsedDueDate == nil || parsedDueDate.Format(dateLayout) != "2026-11-30" {
		t.Fatalf("normalized input = %+v, due date = %v", input, parsedDueDate)
	}
	if (*input.Fields)[0].Position != 1 || (*input.Fields)[0].Type != model.FieldText || (*input.Fields)[0].Label != "ชื่อกิจกรรม" {
		t.Fatalf("first field = %+v", (*input.Fields)[0])
	}
	if (*input.Fields)[1].Position != 2 || (*input.Fields)[1].Options[0] != "ม.1" {
		t.Fatalf("second field = %+v", (*input.Fields)[1])
	}
}

func TestValidateUpdateInputReportsExactFieldPath(t *testing.T) {
	title := "หัวข้อฉบับแก้ไข"
	validFieldUUID := uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb50002")
	tests := []struct {
		name      string
		input     UpdateInput
		wantField string
	}{
		{name: "empty body", input: UpdateInput{}, wantField: "body"},
		{name: "empty title", input: UpdateInput{Title: stringPointer(" ")}, wantField: "title"},
		{name: "invalid date", input: UpdateInput{DueDate: stringPointer("2026-02-30")}, wantField: "due_date"},
		{name: "empty fields", input: UpdateInput{Fields: updateFieldsPointer([]UpdateFieldInput{})}, wantField: "fields"},
		{
			name: "duplicate field UUID",
			input: UpdateInput{Fields: updateFieldsPointer([]UpdateFieldInput{
				{UUID: validFieldUUID, Position: 1, Type: model.FieldText, Label: "หนึ่ง"},
				{UUID: validFieldUUID, Position: 2, Type: model.FieldText, Label: "สอง"},
			})},
			wantField: "fields[1].uuid",
		},
		{
			name: "choice has one option",
			input: UpdateInput{Fields: updateFieldsPointer([]UpdateFieldInput{
				{Position: 1, Type: model.FieldChoice, Label: "ตัวเลือก", Options: []string{"ข้อเดียว"}},
			})},
			wantField: "fields[0].options",
		},
		{name: "valid title", input: UpdateInput{Title: &title}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := normalizeUpdateInput(tt.input)
			_, err := validateUpdateInput(input)
			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("validateUpdateInput() error = %v", err)
				}
				return
			}
			var validationErr *admin.ValidationError
			if !errors.As(err, &validationErr) || validationErr.Field != tt.wantField {
				t.Fatalf("error = %v, want ValidationError field %q", err, tt.wantField)
			}
		})
	}
}

func stringPointer(value string) *string { return &value }

func updateFieldsPointer(value []UpdateFieldInput) *[]UpdateFieldInput { return &value }
