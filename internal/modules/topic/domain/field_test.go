package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestFieldTypeIsValid(t *testing.T) {
	tests := []struct {
		name  string
		value FieldType
		want  bool
	}{
		{"text", FieldTypeText, true},
		{"textarea", FieldTypeTextarea, true},
		{"number", FieldTypeNumber, true},
		{"date", FieldTypeDate, true},
		{"select", FieldTypeSelect, true},
		{"file", FieldTypeFile, true},
		{"empty", "", false},
		{"unknown", "checkbox", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.value.IsValid(); got != tt.want {
				t.Fatalf("IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewTopicFieldSuccess(t *testing.T) {
	topicID := uuid.New()

	field, err := NewTopicField(
		topicID,
		"เลขที่เอกสาร",
		FieldTypeText,
		true,
		2,
	)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	if field.UID == uuid.Nil {
		t.Error("UID was not generated")
	}

	if field.TopicUID != topicID {
		t.Errorf("TopicUID = %v, want %v", field.TopicUID, topicID)
	}

	if field.Label != "เลขที่เอกสาร" {
		t.Errorf("Label = %q, want %q", field.Label, "เลขที่เอกสาร")
	}

	if field.Type != FieldTypeText {
		t.Errorf("Type = %q, want %q", field.Type, FieldTypeText)
	}

	if !field.Required {
		t.Error("Required = false, want true")
	}

	if field.Position != 2 {
		t.Errorf("Position = %d, want 2", field.Position)
	}

	if len(field.Options) != 0 {
		t.Errorf("Options = %+v, want empty", field.Options)
	}
}

func TestNewTopicFieldValidation(t *testing.T) {
	topicID := uuid.New()

	tests := []struct {
		name      string
		topicID   uuid.UUID
		label     string
		fieldType FieldType
		position  int
		wantErr   error
	}{
		{
			name:      "missing topic",
			topicID:   uuid.Nil,
			label:     "label",
			fieldType: FieldTypeText,
			position:  0,
			wantErr:   ErrTopicFieldInvalidTopicUID,
		},
		{
			name:      "missing label",
			topicID:   topicID,
			label:     "",
			fieldType: FieldTypeText,
			position:  0,
			wantErr:   ErrTopicFieldLabelRequired,
		},
		{
			name:      "invalid type",
			topicID:   topicID,
			label:     "label",
			fieldType: "checkbox",
			position:  0,
			wantErr:   ErrTopicFieldInvalidType,
		},
		{
			name:      "negative position",
			topicID:   topicID,
			label:     "label",
			fieldType: FieldTypeText,
			position:  -1,
			wantErr:   ErrTopicFieldInvalidPosition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewTopicField(
				tt.topicID,
				tt.label,
				tt.fieldType,
				false,
				tt.position,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}

			if field != nil {
				t.Fatalf("field = %+v, want nil", field)
			}
		})
	}
}

func TestNewTopicFieldWithOptionsSuccess(t *testing.T) {
	topicID := uuid.New()

	options := []SelectOption{
		{
			Label: " วิชาการ ",
			Value: " academic ",
		},
		{
			Label: "บุคคล",
			Value: "hr",
		},
	}

	field, err := NewTopicFieldWithOptions(
		topicID,
		"ฝ่ายงาน",
		FieldTypeSelect,
		true,
		1,
		options,
	)
	if err != nil {
		t.Fatalf("NewTopicFieldWithOptions() error = %v", err)
	}

	if field.Type != FieldTypeSelect {
		t.Errorf("Type = %q, want %q", field.Type, FieldTypeSelect)
	}

	if len(field.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(field.Options))
	}

	if field.Options[0].Label != "วิชาการ" {
		t.Errorf(
			"Options[0].Label = %q, want %q",
			field.Options[0].Label,
			"วิชาการ",
		)
	}

	if field.Options[0].Value != "academic" {
		t.Errorf(
			"Options[0].Value = %q, want %q",
			field.Options[0].Value,
			"academic",
		)
	}
}

func TestNewTopicFieldWithOptionsValidation(t *testing.T) {
	topicID := uuid.New()

	tests := []struct {
		name      string
		fieldType FieldType
		options   []SelectOption
		wantErr   error
	}{
		{
			name:      "select requires options",
			fieldType: FieldTypeSelect,
			options:   nil,
			wantErr:   ErrTopicFieldSelectOptionsRequired,
		},
		{
			name:      "select option label required",
			fieldType: FieldTypeSelect,
			options: []SelectOption{
				{
					Label: "",
					Value: "academic",
				},
			},
			wantErr: ErrTopicFieldSelectOptionLabelRequired,
		},
		{
			name:      "select option value required",
			fieldType: FieldTypeSelect,
			options: []SelectOption{
				{
					Label: "วิชาการ",
					Value: "",
				},
			},
			wantErr: ErrTopicFieldSelectOptionValueRequired,
		},
		{
			name:      "duplicate select option value",
			fieldType: FieldTypeSelect,
			options: []SelectOption{
				{
					Label: "วิชาการ",
					Value: "academic",
				},
				{
					Label: "ฝ่ายวิชาการ",
					Value: "academic",
				},
			},
			wantErr: ErrTopicFieldSelectOptionDuplicateValue,
		},
		{
			name:      "options only allowed for select",
			fieldType: FieldTypeText,
			options: []SelectOption{
				{
					Label: "ตัวเลือก",
					Value: "option",
				},
			},
			wantErr: ErrTopicFieldOptionsOnlyForSelect,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewTopicFieldWithOptions(
				topicID,
				"field",
				tt.fieldType,
				false,
				0,
				tt.options,
			)

			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}

			if field != nil {
				t.Fatalf("field = %+v, want nil", field)
			}
		})
	}
}

