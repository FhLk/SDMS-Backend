// Package authorization รวมกฎสิทธิ์ของระบบไว้ที่เดียว เพื่อให้ทุก API
// ตัดสินสิทธิ์เหมือนกันและไม่ต้องเขียนเงื่อนไขบทบาทซ้ำในแต่ละ handler.
package authorization

import (
	"github.com/FhLk/SDMS-Backend/internal/auth"
	"github.com/FhLk/SDMS-Backend/internal/model"
	"github.com/google/uuid"
)

// CanManageUsers อนุญาตเฉพาะผู้ใช้ที่ยืนยันตัวตนและมีบทบาทแอดมิน.
func CanManageUsers(profile auth.Profile) bool {
	return isAuthenticated(profile) && profile.User.IsAdmin
}

// CanManageTopic ตรวจว่าผู้ใช้สร้างหัวข้อของฝ่ายที่ระบุได้หรือไม่.
// แอดมินสร้างได้ทุกฝ่าย ส่วนหัวหน้าฝ่ายสร้างได้เฉพาะฝ่ายที่ตนเป็นหัวหน้า.
func CanManageTopic(profile auth.Profile, divisionUUID uuid.UUID) bool {
	if !isAuthenticated(profile) || divisionUUID == uuid.Nil {
		return false
	}

	return profile.User.IsAdmin || isHeadOf(profile, divisionUUID)
}

// CanEditTopic ใช้กับหัวข้อที่มีอยู่แล้ว โดยให้คนสร้างยังจัดการหัวข้อเดิมได้
// แม้ภายหลังจะไม่ได้เป็นหัวหน้าฝ่ายนั้นแล้ว ตามกติกาของ BE-09.
func CanEditTopic(profile auth.Profile, topic model.Topic) bool {
	if !isAuthenticated(profile) || topic.UUID == uuid.Nil {
		return false
	}

	return profile.User.IsAdmin ||
		isHeadOf(profile, topic.DivisionUUID) ||
		(topic.CreatedByUUID != uuid.Nil && topic.CreatedByUUID == profile.User.UUID)
}

// CanViewAllTopicSubmissions ตรวจสิทธิ์ดูคำตอบของผู้ส่งทุกคนในหัวข้อ.
// หัวข้อทั่วไปเปิดให้ผู้ใช้ที่เข้าสู่ระบบดูได้ แต่หัวข้อลับอนุญาตเฉพาะ
// ผู้บริหาร หัวหน้าฝ่ายเจ้าของหัวข้อ หรือผู้สร้างหัวข้อนั้น.
func CanViewAllTopicSubmissions(profile auth.Profile, topic model.Topic) bool {
	if !isAuthenticated(profile) {
		return false
	}
	if !topic.IsSecret {
		return true
	}

	return profile.User.IsExec ||
		isHeadOf(profile, topic.DivisionUUID) ||
		(topic.CreatedByUUID != uuid.Nil && topic.CreatedByUUID == profile.User.UUID)
}

// CanViewDivisionStats ตรวจสิทธิ์ดูสถิติของฝ่าย.
// ผู้บริหารดูได้ทุกฝ่าย ส่วนหัวหน้าฝ่ายดูได้เฉพาะฝ่ายของตน.
func CanViewDivisionStats(profile auth.Profile, divisionUUID uuid.UUID) bool {
	if !isAuthenticated(profile) || divisionUUID == uuid.Nil {
		return false
	}

	return profile.User.IsExec || isHeadOf(profile, divisionUUID)
}

// isAuthenticated ป้องกันไม่ให้ profile ว่างถูกนำไปใช้เป็นผู้ใช้ที่ผ่านการยืนยันแล้ว.
func isAuthenticated(profile auth.Profile) bool {
	return profile.User.UUID != uuid.Nil
}

// isHeadOf เปรียบเทียบ UUID ของฝ่ายเป้าหมายกับรายการฝ่ายที่ผู้ใช้เป็นหัวหน้า.
func isHeadOf(profile auth.Profile, divisionUUID uuid.UUID) bool {
	if divisionUUID == uuid.Nil {
		return false
	}

	for _, division := range profile.HeadDivisions {
		if division.UUID == divisionUUID {
			return true
		}
	}
	return false
}
