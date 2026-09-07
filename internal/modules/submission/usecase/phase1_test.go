package usecase

import (
	"context"
	"errors"
	"testing"

	submissiondomain "sdms/internal/modules/submission/domain"
	topicdomain "sdms/internal/modules/topic/domain"
	userdomain "sdms/internal/modules/user/domain"

	"github.com/google/uuid"
)

type fakeUserDirectory struct {
	users []userdomain.User
}

func (f *fakeUserDirectory) FindByID(_ context.Context, id uuid.UUID) (*userdomain.User, error) {
	for i := range f.users {
		if f.users[i].UID == id {
			return &f.users[i], nil
		}
	}
	return nil, userdomain.ErrUserNotFound
}

func (f *fakeUserDirectory) List(context.Context) ([]userdomain.User, error) {
	return append([]userdomain.User(nil), f.users...), nil
}

func TestPhase1RequiredFileControlsCompletion(t *testing.T) {
	topicUID := uuid.New()
	teacherUID := uuid.New()
	fileFieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
		return &topicdomain.Topic{UID: topicUID, AcademicYear: "2569", Name: "หลักฐาน", IsActive: true, FormVersion: 2}, nil
	}}
	fieldRepo := &fakeFieldRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]topicdomain.TopicField, error) {
		return []topicdomain.TopicField{{UID: fileFieldUID, TopicUID: topicUID, Label: "ไฟล์หลักฐาน", Type: topicdomain.FieldTypeFile, Required: true}}, nil
	}}
	users := &fakeUserDirectory{users: []userdomain.User{{UID: teacherUID, Role: userdomain.RoleTeacher, Status: userdomain.StatusActive}}}

	service := NewSubmissionService(&fakeSubmissionRepository{}, topicRepo, fieldRepo, users)
	submission, err := service.Create(context.Background(), topicUID, CreateSubmissionInput{SubmittedBy: teacherUID})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if submission.CompletionStatus != submissiondomain.CompletionIncomplete {
		t.Fatalf("completion = %s, want INCOMPLETE", submission.CompletionStatus)
	}
	if len(submission.MissingRequiredFields) != 1 || submission.MissingRequiredFields[0].FieldUID != fileFieldUID {
		t.Fatalf("missing fields = %+v", submission.MissingRequiredFields)
	}
	if submission.FormVersion != 2 || len(submission.FormSnapshot) != 1 {
		t.Fatalf("form version/snapshot = %d / %+v", submission.FormVersion, submission.FormSnapshot)
	}
}

func TestPhase1TopicSubmissionStatusIncludesTeachersWhoDidNotSubmit(t *testing.T) {
	topicUID := uuid.New()
	teacherDone := uuid.New()
	teacherMissing := uuid.New()
	fieldUID := uuid.New()
	text := "กิจกรรม A"

	topicRepo := &fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
		return &topicdomain.Topic{UID: topicUID, AcademicYear: "2569", Name: "งาน", IsActive: true, FormVersion: 1}, nil
	}}
	fieldRepo := &fakeFieldRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]topicdomain.TopicField, error) {
		return []topicdomain.TopicField{{UID: fieldUID, TopicUID: topicUID, Label: "ชื่อกิจกรรม", Type: topicdomain.FieldTypeText, Required: true}}, nil
	}}
	submissionRepo := &fakeSubmissionRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]submissiondomain.Submission, error) {
		return []submissiondomain.Submission{{
			UID: uuid.New(), TopicUID: topicUID, SubmittedBy: teacherDone,
			Values: []submissiondomain.SubmissionValue{{FieldUID: fieldUID, TextValue: &text}},
		}}, nil
	}}
	users := &fakeUserDirectory{users: []userdomain.User{
		{UID: teacherDone, FirstName: "ส่งแล้ว", Role: userdomain.RoleTeacher, Status: userdomain.StatusActive},
		{UID: teacherMissing, FirstName: "ยังไม่ส่ง", Role: userdomain.RoleTeacher, Status: userdomain.StatusActive},
		{UID: uuid.New(), Role: userdomain.RoleDirector, Status: userdomain.StatusActive},
	}}

	service := NewSubmissionService(submissionRepo, topicRepo, fieldRepo, users)
	status, err := service.GetTopicSubmissionStatus(context.Background(), topicUID)
	if err != nil {
		t.Fatalf("GetTopicSubmissionStatus() error = %v", err)
	}
	if len(status) != 2 {
		t.Fatalf("status len = %d, want 2 teachers", len(status))
	}
	byTeacher := map[uuid.UUID]TopicSubmissionStatus{}
	for _, item := range status {
		byTeacher[item.Teacher.UID] = item
	}
	if byTeacher[teacherDone].Status != "COMPLETE" {
		t.Fatalf("submitted teacher status = %s", byTeacher[teacherDone].Status)
	}
	if byTeacher[teacherMissing].Status != "NOT_SUBMITTED" {
		t.Fatalf("missing teacher status = %s", byTeacher[teacherMissing].Status)
	}
}

