package topic

import (
	"errors"
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

func TestValidateCreateInputAcceptsAllFieldTypes(t *testing.T) {
	input := normalizeCreateInput(CreateInput{
		DivisionUUID:  uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb10001"),
		CreatedByUUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb10002"),
		Title:         " แบบรายงานประจำเดือน ",
		DueDate:       " 2026-10-31 ",
		Fields: []FieldInput{
			{Type: model.FieldText, Label: " ชื่อกิจกรรม ", Required: true},
			{Type: model.FieldPara, Label: "รายละเอียด"},
			{Type: model.FieldNum, Label: "จำนวน"},
			{Type: model.FieldDate, Label: "วันที่"},
			{Type: model.FieldChoice, Label: "ระดับชั้น", Options: []string{" ม.1 ", "ม.2"}},
			{Type: model.FieldFile, Label: "หลักฐาน"},
		},
	})

	if _, err := validateCreateInput(input); err != nil {
		t.Fatalf("validateCreateInput() error = %v", err)
	}
	if input.Title != "แบบรายงานประจำเดือน" || input.DueDate != "2026-10-31" {
		t.Fatalf("normalized input = %+v", input)
	}
	for index, field := range input.Fields {
		if field.Position != int16(index+1) {
			t.Fatalf("field %d position = %d, want %d", index, field.Position, index+1)
		}
		if field.Options == nil {
			t.Fatalf("field %d options = nil, want JSON-array-compatible empty slice", index)
		}
	}
	if input.Fields[4].Options[0] != "ม.1" {
		t.Fatalf("choice options = %v, want trimmed values", input.Fields[4].Options)
	}
}

func TestValidateCreateInputReportsExactFieldPath(t *testing.T) {
	valid := CreateInput{
		DivisionUUID:  uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb20001"),
		CreatedByUUID: uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6fb20002"),
		Title:         "หัวข้อทดสอบ",
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Position: 1, Type: model.FieldText, Label: "ข้อความ"}},
	}

	tests := []struct {
		name      string
		mutate    func(*CreateInput)
		wantField string
	}{
		{name: "missing title", mutate: func(input *CreateInput) { input.Title = "" }, wantField: "title"},
		{name: "invalid date", mutate: func(input *CreateInput) { input.DueDate = "2026-02-30" }, wantField: "due_date"},
		{name: "missing fields", mutate: func(input *CreateInput) { input.Fields = nil }, wantField: "fields"},
		{name: "invalid type", mutate: func(input *CreateInput) { input.Fields[0].Type = "email" }, wantField: "fields[0].type"},
		{name: "missing label", mutate: func(input *CreateInput) { input.Fields[0].Label = "" }, wantField: "fields[0].label"},
		{
			name: "choice has one option",
			mutate: func(input *CreateInput) {
				input.Fields[0].Type = model.FieldChoice
				input.Fields[0].Options = []string{"เท่านั้น"}
			},
			wantField: "fields[0].options",
		},
		{
			name: "non choice has options",
			mutate: func(input *CreateInput) {
				input.Fields[0].Options = []string{"ไม่ควรมี"}
			},
			wantField: "fields[0].options",
		},
		{
			name: "duplicate position",
			mutate: func(input *CreateInput) {
				input.Fields = append(input.Fields, FieldInput{Position: 1, Type: model.FieldDate, Label: "วันที่"})
			},
			wantField: "fields[1].position",
		},
		{
			name: "duplicate choice option",
			mutate: func(input *CreateInput) {
				input.Fields[0].Type = model.FieldChoice
				input.Fields[0].Options = []string{"ม.1", "ม.1"}
			},
			wantField: "fields[0].options[1]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			input.Fields = append([]FieldInput(nil), valid.Fields...)
			tt.mutate(&input)
			input = normalizeCreateInput(input)
			_, err := validateCreateInput(input)
			var validationErr *admin.ValidationError
			if !errors.As(err, &validationErr) || validationErr.Field != tt.wantField {
				t.Fatalf("error = %v, want ValidationError field %q", err, tt.wantField)
			}
		})
	}
}
