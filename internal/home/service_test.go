package home

import (
	"fmt"
	"testing"
	"time"

	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

func TestBuildPersonalCountsStatusesAndDivisions(t *testing.T) {
	firstDivision := model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "aca", SortOrder: 1}
	secondDivision := model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "bud", SortOrder: 2}
	dueToday := time.Date(2026, time.October, 13, 0, 0, 0, 0, time.UTC)
	dueYesterday := time.Date(2026, time.October, 12, 0, 0, 0, 0, time.UTC)
	onTime := time.Date(2026, time.October, 12, 16, 59, 0, 0, time.UTC)
	late := time.Date(2026, time.October, 12, 17, 0, 0, 0, time.UTC)
	onTimeUUID := uuid.New()
	lateUUID := uuid.New()

	personal := buildPersonal([]assignmentRow{
		assignmentFixture(firstDivision, "ส่งทัน", dueYesterday, &onTimeUUID, &onTime),
		assignmentFixture(firstDivision, "เลยกำหนด", dueYesterday, nil, nil),
		assignmentFixture(secondDivision, "ยังไม่ส่ง", dueToday, nil, nil),
		assignmentFixture(secondDivision, "ส่งช้า", dueYesterday, &lateUUID, &late),
	}, []model.Division{firstDivision, secondDivision}, time.Date(2026, time.October, 13, 12, 0, 0, 0, bangkokLocation))

	want := Counts{TopicCount: 4, RequiredCount: 4, SubmittedCount: 2, PendingCount: 1, OverdueCount: 1}
	if personal.Counts != want {
		t.Fatalf("personal counts = %+v, want %+v", personal.Counts, want)
	}
	if len(personal.PendingTopics) != 2 || personal.PendingTopics[0].Status != StatusOverdue || personal.PendingTopics[1].Status != StatusPending {
		t.Fatalf("pending topics = %+v", personal.PendingTopics)
	}
	if len(personal.RecentSubmissions) != 2 || personal.RecentSubmissions[0].Status != StatusLate || personal.RecentSubmissions[1].Status != StatusSubmitted {
		t.Fatalf("recent submissions = %+v", personal.RecentSubmissions)
	}
	if got := personal.Divisions[0].Counts; got.SubmittedCount != 1 || got.OverdueCount != 1 {
		t.Fatalf("first division counts = %+v", got)
	}
	if got := personal.Divisions[1].Counts; got.SubmittedCount != 1 || got.PendingCount != 1 {
		t.Fatalf("second division counts = %+v", got)
	}
}

func TestBuildPersonalKeepsOnlyFiveMostRecentSubmissions(t *testing.T) {
	division := model.Division{UUIDModel: model.UUIDModel{UUID: uuid.New()}, Code: "aca"}
	rows := make([]assignmentRow, 0, 6)
	for index := 0; index < 6; index++ {
		submissionUUID := uuid.New()
		submittedAt := time.Date(2026, time.October, index+1, 0, 0, 0, 0, time.UTC)
		rows = append(rows, assignmentFixture(division, fmt.Sprintf("หัวข้อ %d", index), submittedAt, &submissionUUID, &submittedAt))
	}
	personal := buildPersonal(rows, []model.Division{division}, time.Now())
	if len(personal.RecentSubmissions) != 5 {
		t.Fatalf("recent submission count = %d, want 5", len(personal.RecentSubmissions))
	}
	if got := personal.RecentSubmissions[0].SubmittedAt.Day(); got != 6 {
		t.Fatalf("newest submission day = %d, want 6", got)
	}
	if got := personal.RecentSubmissions[4].SubmittedAt.Day(); got != 2 {
		t.Fatalf("oldest retained submission day = %d, want 2", got)
	}
}

func TestSumDivisionCounts(t *testing.T) {
	total := sumDivisionCounts([]DivisionSummary{
		{Counts: Counts{TopicCount: 2, RequiredCount: 5, SubmittedCount: 3, PendingCount: 1, OverdueCount: 1}},
		{Counts: Counts{TopicCount: 1, RequiredCount: 4, SubmittedCount: 2, PendingCount: 2}},
	})
	want := Counts{TopicCount: 3, RequiredCount: 9, SubmittedCount: 5, PendingCount: 3, OverdueCount: 1}
	if total != want {
		t.Fatalf("school counts = %+v, want %+v", total, want)
	}
}

func assignmentFixture(division model.Division, title string, dueDate time.Time, submissionUUID *uuid.UUID, submittedAt *time.Time) assignmentRow {
	return assignmentRow{
		TopicUUID:        uuid.New(),
		Title:            title,
		DueDate:          dueDate,
		DivisionUUID:     division.UUID,
		DivisionCode:     division.Code,
		DivisionName:     division.Name,
		DivisionShort:    division.ShortName,
		DivisionOrder:    division.SortOrder,
		AcademicYear:     2569,
		Semester:         1,
		SubmissionUUID:   submissionUUID,
		SubmittedAt:      submittedAt,
		SubmissionUpdate: submittedAt,
	}
}
