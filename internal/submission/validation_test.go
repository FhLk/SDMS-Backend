package submission

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

func TestNormalizeSubmissionValuesAcceptsEveryFieldType(t *testing.T) {
	fields := validationTestFields()
	fileUUID := uuid.New()
	values := map[string]json.RawMessage{
		fields[0].UUID.String(): json.RawMessage(`"  ข้อความ  "`),
		fields[1].UUID.String(): json.RawMessage(`"รายละเอียด"`),
		fields[2].UUID.String(): json.RawMessage(`12.5`),
		fields[3].UUID.String(): json.RawMessage(`"2026-10-13"`),
		fields[4].UUID.String(): json.RawMessage(`"ม.2"`),
		fields[5].UUID.String(): json.RawMessage(`["` + fileUUID.String() + `"]`),
	}

	normalized, references, violations := normalizeSubmissionValues(values, fields)
	if len(violations) != 0 {
		t.Fatalf("violations = %+v, want none", violations)
	}
	if got := string(normalized[fields[0].UUID.String()]); got != `"ข้อความ"` {
		t.Fatalf("normalized text = %s, want trimmed value", got)
	}
	if len(references) != 1 || references[0].UUID != fileUUID || references[0].FieldUUID != fields[5].UUID {
		t.Fatalf("file references = %+v", references)
	}
}

func TestNormalizeSubmissionValuesReturnsAllFieldViolations(t *testing.T) {
	fields := validationTestFields()
	unknown := uuid.NewString()
	values := map[string]json.RawMessage{
		unknown:                 json.RawMessage(`true`),
		fields[0].UUID.String(): json.RawMessage(`"   "`),
		fields[1].UUID.String(): json.RawMessage(`123`),
		fields[2].UUID.String(): json.RawMessage(`"12"`),
		fields[3].UUID.String(): json.RawMessage(`"2026-02-30"`),
		fields[4].UUID.String(): json.RawMessage(`"ม.9"`),
		fields[5].UUID.String(): json.RawMessage(`["not-a-uuid"]`),
	}

	_, _, violations := normalizeSubmissionValues(values, fields)
	if got, want := len(violations), 7; got != want {
		t.Fatalf("violations = %+v, want %d items", violations, want)
	}
	seen := make(map[string]bool, len(violations))
	for _, violation := range violations {
		seen[violation.Field] = true
	}
	if !seen["values."+unknown] || !seen["values."+fields[0].UUID.String()] || !seen["values."+fields[5].UUID.String()] {
		t.Fatalf("violation paths = %+v", seen)
	}
}

func TestNormalizeSubmissionValuesOmitsBlankOptionalField(t *testing.T) {
	field := model.TopicField{
		UUIDModel: model.UUIDModel{UUID: uuid.New()},
		Type:      model.FieldText,
		Required:  false,
	}
	values := map[string]json.RawMessage{field.UUID.String(): json.RawMessage(`"  "`)}
	normalized, _, violations := normalizeSubmissionValues(values, []model.TopicField{field})
	if len(violations) != 0 || len(normalized) != 0 {
		t.Fatalf("normalized/violations = %+v / %+v, want empty", normalized, violations)
	}
}

func TestSubmissionStatusUsesBangkokCalendarDate(t *testing.T) {
	due := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	onTime := time.Date(2026, time.October, 13, 16, 59, 0, 0, time.UTC) // 23:59 เวลาไทย
	late := time.Date(2026, time.October, 13, 17, 0, 0, 0, time.UTC)    // 00:00 วันถัดไปเวลาไทย
	if got := submissionStatus(onTime, due); got != StatusSubmitted {
		t.Fatalf("on-time status = %q", got)
	}
	if got := submissionStatus(late, due); got != StatusLate {
		t.Fatalf("late status = %q", got)
	}
}

func validationTestFields() []model.TopicField {
	choiceOptions, _ := json.Marshal([]string{"ม.1", "ม.2"})
	return []model.TopicField{
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldText, Required: true},
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldPara, Required: true},
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldNum, Required: true},
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldDate, Required: true},
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldChoice, Required: true, Options: datatypes.JSON(choiceOptions)},
		{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Type: model.FieldFile, Required: true},
	}
}
