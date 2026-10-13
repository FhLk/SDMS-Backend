package submission

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestTableRowStatusCoversUnsubmittedStates(t *testing.T) {
	due := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	before := time.Date(2026, time.October, 12, 12, 0, 0, 0, time.UTC)
	after := time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC)
	open := model.Topic{DueDate: due, IsOpen: true}
	closed := model.Topic{DueDate: due, IsOpen: false}

	if got := tableRowStatus(nil, open, before); got != StatusPending {
		t.Fatalf("pending status = %q", got)
	}
	if got := tableRowStatus(nil, open, after); got != StatusOverdue {
		t.Fatalf("overdue status = %q", got)
	}
	if got := tableRowStatus(nil, closed, before); got != StatusNotSubmitted {
		t.Fatalf("closed status = %q", got)
	}
}

func TestTableValuesIncludesEveryActiveFieldAndExcludesUnknownKeys(t *testing.T) {
	first := model.TopicField{UUIDModel: model.UUIDModel{UUID: uuid.New()}}
	second := model.TopicField{UUIDModel: model.UUIDModel{UUID: uuid.New()}}
	unknown := uuid.NewString()
	stored := datatypes.JSON([]byte(`{"` + first.UUID.String() + `":"ตอบแล้ว","` + unknown + `":"ซ่อน"}`))

	encoded, err := tableValues([]model.TopicField{first, second}, stored)
	if err != nil {
		t.Fatalf("tableValues() error = %v", err)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &values); err != nil {
		t.Fatalf("decode values: %v", err)
	}
	if len(values) != 2 || string(values[first.UUID.String()]) != `"ตอบแล้ว"` || string(values[second.UUID.String()]) != "null" {
		t.Fatalf("values = %s", encoded)
	}
	if _, exists := values[unknown]; exists {
		t.Fatalf("hidden key %q leaked in table values", unknown)
	}
}

func TestListTopicTableRejectsInvalidFilterBeforeDatabase(t *testing.T) {
	service := NewService(nil, nil)
	_, err := service.ListTopicTable(t.Context(), uuid.New(), auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: uuid.New()}}}, "late")
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Fields) != 1 || validationErr.Fields[0].Field != "status" {
		t.Fatalf("error = %#v, want status ValidationError", err)
	}
}
