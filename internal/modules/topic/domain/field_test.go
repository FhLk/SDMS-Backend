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
	field, err := NewTopicField(topicID, "เลขที่เอกสาร", FieldTypeText, true, 2)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}
	if field.UID == uuid.Nil {
		t.Error("UID was not generated")
	}
	if field.TopicUID != topicID || field.Label != "เลขที่เอกสาร" || field.Type != FieldTypeText ||
		!field.Required || field.Position != 2 {
		t.Errorf("field = %+v", field)
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
		{"missing topic", uuid.Nil, "label", FieldTypeText, 0, ErrTopicFieldInvalidTopicUID},
		{"missing label", topicID, "", FieldTypeText, 0, ErrTopicFieldLabelRequired},
		{"invalid type", topicID, "label", "checkbox", 0, ErrTopicFieldInvalidType},
		{"negative position", topicID, "label", FieldTypeText, -1, ErrTopicFieldInvalidPosition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, err := NewTopicField(tt.topicID, tt.label, tt.fieldType, false, tt.position)
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
	field, err := NewTopicField(uuid.New(), "old", FieldTypeText, false, 0)
	if err != nil {
		t.Fatalf("NewTopicField() error = %v", err)
	}

	if err := field.Update("new", FieldTypeTextarea, true, 3); err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if field.Label != "new" || field.Type != FieldTypeTextarea || !field.Required || field.Position != 3 {
		t.Errorf("field = %+v", field)
	}

	if err := field.Update("", FieldTypeText, false, 0); !errors.Is(err, ErrTopicFieldLabelRequired) {
		t.Fatalf("Update() error = %v, want %v", err, ErrTopicFieldLabelRequired)
	}
}