func TestPhase1TeacherCanUpdateOwnSubmission(t *testing.T) {
	topicUID := uuid.New()
	teacherUID := uuid.New()
	submissionUID := uuid.New()
	fieldUID := uuid.New()

	topicRepo := &fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
		return &topicdomain.Topic{UID: topicUID, AcademicYear: "2569", Name: "งาน", IsActive: true, FormVersion: 3}, nil
	}}
	fieldRepo := &fakeFieldRepository{findAllByTopicIDFn: func(context.Context, uuid.UUID) ([]topicdomain.TopicField, error) {
		return []topicdomain.TopicField{{UID: fieldUID, TopicUID: topicUID, Label: "ชื่อกิจกรรม", Type: topicdomain.FieldTypeText, Required: true, IsPreview: true}}, nil
	}}

	existing := &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID, SubmittedBy: teacherUID, FormVersion: 1}
	updatedCalled := false
	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDAndSubmittedByFn: func(_ context.Context, gotSubmissionUID, gotTopicUID, gotTeacherUID uuid.UUID) (*submissiondomain.Submission, error) {
			if gotSubmissionUID != submissionUID || gotTopicUID != topicUID || gotTeacherUID != teacherUID {
				t.Fatalf("ownership lookup = %s / %s / %s", gotSubmissionUID, gotTopicUID, gotTeacherUID)
			}
			return existing, nil
		},
		updateValuesFn: func(_ context.Context, got *submissiondomain.Submission) error {
			updatedCalled = true
			if got.UID != submissionUID || got.FormVersion != 3 || len(got.FormSnapshot) != 1 || len(got.Values) != 1 {
				t.Fatalf("updated submission = %+v", got)
			}
			return nil
		},
	}

	service := NewSubmissionService(submissionRepo, topicRepo, fieldRepo)
	updated, err := service.UpdateForSubmitter(context.Background(), topicUID, submissionUID, teacherUID, UpdateSubmissionInput{
		Values: []SubmissionValueInput{{FieldUID: fieldUID, Value: []byte(`"กิจกรรมใหม่"`)}},
	})
	if err != nil {
		t.Fatalf("UpdateForSubmitter() error = %v", err)
	}
	if !updatedCalled {
		t.Fatal("repository UpdateValues was not called")
	}
	if updated.CompletionStatus != submissiondomain.CompletionComplete || len(updated.Values) != 1 || updated.Values[0].TextValue == nil || *updated.Values[0].TextValue != "กิจกรรมใหม่" {
		t.Fatalf("updated result = %+v", updated)
	}
}

func TestPhase1TeacherCannotUpdateAnotherTeachersSubmission(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()
	otherTeacherUID := uuid.New()

	topicRepo := &fakeTopicRepository{findByIDFn: func(context.Context, uuid.UUID) (*topicdomain.Topic, error) {
		return &topicdomain.Topic{UID: topicUID, AcademicYear: "2569", Name: "งาน", IsActive: true, FormVersion: 1}, nil
	}}
	updatedCalled := false
	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDAndSubmittedByFn: func(_ context.Context, _, _, gotTeacherUID uuid.UUID) (*submissiondomain.Submission, error) {
			if gotTeacherUID == teacherUID {
				return nil, submissiondomain.ErrSubmissionNotFound
			}
			return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID, SubmittedBy: otherTeacherUID}, nil
		},
		updateValuesFn: func(context.Context, *submissiondomain.Submission) error {
			updatedCalled = true
			return nil
		},
	}

	service := NewSubmissionService(submissionRepo, topicRepo, &fakeFieldRepository{})
	_, err := service.UpdateForSubmitter(context.Background(), topicUID, submissionUID, teacherUID, UpdateSubmissionInput{})
	if !errors.Is(err, submissiondomain.ErrSubmissionNotFound) {
		t.Fatalf("UpdateForSubmitter() error = %v, want not found", err)
	}
	if updatedCalled {
		t.Fatal("repository UpdateValues must not be called for another teacher's submission")
	}
}

func TestPhase1TeacherCanDeleteOwnSubmission(t *testing.T) {
	topicUID := uuid.New()
	submissionUID := uuid.New()
	teacherUID := uuid.New()
	deleted := uuid.Nil

	submissionRepo := &fakeSubmissionRepository{
		findByIDAndTopicIDAndSubmittedByFn: func(_ context.Context, gotSubmissionUID, gotTopicUID, gotTeacherUID uuid.UUID) (*submissiondomain.Submission, error) {
			if gotSubmissionUID != submissionUID || gotTopicUID != topicUID || gotTeacherUID != teacherUID {
				t.Fatalf("ownership lookup = %s / %s / %s", gotSubmissionUID, gotTopicUID, gotTeacherUID)
			}
			return &submissiondomain.Submission{UID: submissionUID, TopicUID: topicUID, SubmittedBy: teacherUID}, nil
		},
		deleteFn: func(_ context.Context, got uuid.UUID) error {
			deleted = got
			return nil
		},
	}

	service := NewSubmissionService(submissionRepo, &fakeTopicRepository{}, &fakeFieldRepository{})
	if err := service.DeleteForSubmitter(context.Background(), topicUID, submissionUID, teacherUID); err != nil {
		t.Fatalf("DeleteForSubmitter() error = %v", err)
	}
	if deleted != submissionUID {
		t.Fatalf("deleted uid = %s, want %s", deleted, submissionUID)
	}
}
