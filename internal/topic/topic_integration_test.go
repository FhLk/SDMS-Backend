package topic

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/admin"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestTopicServiceWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// rollback ผู้ใช้ หัวข้อ fields และ assignees ทั้งหมดเมื่อ test จบ.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	suffix := uuid.NewString()[:8]
	divisions := []model.Division{
		{Code: "a" + suffix[:7], Name: "ฝ่ายทดสอบหนึ่ง " + suffix, ShortName: "ทดสอบหนึ่ง", SortOrder: 90},
		{Code: "b" + suffix[:7], Name: "ฝ่ายทดสอบสอง " + suffix, ShortName: "ทดสอบสอง", SortOrder: 91},
	}
	if err := tx.Create(&divisions).Error; err != nil {
		t.Fatalf("create isolated divisions: %v", err)
	}
	var currentYear model.AcademicYear
	if err := tx.Where("is_current = ?", true).Take(&currentYear).Error; err != nil {
		t.Fatalf("find current academic year: %v", err)
	}

	inactiveAt := time.Now()
	users := []model.User{
		{Username: "be08." + suffix + ".admin", PasswordHash: "integration-hash", FullName: "ผู้สร้าง", IsAdmin: true},
		{Username: "be08." + suffix + ".teacher", PasswordHash: "integration-hash", FullName: "ครูที่ต้องส่ง"},
		{Username: "be08." + suffix + ".exec", PasswordHash: "integration-hash", FullName: "ผู้บริหาร", IsExec: true},
		{Username: "be08." + suffix + ".inactive", PasswordHash: "integration-hash", FullName: "บัญชีปิด", DeactivatedAt: &inactiveAt},
		{Username: "be08." + suffix + ".other", PasswordHash: "integration-hash", FullName: "ครูต่างฝ่าย"},
	}
	if err := tx.Create(&users).Error; err != nil {
		t.Fatalf("create users: %v", err)
	}
	if err := tx.Model(&divisions[0]).Update("head_user_uuid", users[1].UUID).Error; err != nil {
		t.Fatalf("set division head: %v", err)
	}
	var activeAccountCount int64
	if err := tx.Model(&model.User{}).Where("deactivated_at IS NULL").Count(&activeAccountCount).Error; err != nil {
		t.Fatalf("count active accounts: %v", err)
	}

	service := NewService(tx)
	title := "BE-08 " + suffix
	created, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         " " + title + " ",
		DueDate:       "2026-10-31",
		IsSecret:      true,
		Fields: []FieldInput{
			{Position: 6, Type: model.FieldFile, Label: "หลักฐาน", Required: true},
			{Position: 1, Type: model.FieldText, Label: "ชื่อกิจกรรม", Required: true},
			{Position: 5, Type: model.FieldChoice, Label: "ระดับชั้น", Required: true, Options: []string{"ม.1", "ม.2"}},
			{Position: 3, Type: model.FieldNum, Label: "จำนวน", Required: false},
			{Position: 2, Type: model.FieldPara, Label: "รายละเอียด", Required: false},
			{Position: 4, Type: model.FieldDate, Label: "วันที่", Required: true},
		},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.UUID == uuid.Nil || created.Title != title || !created.IsOpen || !created.IsSecret {
		t.Fatalf("created topic = %+v", created)
	}
	if created.AcademicYearUUID != currentYear.UUID || created.Semester != currentYear.CurrentSemester {
		t.Fatalf("period = year %s semester %d, want %s semester %d", created.AcademicYearUUID, created.Semester, currentYear.UUID, currentYear.CurrentSemester)
	}
	if len(created.Fields) != 6 {
		t.Fatalf("fields = %d, want 6", len(created.Fields))
	}
	for index, field := range created.Fields {
		if field.Position != int16(index+1) {
			t.Fatalf("field %d position = %d, want %d", index, field.Position, index+1)
		}
	}
	if int64(len(created.Assignees)) != activeAccountCount {
		t.Fatalf("assignees = %d, want all %d active accounts", len(created.Assignees), activeAccountCount)
	}

	assertTopicRelatedCount(t, tx, &model.TopicField{}, "topic_uuid = ?", 6, created.UUID)
	assertTopicRelatedCount(t, tx, &model.TopicField{}, "topic_uuid = ? AND required = ?", 2, created.UUID, false)
	assertTopicRelatedCount(t, tx, &model.TopicAssignee{}, "topic_uuid = ?", activeAccountCount, created.UUID)
	var assigneeUUIDs []uuid.UUID
	if err := tx.Model(&model.TopicAssignee{}).
		Where("topic_uuid = ?", created.UUID).
		Order("user_uuid").
		Pluck("user_uuid", &assigneeUUIDs).Error; err != nil {
		t.Fatalf("list assignee UUIDs: %v", err)
	}
	wantAssignees := map[uuid.UUID]bool{
		users[0].UUID: false,
		users[1].UUID: false,
		users[2].UUID: false,
		users[4].UUID: false,
	}
	for _, userUUID := range assigneeUUIDs {
		if _, expected := wantAssignees[userUUID]; expected {
			wantAssignees[userUUID] = true
		}
	}
	for userUUID, found := range wantAssignees {
		if !found {
			t.Errorf("active account %s is missing from assignees: %v", userUUID, assigneeUUIDs)
		}
	}

	// เมื่อระบุรายชื่อเอง ต้องสร้าง assignment เฉพาะคนที่เลือกและปฏิเสธคนที่ไม่เข้าเงื่อนไข.
	selectedAssignees := []uuid.UUID{users[2].UUID}
	selectedTitle := title + " selected assignee"
	selectedTopic, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         selectedTitle,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ข้อความ"}},
		AssigneeUUIDs: &selectedAssignees,
	})
	if err != nil {
		t.Fatalf("Create(selected assignee) error = %v", err)
	}
	if len(selectedTopic.Assignees) != 1 || selectedTopic.Assignees[0].UserUUID != users[2].UUID {
		t.Fatalf("selected assignees = %+v", selectedTopic.Assignees)
	}

	ineligibleAssignees := []uuid.UUID{users[3].UUID}
	ineligibleTitle := title + " ineligible assignee"
	_, err = service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         ineligibleTitle,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ข้อความ"}},
		AssigneeUUIDs: &ineligibleAssignees,
	})
	var assigneeValidationErr *admin.ValidationError
	if !errors.As(err, &assigneeValidationErr) || assigneeValidationErr.Field != "assignee_uuids[0]" {
		t.Fatalf("ineligible assignee error = %v, want assignee_uuids[0] ValidationError", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "title = ?", 0, ineligibleTitle)

	// validation ต้องเกิดก่อนเริ่มเขียน จึงไม่มีหัวข้อค้างเมื่อ choice มีตัวเลือกไม่ครบ.
	invalidTitle := title + " invalid"
	_, err = service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         invalidTitle,
		DueDate:       "2026-10-31",
		Fields: []FieldInput{{
			Type: model.FieldChoice, Label: "ตัวเลือกไม่ครบ", Options: []string{"ข้อเดียว"},
		}},
	})
	var validationErr *admin.ValidationError
	if !errors.As(err, &validationErr) || validationErr.Field != "fields[0].options" {
		t.Fatalf("invalid choice error = %v, want fields[0].options ValidationError", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "title = ?", 0, invalidTitle)

	// ฝ่ายใหม่ใช้กลุ่มผู้ส่งสากลชุดเดียวกันโดยไม่ต้องสร้าง membership.
	emptyDivision := model.Division{
		Code:      "x" + suffix[:6],
		Name:      "ฝ่ายทดสอบว่าง " + suffix,
		ShortName: "ว่าง",
		SortOrder: 99,
	}
	if err := tx.Create(&emptyDivision).Error; err != nil {
		t.Fatalf("create empty division: %v", err)
	}
	emptyTitle := title + " empty division"
	emptyDivisionTopic, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  emptyDivision.UUID,
		CreatedByUUID: users[0].UUID,
		Title:         emptyTitle,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ข้อความ"}},
	})
	if err != nil {
		t.Fatalf("Create(new division) error = %v", err)
	}
	if int64(len(emptyDivisionTopic.Assignees)) != activeAccountCount {
		t.Fatalf("new division assignees = %d, want %d active accounts", len(emptyDivisionTopic.Assignees), activeAccountCount)
	}

	// หากไม่มีปีปัจจุบัน transaction ต้องหยุดก่อนสร้าง topic.
	if err := tx.Model(&model.AcademicYear{}).
		Where("is_current = ?", true).
		Update("is_current", false).Error; err != nil {
		t.Fatalf("clear current year for test: %v", err)
	}
	noYearTitle := title + " no year"
	_, err = service.Create(context.Background(), CreateInput{
		DivisionUUID:  divisions[0].UUID,
		CreatedByUUID: users[0].UUID,
		Title:         noYearTitle,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ข้อความ"}},
	})
	if !errors.Is(err, ErrCurrentAcademicYearNotSet) {
		t.Fatalf("Create(no current year) error = %v, want ErrCurrentAcademicYearNotSet", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "title = ?", 0, noYearTitle)
}

