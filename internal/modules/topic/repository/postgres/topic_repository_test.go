package postgres

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"sdms/internal/modules/topic/domain"

	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	gormpostgres "gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var testDB *gorm.DB

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := tcpostgres.Run(
		ctx,
		"postgres:17-alpine",
		tcpostgres.WithDatabase("sdms_test"),
		tcpostgres.WithUsername("sdms"),
		tcpostgres.WithPassword("sdms_password"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start postgres container: %v\n", err)
		os.Exit(1)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		fmt.Fprintf(os.Stderr, "failed to get connection string: %v\n", err)
		os.Exit(1)
	}

	testDB, err = gorm.Open(
		gormpostgres.Open(dsn),
		&gorm.Config{},
	)
	if err != nil {
		_ = testcontainers.TerminateContainer(container)
		fmt.Fprintf(os.Stderr, "failed to connect database: %v\n", err)
		os.Exit(1)
	}

	if err := testDB.AutoMigrate(&TopicModel{}); err != nil {
		_ = testcontainers.TerminateContainer(container)
		fmt.Fprintf(os.Stderr, "failed to migrate database: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	if err := testcontainers.TerminateContainer(container); err != nil {
		fmt.Fprintf(os.Stderr, "failed to terminate postgres container: %v\n", err)
	}

	os.Exit(code)
}

func cleanDatabase(t *testing.T) {
	t.Helper()

	if err := testDB.Exec("TRUNCATE TABLE topics").Error; err != nil {
		t.Fatalf("failed to clean database: %v", err)
	}
}

func TestRepository_Create_Success(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)
	topicID := uuid.New()

	topic := &domain.Topic{
		UID:         topicID,
		Name:        "งานวิชาการ",
		Description: "เอกสารงานวิชาการ",
		IsActive:    true,
	}

	err := repository.Create(context.Background(), topic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topic.UID != topicID {
		t.Errorf("expected ID %s, got %s", topicID, topic.UID)
	}

	if topic.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be populated")
	}

	if topic.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be populated")
	}

	var model TopicModel

	err = testDB.
		Where("uid = ?", topicID).
		First(&model).
		Error
	if err != nil {
		t.Fatalf("failed to find topic directly from database: %v", err)
	}

	if model.Name != "งานวิชาการ" {
		t.Errorf("expected name %q, got %q", "งานวิชาการ", model.Name)
	}

	if model.Description != "เอกสารงานวิชาการ" {
		t.Errorf("unexpected description: %q", model.Description)
	}

	if !model.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestRepository_FindByID_Success(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)
	topicID := uuid.New()

	err := testDB.Create(&TopicModel{
		UID:         topicID,
		Name:        "งานวิชาการ",
		Description: "รายละเอียด",
		IsActive:    true,
	}).Error
	if err != nil {
		t.Fatalf("failed to prepare test data: %v", err)
	}

	topic, err := repository.FindByID(context.Background(), topicID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topic == nil {
		t.Fatal("expected topic, got nil")
	}

	if topic.UID != topicID {
		t.Errorf("expected ID %s, got %s", topicID, topic.UID)
	}

	if topic.Name != "งานวิชาการ" {
		t.Errorf("expected name %q, got %q", "งานวิชาการ", topic.Name)
	}

	if topic.Description != "รายละเอียด" {
		t.Errorf("unexpected description: %q", topic.Description)
	}

	if !topic.IsActive {
		t.Error("expected IsActive to be true")
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)
	topicID := uuid.New()

	topic, err := repository.FindByID(context.Background(), topicID)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf("expected ErrTopicNotFound, got %v", err)
	}

	if topic != nil {
		t.Errorf("expected nil topic, got %+v", topic)
	}
}

func TestRepository_FindAll_Success(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)

	topic1 := TopicModel{
		UID:         uuid.New(),
		Name:        "งานวิชาการ",
		Description: "Topic 1",
		IsActive:    true,
	}

	topic2 := TopicModel{
		UID:         uuid.New(),
		Name:        "งานบุคคล",
		Description: "Topic 2",
		IsActive:    true,
	}

	if err := testDB.Create(&[]TopicModel{topic1, topic2}).Error; err != nil {
		t.Fatalf("failed to prepare test data: %v", err)
	}

	topics, err := repository.FindAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(topics) != 2 {
		t.Fatalf("expected 2 topics, got %d", len(topics))
	}

	found := make(map[uuid.UUID]domain.Topic, len(topics))
	for _, topic := range topics {
		found[topic.UID] = topic
	}

	if _, ok := found[topic1.UID]; !ok {
		t.Errorf("topic %s was not found", topic1.UID)
	}

	if _, ok := found[topic2.UID]; !ok {
		t.Errorf("topic %s was not found", topic2.UID)
	}
}

func TestRepository_FindAll_Empty(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)

	topics, err := repository.FindAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if topics == nil {
		t.Fatal("expected empty slice, got nil")
	}

	if len(topics) != 0 {
		t.Errorf("expected 0 topics, got %d", len(topics))
	}
}

func TestRepository_Update_Success(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)
	topicID := uuid.New()

	if err := testDB.Create(&TopicModel{
		UID:         topicID,
		Name:        "ชื่อเดิม",
		Description: "รายละเอียดเดิม",
		IsActive:    true,
	}).Error; err != nil {
		t.Fatalf("failed to prepare test data: %v", err)
	}

	topic := &domain.Topic{
		UID:         topicID,
		Name:        "ชื่อใหม่",
		Description: "รายละเอียดใหม่",
		IsActive:    false,
	}

	err := repository.Update(context.Background(), topic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var model TopicModel

	if err := testDB.
		Where("uid = ?", topicID).
		First(&model).
		Error; err != nil {
		t.Fatalf("failed to query updated topic: %v", err)
	}

	if model.Name != "ชื่อใหม่" {
		t.Errorf("expected name %q, got %q", "ชื่อใหม่", model.Name)
	}

	if model.Description != "รายละเอียดใหม่" {
		t.Errorf("unexpected description: %q", model.Description)
	}

	if model.IsActive {
		t.Error("expected IsActive to be false")
	}
}

func TestRepository_Update_NotFound(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)

	topic := &domain.Topic{
		UID:         uuid.New(),
		Name:        "ไม่อยู่ในระบบ",
		Description: "test",
		IsActive:    true,
	}

	err := repository.Update(context.Background(), topic)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf("expected ErrTopicNotFound, got %v", err)
	}
}

func TestRepository_Delete_Success(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)
	topicID := uuid.New()

	if err := testDB.Create(&TopicModel{
		UID:         topicID,
		Name:        "งานวิชาการ",
		Description: "test",
		IsActive:    true,
	}).Error; err != nil {
		t.Fatalf("failed to prepare test data: %v", err)
	}

	err := repository.Delete(context.Background(), topicID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64

	if err := testDB.
		Model(&TopicModel{}).
		Where("uid = ?", topicID).
		Count(&count).
		Error; err != nil {
		t.Fatalf("failed to count topic: %v", err)
	}

	if count != 0 {
		t.Errorf("expected topic to be deleted, count = %d", count)
	}
}

func TestRepository_Delete_NotFound(t *testing.T) {
	cleanDatabase(t)

	repository := NewRepository(testDB)

	err := repository.Delete(
		context.Background(),
		uuid.New(),
	)

	if !errors.Is(err, domain.ErrTopicNotFound) {
		t.Errorf("expected ErrTopicNotFound, got %v", err)
	}
}
