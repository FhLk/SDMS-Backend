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

[x] Topic Domain Entity

[x] Repository Interface
    [x] Create
    [x] FindAll
    [x] FindByID
    [x] Update
    [x] Delete

[x] Domain Errors

[x] Topic Usecase
    [x] validation
    [x] create
    [x] list
    [x] detail
    [x] update
    [x] delete

[x] PostgreSQL Repository
    [x] GORM Model
    [x] Mapper
    [x] Create
    [x] FindAll
    [x] FindByID
    [x] Update
    [x] Delete

[x] HTTP Layer
    [x] Request DTO
    [x] Response DTO
    [x] Handler
    [x] Error Mapping
    [x] Routes

[x] Dependency Injection

[x] Migration

[x] Test API

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