func TestTopicManagementServiceWithPostgreSQL(t *testing.T) {
	dsn := os.Getenv("SDMS_INTEGRATION_DATABASE_URL")
	if dsn == "" {
		t.Skip("set SDMS_INTEGRATION_DATABASE_URL to run the PostgreSQL integration test")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open PostgreSQL: %v", err)
	}
	tx := db.Begin()
	if tx.Error != nil {
		t.Fatalf("begin transaction: %v", tx.Error)
	}
	// ทุกการแก้ ปิด เปิด และลบใน test นี้ถูก rollback เพื่อไม่กระทบข้อมูลจริง.
	t.Cleanup(func() { _ = tx.Rollback().Error })

	var division model.Division
	if err := tx.Order("sort_order").First(&division).Error; err != nil {
		t.Fatalf("find division: %v", err)
	}
	suffix := uuid.NewString()[:8]
	creator := model.User{
		Username:     "be09." + suffix,
		PasswordHash: "integration-hash",
		FullName:     "ผู้สร้างหัวข้อ BE-09",
	}
	if err := tx.Create(&creator).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	secondAssignee := model.User{
		Username:     "be09." + suffix + ".second",
		PasswordHash: "integration-hash",
		FullName:     "ผู้รับผิดชอบคนที่สอง",
	}
	if err := tx.Create(&secondAssignee).Error; err != nil {
		t.Fatalf("create second assignee: %v", err)
	}

	service := NewService(tx)
	initialAssignees := []uuid.UUID{creator.UUID, secondAssignee.UUID}
	created, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  division.UUID,
		CreatedByUUID: creator.UUID,
		Title:         "BE-09 " + suffix,
		DueDate:       "2026-10-31",
		IsSecret:      true,
		Fields: []FieldInput{
			{Position: 1, Type: model.FieldText, Label: "ช่องที่มีคำตอบ", Required: true},
			{Position: 2, Type: model.FieldNum, Label: "ช่องที่ยังไม่มีคำตอบ", Required: false},
			{Position: 3, Type: model.FieldChoice, Label: "ช่องที่เก็บไว้", Required: true, Options: []string{"หนึ่ง", "สอง"}},
		},
		AssigneeUUIDs: &initialAssignees,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	answeredField := created.Fields[0]
	unusedField := created.Fields[1]
	retainedField := created.Fields[2]
	values := datatypes.JSON([]byte(fmt.Sprintf(`{"%s":"มีคำตอบแล้ว"}`, answeredField.UUID)))
	if err := tx.Create(&model.Submission{
		TopicUUID:   created.UUID,
		UserUUID:    creator.UUID,
		Values:      values,
		SubmittedAt: time.Now().UTC(),
	}).Error; err != nil {
		t.Fatalf("create submission: %v", err)
	}

	onlySecondAssignee := []uuid.UUID{secondAssignee.UUID}
	_, err = service.Update(context.Background(), created.UUID, UpdateInput{AssigneeUUIDs: &onlySecondAssignee})
	if !errors.Is(err, ErrAssigneeHasSubmission) {
		t.Fatalf("Update(remove submitted assignee) error = %v, want ErrAssigneeHasSubmission", err)
	}
	assertTopicRelatedCount(t, tx, &model.TopicAssignee{}, "topic_uuid = ?", 2, created.UUID)

	onlySubmittedAssignee := []uuid.UUID{creator.UUID}
	updatedAssignees, err := service.Update(context.Background(), created.UUID, UpdateInput{AssigneeUUIDs: &onlySubmittedAssignee})
	if err != nil {
		t.Fatalf("Update(remove unsubmitted assignee) error = %v", err)
	}
	if len(updatedAssignees.Assignees) != 1 || updatedAssignees.Assignees[0].UserUUID != creator.UUID {
		t.Fatalf("updated assignees = %+v", updatedAssignees.Assignees)
	}

	// เมื่อมีผู้ส่งแล้ว แม้ช่องนั้นยังไม่มีค่า ก็ห้ามเปลี่ยนชนิดของช่องเดิมทั้งฟอร์ม.
	typeChangedFields := []UpdateFieldInput{
		{UUID: answeredField.UUID, Position: 1, Type: answeredField.Type, Label: answeredField.Label},
		{UUID: unusedField.UUID, Position: 2, Type: model.FieldDate, Label: unusedField.Label},
		{UUID: retainedField.UUID, Position: 3, Type: retainedField.Type, Label: retainedField.Label, Options: []string{"หนึ่ง", "สอง"}},
	}
	_, err = service.Update(context.Background(), created.UUID, UpdateInput{Fields: &typeChangedFields})
	if !errors.Is(err, ErrFieldTypeLocked) {
		t.Fatalf("Update(type change) error = %v, want ErrFieldTypeLocked", err)
	}
	var unchanged model.TopicField
	if err := tx.Where("uuid = ?", unusedField.UUID).First(&unchanged).Error; err != nil {
		t.Fatalf("reload field after conflict: %v", err)
	}
	if unchanged.Type != model.FieldNum {
		t.Fatalf("field type after rollback = %s, want %s", unchanged.Type, model.FieldNum)
	}

	updatedTitle := "BE-09 แก้ไข " + suffix
	updatedDueDate := "2026-11-30"
	isSecret := false
	required := false
	updatedFields := []UpdateFieldInput{
		{
			UUID:     retainedField.UUID,
			Position: 1,
			Type:     model.FieldChoice,
			Label:    "ช่องเดิมที่แก้ชื่อ",
			Options:  []string{"A", "B", "C"},
		},
		{
			Position: 2,
			Type:     model.FieldFile,
			Label:    "ช่องใหม่",
			Required: &required,
		},
	}
	updated, err := service.Update(context.Background(), created.UUID, UpdateInput{
		Title:    &updatedTitle,
		DueDate:  &updatedDueDate,
		IsSecret: &isSecret,
		Fields:   &updatedFields,
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if updated.Title != updatedTitle || updated.DueDate.Format(dateLayout) != updatedDueDate || updated.IsSecret {
		t.Fatalf("updated topic = %+v", updated)
	}
	if len(updated.Fields) != 2 || updated.Fields[0].UUID != retainedField.UUID || updated.Fields[1].Type != model.FieldFile || updated.Fields[1].Required {
		t.Fatalf("updated fields = %+v", updated.Fields)
	}

	var hiddenAnswered model.TopicField
	if err := tx.Where("uuid = ?", answeredField.UUID).First(&hiddenAnswered).Error; err != nil {
		t.Fatalf("find answered field: %v", err)
	}
	if hiddenAnswered.HiddenAt == nil {
		t.Fatal("answered field was deleted instead of hidden")
	}
	assertTopicRelatedCount(t, tx, &model.TopicField{}, "uuid = ?", 0, unusedField.UUID)

	closed, err := service.SetOpen(context.Background(), created.UUID, false)
	if err != nil {
		t.Fatalf("SetOpen(false) error = %v", err)
	}
	if closed.IsOpen || closed.ClosedAt == nil {
		t.Fatalf("closed topic = %+v", closed)
	}
	lastClosedAt := *closed.ClosedAt
	closedAgain, err := service.SetOpen(context.Background(), created.UUID, false)
	if err != nil {
		t.Fatalf("SetOpen(false again) error = %v", err)
	}
	if closedAgain.ClosedAt == nil || !closedAgain.ClosedAt.Equal(lastClosedAt) || !closedAgain.UpdatedAt.Equal(closed.UpdatedAt) {
		t.Fatalf("idempotent close changed timestamps: first=%+v second=%+v", closed, closedAgain)
	}
	reopened, err := service.SetOpen(context.Background(), created.UUID, true)
	if err != nil {
		t.Fatalf("SetOpen(true) error = %v", err)
	}
	if !reopened.IsOpen || reopened.ClosedAt == nil || !reopened.ClosedAt.Equal(lastClosedAt) {
		t.Fatalf("reopened topic = %+v", reopened)
	}
	reopenedAgain, err := service.SetOpen(context.Background(), created.UUID, true)
	if err != nil {
		t.Fatalf("SetOpen(true again) error = %v", err)
	}
	if !reopenedAgain.UpdatedAt.Equal(reopened.UpdatedAt) {
		t.Fatalf("idempotent reopen changed updated_at: first=%s second=%s", reopened.UpdatedAt, reopenedAgain.UpdatedAt)
	}

	if err := service.Delete(context.Background(), created.UUID); !errors.Is(err, ErrTopicHasSubmissions) {
		t.Fatalf("Delete(topic with submission) error = %v, want ErrTopicHasSubmissions", err)
	}
	assertTopicRelatedCount(t, tx, &model.Topic{}, "uuid = ?", 1, created.UUID)

	deletable, err := service.Create(context.Background(), CreateInput{
		DivisionUUID:  division.UUID,
		CreatedByUUID: creator.UUID,
		Title:         "BE-09 delete " + suffix,
		DueDate:       "2026-10-31",
		Fields:        []FieldInput{{Type: model.FieldText, Label: "ลบได้"}},
	})
	if err != nil {
		t.Fatalf("Create(deletable) error = %v", err)
	}
	if err := service.Delete(context.Background(), deletable.UUID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}
	if _, err := service.Get(context.Background(), deletable.UUID); !errors.Is(err, ErrTopicNotFound) {
		t.Fatalf("Get(deleted topic) error = %v, want ErrTopicNotFound", err)
	}
}

func assertTopicRelatedCount(t *testing.T, db *gorm.DB, value any, query string, want int64, arguments ...any) {
	t.Helper()
	var count int64
	if err := db.Model(value).Where(query, arguments...).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != want {
		t.Fatalf("row count = %d, want %d", count, want)
	}
}
