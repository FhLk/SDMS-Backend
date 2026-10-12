package authorization

import (
	"testing"

	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

var (
	testUserUUID      = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f810001")
	testCreatorUUID   = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f810002")
	testDivisionUUID  = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f810003")
	testOtherDivision = uuid.MustParse("0199d6d0-bec9-7aa0-b2d8-96dc6f810004")
)

func TestCanManageUsers(t *testing.T) {
	tests := []struct {
		name    string
		profile auth.Profile
		want    bool
	}{
		{name: "admin is allowed", profile: profileWithRoles(true, false), want: true},
		{name: "executive is denied", profile: profileWithRoles(false, true)},
		{name: "teacher is denied", profile: authenticatedProfile()},
		{name: "empty profile is denied", profile: auth.Profile{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanManageUsers(tt.profile); got != tt.want {
				t.Fatalf("CanManageUsers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanManageTopic(t *testing.T) {
	tests := []struct {
		name    string
		profile auth.Profile
		want    bool
	}{
		{name: "admin can manage every division", profile: profileWithRoles(true, false), want: true},
		{name: "head can manage own division", profile: profileWithHead(testDivisionUUID), want: true},
		{name: "head cannot manage another division", profile: profileWithHead(testOtherDivision)},
		{name: "executive cannot manage topic by role alone", profile: profileWithRoles(false, true)},
		{name: "teacher cannot manage topic", profile: authenticatedProfile()},
		{name: "empty profile is denied", profile: auth.Profile{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanManageTopic(tt.profile, testDivisionUUID); got != tt.want {
				t.Fatalf("CanManageTopic() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanViewAllTopicSubmissions(t *testing.T) {
	secretTopic := model.Topic{
		DivisionUUID:  testDivisionUUID,
		CreatedByUUID: testCreatorUUID,
		IsSecret:      true,
	}
	publicTopic := secretTopic
	publicTopic.IsSecret = false

	tests := []struct {
		name    string
		profile auth.Profile
		topic   model.Topic
		want    bool
	}{
		{name: "authenticated user can view public topic", profile: authenticatedProfile(), topic: publicTopic, want: true},
		{name: "executive can view secret topic", profile: profileWithRoles(false, true), topic: secretTopic, want: true},
		{name: "division head can view own secret topic", profile: profileWithHead(testDivisionUUID), topic: secretTopic, want: true},
		{name: "other division head cannot view secret topic", profile: profileWithHead(testOtherDivision), topic: secretTopic},
		{name: "creator can view own secret topic", profile: profileWithUser(testCreatorUUID), topic: secretTopic, want: true},
		{name: "teacher cannot view secret topic", profile: authenticatedProfile(), topic: secretTopic},
		{name: "admin has no content access by role alone", profile: profileWithRoles(true, false), topic: secretTopic},
		{name: "empty profile cannot view public topic", profile: auth.Profile{}, topic: publicTopic},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanViewAllTopicSubmissions(tt.profile, tt.topic); got != tt.want {
				t.Fatalf("CanViewAllTopicSubmissions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCanViewDivisionStats(t *testing.T) {
	tests := []struct {
		name    string
		profile auth.Profile
		want    bool
	}{
		{name: "executive can view every division", profile: profileWithRoles(false, true), want: true},
		{name: "head can view own division", profile: profileWithHead(testDivisionUUID), want: true},
		{name: "head cannot view another division", profile: profileWithHead(testOtherDivision)},
		{name: "teacher cannot view division stats", profile: authenticatedProfile()},
		{name: "admin has no stats access by role alone", profile: profileWithRoles(true, false)},
		{name: "empty profile is denied", profile: auth.Profile{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanViewDivisionStats(tt.profile, testDivisionUUID); got != tt.want {
				t.Fatalf("CanViewDivisionStats() = %v, want %v", got, tt.want)
			}
		})
	}
}

func authenticatedProfile() auth.Profile {
	return profileWithUser(testUserUUID)
}

func profileWithUser(userUUID uuid.UUID) auth.Profile {
	return auth.Profile{User: model.User{UUIDModel: model.UUIDModel{UUID: userUUID}}}
}

func profileWithRoles(admin, executive bool) auth.Profile {
	profile := authenticatedProfile()
	profile.User.IsAdmin = admin
	profile.User.IsExec = executive
	return profile
}

func profileWithHead(divisionUUID uuid.UUID) auth.Profile {
	profile := authenticatedProfile()
	profile.HeadDivisions = []model.Division{{
		UUIDModel: model.UUIDModel{UUID: divisionUUID},
	}}
	return profile
}
