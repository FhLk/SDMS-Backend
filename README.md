SDMS API

REST API สำหรับระบบ School Document Management System (SDMS) พัฒนาด้วย Go โดยใช้ Fiber v3, GORM และ PostgreSQL และจัดโครงสร้างโปรเจกต์ตามแนวคิด Clean Architecture

Tech Stack

Go 1.25+

Fiber v3

GORM

PostgreSQL

Docker / Docker Compose

godotenv

Project Structure

sdms-api/
├── cmd/
│   └── api/
│       └── main.go
│
├── internal/
│   ├── config/
│   │   └── config.go
│   │
│   ├── modules/
│   │   └── health/
│   │       └── delivery/
│   │           └── http/
│   │               └── handler.go
│   │
│   └── platform/
│       ├── database/
│       │   └── postgres.go
│       │
│       └── http/
│           └── router.go
│
├── .env
├── .env.example
├── .gitignore
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md

Architecture

โปรเจกต์ใช้แนวคิด Clean Architecture เพื่อแยก Business Logic ออกจาก Framework และ Infrastructure

HTTP / Fiber
     │
     ▼
Delivery
     │
     ▼
Usecase
     │
     ▼
Domain
     ▲
     │
Repository Interface
     ▲
     │
Repository Implementation
     │
     ▼
GORM / PostgreSQL

หลักการสำคัญ:

Domain ไม่ควร import Fiber

Domain ไม่ควร import GORM

Handler ไม่ควร query database โดยตรง

Business Logic ควรอยู่ใน Usecase

Repository interface ควรประกาศใน Domain

GORM implementation ควรอยู่ใน Repository layer

Requirements

ก่อนเริ่มใช้งานควรติดตั้ง:

Go

ตรวจสอบเวอร์ชัน:

go version

ควรเป็น Go 1.25 หรือใหม่กว่า

Docker

ตรวจสอบ:

docker --version
docker compose version

Getting Started

1. Clone หรือเข้า Project

cd sdms-api

2. Install Dependencies

go mod tidy

หรือถ้ายังไม่ได้ติดตั้ง dependency:

go get github.com/gofiber/fiber/v3
go get gorm.io/gorm
go get gorm.io/driver/postgres
go get github.com/joho/godotenv

Environment Variables

สร้าง .env จาก .env.example

cp .env.example .env

ตัวอย่าง .env

APP_ENV=development
APP_PORT=8080

DB_HOST=localhost
DB_PORT=5432
DB_USER=sdms
DB_PASSWORD=sdms_password
DB_NAME=sdms_db
DB_SSLMODE=disable
DB_TIMEZONE=Asia/Bangkok

ห้าม commit .env ที่มี password หรือ secret จริงขึ้น Git repository

PostgreSQL

โปรเจกต์ใช้ PostgreSQL ผ่าน Docker Compose

ตัวอย่าง docker-compose.yml

services:
  postgres:
    image: postgres:17
    container_name: sdms-postgres
    restart: unless-stopped

    environment:
      POSTGRES_USER: sdms
      POSTGRES_PASSWORD: sdms_password
      POSTGRES_DB: sdms_db

    ports:
      - "5432:5432"

    volumes:
      - sdms_postgres_data:/var/lib/postgresql/data

volumes:
  sdms_postgres_data:

Start PostgreSQL

docker compose up -d

ตรวจสอบสถานะ:

docker compose ps

ดู logs:

docker compose logs postgres

หยุด services:

docker compose down

หยุดและลบ database volume:

docker compose down -v

คำสั่ง docker compose down -v จะลบข้อมูล PostgreSQL ใน volume ด้วย

Run Application

รัน API:

go run ./cmd/api

เมื่อสำเร็จควรเห็น log คล้าย:

database connected
server running on http://localhost:8080

Build

ตรวจสอบว่า project compile ผ่าน:

go build ./...

Format source code:

go fmt ./...

API

Base URL:

http://localhost:8080/api/v1

Health Check

GET /api/v1/health

ทดสอบด้วย curl:

curl http://localhost:8080/api/v1/health

ตัวอย่าง response:

{
  "status": "ok",
  "database": "connected",
  "service": "sdms-api"
}

Planned Modules

ระบบจะค่อย ๆ แบ่งออกเป็น module ดังนี้:

internal/modules/
├── health/
├── auth/
├── user/
├── topic/
├── form_schema/
├── submission/
└── file/

Topic

ใช้สำหรับ Folder หรือหัวข้อเอกสารที่ ผอ. สามารถสร้าง แก้ไข และลบได้

ตัวอย่าง API ที่จะพัฒนา:

POST   /api/v1/topics
GET    /api/v1/topics
GET    /api/v1/topics/:id
PUT    /api/v1/topics/:id
DELETE /api/v1/topics/:id

Dynamic Form

แต่ละ Topic สามารถมี Form Schema ของตัวเอง เช่น:

Topic
└── Form Schema
    ├── Text Field
    ├── Number Field
    ├── Date Field
    ├── Select Field
    └── File Field

แนะนำให้ใช้ Form Schema Version เพื่อให้ Submission เก่ายังคงอ้างอิงโครงสร้าง Form เดิมได้ แม้ ผอ. จะเปลี่ยน Field ภายหลัง

ตัวอย่าง:

Topic
├── Form Schema v1
└── Form Schema v2

Submission

ครูจะกรอก Dynamic Form และส่งข้อมูลเข้า Topic

แนวคิดโครงสร้างข้อมูล:

Topic
  │
  ▼
Form Schema
  │
  ▼
Form Fields

Form Schema
  │
  ▼
Submission
  ├── Submission Values
  └── Submission Files

Suggested Development Order

แนะนำให้พัฒนาตามลำดับ:

Infrastructure

Fiber

Config

PostgreSQL

GORM

Topic CRUD

Domain

Repository

Usecase

Handler

User และ Role

Director

Teacher

Authentication

Login

JWT

Refresh Token

Dynamic Form

Form Schema

Form Field

Schema Version

Submission

Dynamic values

Validation

File Upload

S3 หรือ MinIO

File metadata ใน PostgreSQL

Audit Log

Git Ignore

ตัวอย่าง .gitignore

.env

bin/
tmp/
dist/

*.log

.DS_Store

.idea/
.vscode/

Development Commands

# Start PostgreSQL
docker compose up -d

# Stop PostgreSQL
docker compose down

# Install / clean dependencies
go mod tidy

# Format code
go fmt ./...

# Build
go build ./...

# Run API
go run ./cmd/api

Current Status

Go project setup

Fiber v3

Environment configuration

PostgreSQL with Docker Compose

GORM connection

Health Check API

Topic CRUD

User / Role

Authentication

Dynamic Form

Submission

File Upload

Audit Log

License

Internal project for School Document Management System.