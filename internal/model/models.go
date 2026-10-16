// Package model: GORM models ของ SDMS — ตรงกับพจนานุกรมข้อมูล
package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// UUIDModel gives every application table the same UUIDv7 primary key.
// UUIDv7 keeps newly inserted keys time-ordered, which is friendlier to the
// PostgreSQL B-tree primary-key index than fully random UUIDv4 values.
type UUIDModel struct {
	UUID uuid.UUID `gorm:"type:uuid;primaryKey" json:"uuid"`
}

// BeforeCreate generates a UUIDv7 unless a migration or caller supplied one.
// The hook is promoted to every model that embeds UUIDModel.
func (m *UUIDModel) BeforeCreate(_ *gorm.DB) error {
	if m.UUID != uuid.Nil {
		return nil
	}

	generated, err := uuid.NewV7()
	if err != nil {
		return err
	}
	m.UUID = generated
	return nil
}

// ประเภทช่องในฟอร์ม
const (
	FieldText   = "text"   // ข้อความ
	FieldPara   = "para"   // ข้อความยาว
	FieldNum    = "num"    // ตัวเลข
	FieldDate   = "date"   // วันที่
	FieldChoice = "choice" // ตัวเลือก
	FieldFile   = "file"   // ไฟล์
)

// สถานะไฟล์
const (
	FileStaged   = "staged"   // อัปโหลดแล้ว ยังไม่ได้ผูกกับคำตอบ
	FileAttached = "attached" // ผูกกับคำตอบแล้ว
	FileTrashed  = "trashed"  // ถูกเอาออกและลบ object จาก storage แล้ว
)

// User บุคลากร 1 คน — ไม่ลบจริง ใช้ DeactivatedAt แทน
type User struct {
	UUIDModel
	Username     string `gorm:"size:32;not null;uniqueIndex;check:chk_users_username,username ~ '^[a-z0-9._-]{3,32}$'"`
	PasswordHash string `gorm:"not null" json:"-"`
	Prefix       string `gorm:"size:30;not null;default:''"`
	FirstName    string `gorm:"size:120;not null"`
	LastName     string `gorm:"size:120;not null;default:''"`
	// FullName is a transitional, non-persistent field for internal callers that
	// still construct User values with a display name. New code stores the three
	// name components above and uses DisplayName when rendering a name.
	FullName      string     `gorm:"-" json:"-"`
	Position      string     `gorm:"size:120;not null"`
	IsExec        bool       `gorm:"not null;default:false"`
	IsAdmin       bool       `gorm:"not null;default:false"`
	DeactivatedAt *time.Time `gorm:"index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Divisions []Division `gorm:"many2many:user_divisions;foreignKey:UUID;joinForeignKey:UserUUID;references:UUID;joinReferences:DivisionUUID"`
}

// Division ฝ่าย 4 ฝ่าย — หัวหน้าฝ่ายละ 1 คน และ 1 คนเป็นหัวหน้าได้ฝ่ายเดียว
type Division struct {
	UUIDModel
	Code         string     `gorm:"size:8;not null;uniqueIndex"` // aca | bud | per | gen
	Name         string     `gorm:"size:120;not null"`
	ShortName    string     `gorm:"size:60;not null"`
	SortOrder    int16      `gorm:"not null"`
	HeadUserUUID *uuid.UUID `gorm:"type:uuid;uniqueIndex"`

	HeadUser *User `gorm:"foreignKey:HeadUserUUID;references:UUID;constraint:OnDelete:SET NULL"`
}

// UserDivision การสังกัดฝ่าย (1 คนสังกัดได้หลายฝ่าย)
type UserDivision struct {
	UUIDModel
	UserUUID     uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_user_divisions_user_division,priority:1;index"`
	DivisionUUID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_user_divisions_user_division,priority:2;index"`

	User     User     `gorm:"foreignKey:UserUUID;references:UUID;constraint:OnDelete:CASCADE"`
	Division Division `gorm:"foreignKey:DivisionUUID;references:UUID;constraint:OnDelete:CASCADE"`
}

// AcademicYear ปีการศึกษา (พ.ศ.) — ปีปัจจุบันมีได้ปีเดียว
type AcademicYear struct {
	UUIDModel
	Year            int16 `gorm:"type:smallint;not null;uniqueIndex"` // เช่น 2569
	IsCurrent       bool  `gorm:"not null;default:false;uniqueIndex:uq_academic_years_current,where:is_current"`
	CurrentSemester int16 `gorm:"not null;default:1;check:chk_academic_years_semester,current_semester IN (1,2)"`
	CreatedAt       time.Time

	Topics []Topic `gorm:"foreignKey:AcademicYearUUID;references:UUID;constraint:OnDelete:RESTRICT"`
}

