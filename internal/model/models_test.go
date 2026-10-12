package model

import (
	"testing"

	"github.com/google/uuid"
)

func TestUUIDModelGeneratesOrderedVersion7Keys(t *testing.T) {
	var first User
	var second User
	if err := first.BeforeCreate(nil); err != nil {
		t.Fatalf("generate first UUID: %v", err)
	}
	if err := second.BeforeCreate(nil); err != nil {
		t.Fatalf("generate second UUID: %v", err)
	}

	if first.UUID.Version() != 7 || second.UUID.Version() != 7 {
		t.Fatalf("UUID versions = (%d, %d), want version 7", first.UUID.Version(), second.UUID.Version())
	}
	if first.UUID.String() >= second.UUID.String() {
		t.Fatalf("UUIDs are not ordered: %s >= %s", first.UUID, second.UUID)
	}
}

func TestUUIDModelPreservesSuppliedKey(t *testing.T) {
	want := uuid.MustParse("0199d16f-4f09-7c20-8cc7-e8c3363ea1b0")
	model := UUIDModel{UUID: want}
	if err := model.BeforeCreate(nil); err != nil {
		t.Fatalf("BeforeCreate() error = %v", err)
	}
	if model.UUID != want {
		t.Fatalf("UUID = %s, want supplied UUID %s", model.UUID, want)
	}
}