func TestTopicFieldUpdate(t *testing.T) {
	field, err := NewTopicField(
		uuid.New(),
		"old",
		FieldTypeText,
		false,
		0,
	)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	err = field.Update(
		"new",
		FieldTypeTextarea,
		true,
		3,
		nil,
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if field.Label != "new" {
		t.Errorf("Label = %q, want %q", field.Label, "new")
	}

	if field.Type != FieldTypeTextarea {
		t.Errorf("Type = %q, want %q", field.Type, FieldTypeTextarea)
	}

	if !field.Required {
		t.Error("Required = false, want true")
	}

	if field.Position != 3 {
		t.Errorf("Position = %d, want 3", field.Position)
	}

	err = field.Update(
		"",
		FieldTypeText,
		false,
		0,
		nil,
	)

	if !errors.Is(err, ErrTopicFieldLabelRequired) {
		t.Fatalf(
			"Update() error = %v, want %v",
			err,
			ErrTopicFieldLabelRequired,
		)
	}
}

func TestTopicFieldUpdateSelectOptions(t *testing.T) {
	field, err := NewTopicField(
		uuid.New(),
		"ประเภท",
		FieldTypeText,
		false,
		0,
	)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	options := []SelectOption{
		{
			Label: "วิชาการ",
			Value: "academic",
		},
		{
			Label: "บุคคล",
			Value: "hr",
		},
	}

	err = field.Update(
		"ฝ่ายงาน",
		FieldTypeSelect,
		true,
		1,
		options,
	)
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	if field.Type != FieldTypeSelect {
		t.Errorf("Type = %q, want %q", field.Type, FieldTypeSelect)
	}

	if len(field.Options) != 2 {
		t.Fatalf("len(Options) = %d, want 2", len(field.Options))
	}

	if field.Options[0].Value != "academic" {
		t.Errorf(
			"Options[0].Value = %q, want academic",
			field.Options[0].Value,
		)
	}

	if field.Options[1].Value != "hr" {
		t.Errorf(
			"Options[1].Value = %q, want hr",
			field.Options[1].Value,
		)
	}
}

func TestTopicFieldUpdateSelectValidation(t *testing.T) {
	field, err := NewTopicField(
		uuid.New(),
		"ประเภท",
		FieldTypeText,
		false,
		0,
	)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	err = field.Update(
		"ฝ่ายงาน",
		FieldTypeSelect,
		true,
		1,
		nil,
	)

	if !errors.Is(err, ErrTopicFieldSelectOptionsRequired) {
		t.Fatalf(
			"Update() error = %v, want %v",
			err,
			ErrTopicFieldSelectOptionsRequired,
		)
	}
}

func TestTopicFieldHasSelectOption(t *testing.T) {
	field, err := NewTopicFieldWithOptions(
		uuid.New(),
		"ฝ่ายงาน",
		FieldTypeSelect,
		true,
		1,
		[]SelectOption{
			{
				Label: "วิชาการ",
				Value: "academic",
			},
			{
				Label: "บุคคล",
				Value: "hr",
			},
		},
	)
	if err != nil {
		t.Fatalf("NewTopicFieldWithOptions() error = %v", err)
	}

	if !field.HasSelectOption("academic") {
		t.Error("HasSelectOption(academic) = false, want true")
	}

	if !field.HasSelectOption("hr") {
		t.Error("HasSelectOption(hr) = false, want true")
	}

	if field.HasSelectOption("finance") {
		t.Error("HasSelectOption(finance) = true, want false")
	}
}

func TestTopicFieldHasSelectOptionForNonSelect(t *testing.T) {
	field, err := NewTopicField(
		uuid.New(),
		"ชื่อ",
		FieldTypeText,
		true,
		0,
	)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	if field.HasSelectOption("anything") {
		t.Error("HasSelectOption() = true for text field, want false")
	}
}
