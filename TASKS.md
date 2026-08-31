Phase 1 ✅
Infrastructure
├── Fiber
├── Config
├── PostgreSQL
└── GORM

        ↓

Phase 2
Topic
├── Domain
├── Repository Interface
├── Usecase
├── GORM Repository
└── HTTP Handler

        ↓

Phase 3
User + Role
├── Director
└── Teacher

        ↓

Phase 4
Dynamic Form
Topic
└── FormSchema
     └── FormField

        ↓

Phase 5
Submission
Teacher
  ↓
Dynamic Form
  ↓
Submission
  ├── Values
  └── Files

        ↓

Phase 6
Auth
JWT / Refresh Token

        ↓

Phase 7
File Storage
S3 / MinIO