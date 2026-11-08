package admin

import (
	"errors"
	"fmt"
	"testing"
)

func TestBuildUserImportTemplateCanBeReadByImporter(t *testing.T) {
	tests := []struct {
		format   string
		filename string
	}{
		{format: "csv", filename: "template.csv"},
		{format: "xlsx", filename: "template.xlsx"},
	}

	for _, tt := range tests {
		t.Run(tt.format, func(t *testing.T) {
			content, contentType, filename, err := BuildUserImportTemplate(tt.format)
			if err != nil {
				t.Fatalf("BuildUserImportTemplate() error = %v", err)
			}
			if len(content) == 0 || contentType == "" || filename == "" {
				t.Fatalf("template metadata/content is incomplete")
			}

			rows, err := readImportRows(tt.filename, content)
			if err != nil {
				t.Fatalf("readImportRows(template) error = %v", err)
			}
			if len(rows) != 1 {
				t.Fatalf("rows = %d, want one example row", len(rows))
			}
			if rows[0].Cells[0] != "นาย" || rows[0].Cells[1] != "สมชาย" || rows[0].Cells[2] != "ใจดี" || rows[0].Cells[4] != "somchai.j" || rows[0].Cells[5] != "change-me-123" {
				t.Fatalf("example row = %v", rows[0].Cells)
			}
		})
	}
}

func TestReadImportRowsMapsAliasesAndReorderedColumns(t *testing.T) {
	content := []byte("หมายเหตุ,password,full_name,username\nทดสอบ,\" safe-password \",ครู หนึ่ง,Teacher.One\n")
	rows, err := readImportRows("users.csv", content)
	if err != nil {
		t.Fatalf("readImportRows() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	// รหัสผ่านต้องเก็บช่องว่างตามไฟล์ ส่วน field อื่นจะ normalize ตอน validate.
	want := []string{"", "", "", "ครู หนึ่ง", "Teacher.One", " safe-password "}
	for index := range want {
		if rows[0].Cells[index] != want[index] {
			t.Fatalf("cell %d = %q, want %q", index, rows[0].Cells[index], want[index])
		}
	}
}

func TestReadImportRowsRejectsMissingRequiredColumn(t *testing.T) {
	_, err := readImportRows("users.csv", []byte("ชื่อ-สกุล,username\nครู หนึ่ง,teacher.one\n"))
	var validationErr *ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "header" {
		t.Fatalf("error = %v, want header ValidationError", err)
	}
}

func TestReadImportRowsRejectsUnsupportedFileType(t *testing.T) {
	_, err := readImportRows("users.xls", []byte("legacy spreadsheet"))
	if !errors.Is(err, ErrUnsupportedImportFile) {
		t.Fatalf("error = %v, want ErrUnsupportedImportFile", err)
	}
}

func TestValidateImportRowsCollectsAllErrors(t *testing.T) {
	rows := []rawImportRow{
		{Number: 2, Cells: []string{"", "ครู", "หนึ่ง", "", "Same.User", "short"}},
		{Number: 3, Cells: []string{"", "ครู", "สอง", "", "same.user", "safe-password"}},
		{Number: 4, Cells: []string{"", "", "", "", "bad space", "safe-password"}},
	}

	validated, rowErrors := validateImportRows(rows)
	if len(validated) != 3 {
		t.Fatalf("validated rows = %d, want 3 so all rows can be checked together", len(validated))
	}
	wantErrors := map[string]bool{
		"2/password":   false,
		"2/username":   false,
		"3/username":   false,
		"4/first_name": false,
		"4/username":   false,
	}
	for _, rowError := range rowErrors {
		key := fmt.Sprintf("%d/%s", rowError.Row, rowError.Field)
		if _, exists := wantErrors[key]; exists {
			wantErrors[key] = true
		}
	}
	for key, found := range wantErrors {
		if !found {
			t.Errorf("missing validation error %s; got %+v", key, rowErrors)
		}
	}
}