// Topic หัวข้อที่หัวหน้าฝ่ายเปิดรับ
type Topic struct {
	UUIDModel
	DivisionUUID     uuid.UUID `gorm:"type:uuid;not null;index:idx_topics_division_year,priority:1"`
	AcademicYearUUID uuid.UUID `gorm:"type:uuid;not null;index:idx_topics_division_year,priority:2"`
	Semester         int16     `gorm:"not null;check:chk_topics_semester,semester IN (1,2)"`
	Title            string    `gorm:"size:200;not null"`
	DueDate          time.Time `gorm:"type:date;not null;index"`
	IsOpen           bool      `gorm:"not null;default:true"`
	IsSecret         bool      `gorm:"not null;default:false"`
	CreatedByUUID    uuid.UUID `gorm:"type:uuid;not null"`
	ClosedAt         *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time

	Division     Division        `gorm:"foreignKey:DivisionUUID;references:UUID;constraint:OnDelete:RESTRICT"`
	AcademicYear AcademicYear    `gorm:"foreignKey:AcademicYearUUID;references:UUID;constraint:OnDelete:RESTRICT"`
	CreatedBy    User            `gorm:"foreignKey:CreatedByUUID;references:UUID;constraint:OnDelete:RESTRICT"`
	Fields       []TopicField    `gorm:"constraint:OnDelete:CASCADE"`
	Assignees    []TopicAssignee `gorm:"constraint:OnDelete:CASCADE"`
}

// TopicField ช่องในฟอร์มของหัวข้อ — ช่องที่มีคำตอบแล้วถูก "ลบ" = ตั้ง HiddenAt
type TopicField struct {
	UUIDModel
	TopicUUID uuid.UUID      `gorm:"type:uuid;not null;index"`
	Position  int16          `gorm:"not null"`
	Type      string         `gorm:"size:10;not null;check:chk_topic_fields_type,type IN ('text','para','num','date','choice','file')"`
	Label     string         `gorm:"size:120;not null"`
	Required  bool           `gorm:"not null;default:true"`
	Options   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'"` // เฉพาะ choice: ["ม.1","ม.2"]
	HiddenAt  *time.Time
	CreatedAt time.Time
}

// TopicAssignee รายชื่อผู้ต้องส่ง บันทึกตอนสร้างหัวข้อ
type TopicAssignee struct {
	UUIDModel
	TopicUUID uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_topic_assignees_topic_user,priority:1"`
	UserUUID  uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:uq_topic_assignees_topic_user,priority:2;index"`

	User User `gorm:"foreignKey:UserUUID;references:UUID;constraint:OnDelete:RESTRICT"`
}

// Submission คำตอบ 1 ชุด ต่อ 1 คน ต่อ 1 หัวข้อ
type Submission struct {
	UUIDModel
	TopicUUID   uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_submissions_topic_user,priority:1"`
	UserUUID    uuid.UUID      `gorm:"type:uuid;not null;uniqueIndex:uq_submissions_topic_user,priority:2;index"`
	Values      datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'"` // {"<field_uuid>": ค่า}
	SubmittedAt time.Time      `gorm:"not null"`                         // ส่งครั้งแรก — ใช้ตัดสิน "ช้า"
	UpdatedAt   time.Time

	Topic Topic  `gorm:"foreignKey:TopicUUID;references:UUID;constraint:OnDelete:RESTRICT"`
	User  User   `gorm:"foreignKey:UserUUID;references:UUID;constraint:OnDelete:RESTRICT"`
	Files []File `gorm:"constraint:OnDelete:SET NULL"`
}

// File ไฟล์ 1 ไฟล์ ตัว object อยู่ใน Cloudflare R2 หรือ local storage.
type File struct {
	UUIDModel
	TopicUUID      uuid.UUID  `gorm:"type:uuid;not null;index"`
	FieldUUID      uuid.UUID  `gorm:"type:uuid;not null"`
	SubmissionUUID *uuid.UUID `gorm:"type:uuid;index"`
	UploadedByUUID uuid.UUID  `gorm:"type:uuid;not null"`
	ObjectKey      string     `gorm:"size:512;not null"`
	OriginalName   string     `gorm:"size:255;not null"`
	MimeType       string     `gorm:"size:120;not null"`
	SizeBytes      int64      `gorm:"not null;check:chk_files_size,size_bytes BETWEEN 1 AND 52428800"`
	Status         string     `gorm:"size:10;not null;default:'staged';index;check:chk_files_status,status IN ('staged','attached','trashed')"`
	Position       int16      `gorm:"not null;default:0"`
	CreatedAt      time.Time

	Topic      Topic      `gorm:"foreignKey:TopicUUID;references:UUID;constraint:OnDelete:CASCADE"`
	Field      TopicField `gorm:"foreignKey:FieldUUID;references:UUID;constraint:OnDelete:CASCADE"`
	UploadedBy User       `gorm:"foreignKey:UploadedByUUID;references:UUID;constraint:OnDelete:RESTRICT"`
}

// Session การเข้าสู่ระบบ 1 ครั้ง — cookie เก็บ token, ตารางเก็บแค่ hash
type Session struct {
	UUIDModel
	TokenHash  string    `gorm:"size:64;not null;uniqueIndex"` // sha256 (hex) ของ token
	UserUUID   uuid.UUID `gorm:"type:uuid;not null;index"`
	ExpiresAt  time.Time `gorm:"not null;index"`
	LastSeenAt time.Time `gorm:"not null"`
	IP         string    `gorm:"size:45"`
	UserAgent  string    `gorm:"size:300"`
	CreatedAt  time.Time

	User User `gorm:"foreignKey:UserUUID;references:UUID;constraint:OnDelete:CASCADE"`
}
