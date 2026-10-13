package submission

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

const (
	maximumTextLength = 500
	maximumParaLength = 5000
	dateLayout        = "2006-01-02"
)

// FieldViolation ระบุช่องและสาเหตุที่ทำให้ส่งคำตอบไม่ได้ เพื่อให้ frontend
// แสดงข้อความใต้ช่องที่ผิดทั้งหมดได้จาก response รอบเดียว.
type FieldViolation struct {
	Field     string     `json:"field"`
	FieldUUID *uuid.UUID `json:"field_uuid,omitempty"`
	Message   string     `json:"message"`
}

// ValidationError รวมข้อผิดพลาดรายช่องแทนการคืนเฉพาะข้อผิดพลาดแรก.
type ValidationError struct {
	Fields []FieldViolation
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("submission contains %d invalid field(s)", len(e.Fields))
}

type fileReference struct {
	UUID      uuid.UUID
	FieldUUID uuid.UUID
	Position  int16
	Path      string
}

func normalizeSubmissionValues(values map[string]json.RawMessage, fields []model.TopicField) (map[string]json.RawMessage, []fileReference, []FieldViolation) {
	normalized := make(map[string]json.RawMessage, len(values))
	fileReferences := make([]fileReference, 0)
	violations := make([]FieldViolation, 0)
	active := make(map[string]model.TopicField, len(fields))
	for _, field := range fields {
		active[field.UUID.String()] = field
	}

	// ตรวจ key ที่ไม่ใช่ช่องปัจจุบันแบบเรียงลำดับ เพื่อให้ผล validation คงที่ทุกครั้ง.
	unknownKeys := make([]string, 0)
	for key := range values {
		if _, exists := active[key]; !exists {
			unknownKeys = append(unknownKeys, key)
		}
	}
	sort.Strings(unknownKeys)
	for _, key := range unknownKeys {
		violations = append(violations, FieldViolation{
			Field:   "values." + key,
			Message: "field does not belong to this topic or is hidden",
		})
	}

	for _, field := range fields {
		key := field.UUID.String()
		path := "values." + key
		raw, exists := values[key]
		if !exists || isJSONNull(raw) {
			if field.Required {
				violations = append(violations, fieldViolation(field.UUID, path, "field is required"))
			}
			continue
		}

		value, files, message := normalizeFieldValue(field, raw, path)
		if message != "" {
			violations = append(violations, fieldViolation(field.UUID, path, message))
			continue
		}
		if value == nil {
			if field.Required {
				violations = append(violations, fieldViolation(field.UUID, path, "field is required"))
			}
			continue
		}
		normalized[key] = value
		fileReferences = append(fileReferences, files...)
	}

	return normalized, fileReferences, violations
}

func normalizeFieldValue(field model.TopicField, raw json.RawMessage, path string) (json.RawMessage, []fileReference, string) {
	switch field.Type {
	case model.FieldText, model.FieldPara:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, nil, "value must be a string"
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil, ""
		}
		limit := maximumTextLength
		if field.Type == model.FieldPara {
			limit = maximumParaLength
		}
		if utf8.RuneCountInString(value) > limit {
			return nil, nil, fmt.Sprintf("value must not exceed %d characters", limit)
		}
		return marshalNormalized(value), nil, ""

	case model.FieldNum:
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.UseNumber()
		var decoded any
		if err := decoder.Decode(&decoded); err != nil {
			return nil, nil, "value must be a number"
		}
		if err := ensureJSONEnd(decoder); err != nil {
			return nil, nil, "value must be a number"
		}
		number, ok := decoded.(json.Number)
		if !ok {
			return nil, nil, "value must be a number"
		}
		parsed, err := strconv.ParseFloat(number.String(), 64)
		if err != nil || math.IsInf(parsed, 0) || math.IsNaN(parsed) {
			return nil, nil, "value must be a finite number"
		}
		return json.RawMessage(number.String()), nil, ""

	case model.FieldDate:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, nil, "value must use YYYY-MM-DD"
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil, ""
		}
		parsed, err := time.Parse(dateLayout, value)
		if err != nil || parsed.Format(dateLayout) != value {
			return nil, nil, "value must use YYYY-MM-DD and be a valid date"
		}
		return marshalNormalized(value), nil, ""

	case model.FieldChoice:
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return nil, nil, "value must be a string"
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil, nil, ""
		}
		var options []string
		if err := json.Unmarshal(field.Options, &options); err != nil {
			return nil, nil, "field options are invalid"
		}
		for _, option := range options {
			if value == option {
				return marshalNormalized(value), nil, ""
			}
		}
		return nil, nil, "value must be one of the field options"

	case model.FieldFile:
		var values []string
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, nil, "value must be an array of file UUIDs"
		}
		if len(values) == 0 {
			return nil, nil, ""
		}
		if len(values) > 32767 {
			return nil, nil, "file array is too large"
		}
		files := make([]fileReference, 0, len(values))
		canonical := make([]string, 0, len(values))
		seen := make(map[uuid.UUID]struct{}, len(values))
		for index, value := range values {
			fileUUID, err := uuid.Parse(strings.TrimSpace(value))
			if err != nil || fileUUID == uuid.Nil {
				return nil, nil, fmt.Sprintf("file at index %d must be a valid UUID", index)
			}
			if _, duplicate := seen[fileUUID]; duplicate {
				return nil, nil, fmt.Sprintf("file at index %d must not be duplicated", index)
			}
			seen[fileUUID] = struct{}{}
			canonical = append(canonical, fileUUID.String())
			files = append(files, fileReference{
				UUID:      fileUUID,
				FieldUUID: field.UUID,
				Position:  int16(index),
				Path:      fmt.Sprintf("%s[%d]", path, index),
			})
		}
		return marshalNormalized(canonical), files, ""

	default:
		return nil, nil, "field type is not supported"
	}
}

func fieldViolation(fieldUUID uuid.UUID, field, message string) FieldViolation {
	value := fieldUUID
	return FieldViolation{Field: field, FieldUUID: &value, Message: message}
}

func isJSONNull(value json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(value), []byte("null"))
}

func marshalNormalized(value any) json.RawMessage {
	encoded, _ := json.Marshal(value)
	return encoded
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	err := decoder.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values")
	}
	return err
}
