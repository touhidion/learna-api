# Learna — Architecture Document

## Overview

Learna is a free, self-hosted training portal where admins create structured courses and learners consume them at their own pace. The system is split into two independent repositories:

- **learna-api** — Go (Gin) REST API with PostgreSQL
- **learna-ui** — Next.js 14 (App Router) frontend

---

## High-Level Architecture

```
┌─────────────────────────────────────────────────────────┐
│                      CLIENTS                            │
│              Browser / Mobile Browser                   │
└──────────────────────┬──────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────┐
│                   learna-ui                             │
│               Next.js 14 (App Router)                   │
│                                                         │
│  ┌─────────────┐  ┌──────────────┐  ┌───────────────┐  │
│  │ Public Pages │  │ Learner      │  │ Admin         │  │
│  │ (SSR/SSG)   │  │ Dashboard    │  │ Dashboard     │  │
│  │ - Landing   │  │ - My Courses │  │ - Course CRUD │  │
│  │ - Catalog   │  │ - Progress   │  │ - User Mgmt   │  │
│  │ - Course    │  │ - Profile    │  │ - Analytics   │  │
│  │   Preview   │  │ - Certs      │  │ - Settings    │  │
│  └─────────────┘  └──────────────┘  └───────────────┘  │
└──────────────────────┬──────────────────────────────────┘
                       │ HTTPS (REST)
                       ▼
┌─────────────────────────────────────────────────────────┐
│                   learna-api                            │
│                 Go + Gin Framework                      │
│                                                         │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────────┐ │
│  │ Auth     │  │ Course    │  │ Middleware            │ │
│  │ Module   │  │ Module    │  │ - JWT Auth            │ │
│  │ - Login  │  │ - Courses │  │ - Role Guard          │ │
│  │ - Signup │  │ - Modules │  │ - CORS                │ │
│  │ - JWT    │  │ - Lessons │  │ - Rate Limiter        │ │
│  │ - Roles  │  │ - Attach  │  │ - Request Logger      │ │
│  └──────────┘  └───────────┘  └──────────────────────┘ │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────────┐ │
│  │ User     │  │ Enroll &  │  │ Certificate          │ │
│  │ Module   │  │ Progress  │  │ Module               │ │
│  │ - CRUD   │  │ Module    │  │ - Generate PDF       │ │
│  │ - Roles  │  │ - Enroll  │  │ - Download           │ │
│  │ - Profile│  │ - Track   │  │ - Verify             │ │
│  └──────────┘  └───────────┘  └──────────────────────┘ │
└──────────────────────┬──────────────────────────────────┘
                       │
            ┌──────────┴──────────┐
            ▼                     ▼
┌──────────────────┐   ┌──────────────────┐
│   PostgreSQL     │   │   Cloudinary     │
│                  │   │                  │
│  - Users         │   │  - Thumbnails    │
│  - Courses       │   │  - Attachments   │
│  - Modules       │   │  - Cert logos    │
│  - Lessons       │   │                  │
│  - Enrollments   │   │                  │
│  - Progress      │   │                  │
│  - Certificates  │   │                  │
└──────────────────┘   └──────────────────┘
```

---

## Database Schema (PostgreSQL)

```
┌──────────────────────┐
│       users          │
├──────────────────────┤
│ id           UUID PK │
│ email        TEXT UQ  │
│ password     TEXT     │ ← bcrypt hash
│ name         TEXT     │
│ avatar_url   TEXT     │
│ role         ENUM     │ ← super_admin | admin | learner
│ is_active    BOOL     │
│ created_at   TIMESTZ  │
│ updated_at   TIMESTZ  │
└──────────┬───────────┘
           │
           │ 1:N (created_by)
           ▼
┌──────────────────────┐
│      courses         │
├──────────────────────┤
│ id           UUID PK │
│ title        TEXT     │
│ slug         TEXT UQ  │
│ description  TEXT     │
│ thumbnail_url TEXT    │ ← Cloudinary URL
│ category     TEXT     │
│ status       ENUM     │ ← draft | published | archived
│ created_by   UUID FK  │ → users.id
│ created_at   TIMESTZ  │
│ updated_at   TIMESTZ  │
└──────────┬───────────┘
           │
           │ 1:N
           ▼
┌──────────────────────┐
│      modules         │
├──────────────────────┤
│ id           UUID PK │
│ course_id    UUID FK  │ → courses.id (CASCADE)
│ title        TEXT     │
│ description  TEXT     │
│ sort_order   INT      │
│ created_at   TIMESTZ  │
│ updated_at   TIMESTZ  │
└──────────┬───────────┘
           │
           │ 1:N
           ▼
┌──────────────────────┐       ┌──────────────────────┐
│      lessons         │       │    attachments       │
├──────────────────────┤       ├──────────────────────┤
│ id           UUID PK │──1:N─▶│ id           UUID PK │
│ module_id    UUID FK  │       │ lesson_id    UUID FK  │ → lessons.id (CASCADE)
│ title        TEXT     │       │ file_name    TEXT     │
│ content      TEXT     │ ← MD  │ file_url     TEXT     │ ← Cloudinary URL
│ video_url    TEXT     │       │ file_type    TEXT     │
│ duration_min INT      │       │ file_size    BIGINT   │
│ sort_order   INT      │       │ created_at   TIMESTZ  │
│ created_at   TIMESTZ  │       └──────────────────────┘
│ updated_at   TIMESTZ  │
└──────────────────────┘

┌──────────────────────┐       ┌──────────────────────┐
│    enrollments       │       │  lesson_progress     │
├──────────────────────┤       ├──────────────────────┤
│ id           UUID PK │       │ id           UUID PK │
│ user_id      UUID FK  │       │ user_id      UUID FK  │
│ course_id    UUID FK  │       │ lesson_id    UUID FK  │
│ enrolled_at  TIMESTZ  │       │ completed    BOOL     │
│ completed_at TIMESTZ  │       │ completed_at TIMESTZ  │
│ UNIQUE(user, course) │       │ UNIQUE(user, lesson) │
└──────────────────────┘       └──────────────────────┘

┌──────────────────────┐
│   certificates       │
├──────────────────────┤
│ id           UUID PK │
│ user_id      UUID FK  │
│ course_id    UUID FK  │
│ cert_number  TEXT UQ  │ ← e.g. LEARNA-2026-XXXX
│ issued_at    TIMESTZ  │
│ pdf_url      TEXT     │ ← Cloudinary URL
│ UNIQUE(user, course) │
└──────────────────────┘
```

---

## API Architecture (learna-api)

### Project Structure

```
learna-api/
├── cmd/
│   └── server/
│       └── main.go              # entry point
├── internal/
│   ├── config/
│   │   └── config.go            # env vars, DB, Cloudinary config
│   ├── database/
│   │   ├── postgres.go          # connection pool
│   │   └── migrations/          # SQL migration files
│   ├── middleware/
│   │   ├── auth.go              # JWT extraction & validation
│   │   ├── role.go              # role-based access guard
│   │   ├── cors.go
│   │   ├── ratelimit.go
│   │   └── logger.go
│   ├── models/
│   │   ├── user.go
│   │   ├── course.go
│   │   ├── module.go
│   │   ├── lesson.go
│   │   ├── attachment.go
│   │   ├── enrollment.go
│   │   ├── progress.go
│   │   └── certificate.go
│   ├── handlers/
│   │   ├── auth_handler.go
│   │   ├── user_handler.go
│   │   ├── course_handler.go
│   │   ├── module_handler.go
│   │   ├── lesson_handler.go
│   │   ├── attachment_handler.go
│   │   ├── enrollment_handler.go
│   │   ├── progress_handler.go
│   │   └── certificate_handler.go
│   ├── services/
│   │   ├── auth_service.go
│   │   ├── user_service.go
│   │   ├── course_service.go
│   │   ├── enrollment_service.go
│   │   ├── progress_service.go
│   │   ├── certificate_service.go
│   │   └── cloudinary_service.go
│   ├── repository/
│   │   ├── user_repo.go
│   │   ├── course_repo.go
│   │   ├── module_repo.go
│   │   ├── lesson_repo.go
│   │   ├── enrollment_repo.go
│   │   ├── progress_repo.go
│   │   └── certificate_repo.go
│   ├── dto/
│   │   ├── request/              # request payloads
│   │   └── response/             # response shapes
│   └── utils/
│       ├── jwt.go
│       ├── password.go
│       ├── slug.go
│       ├── pagination.go
│       └── validator.go
├── pkg/
│   └── cloudinary/
│       └── client.go             # Cloudinary SDK wrapper
├── .env.example
├── Makefile
├── Dockerfile
├── docker-compose.yml            # API + Postgres + optional services
├── go.mod
└── go.sum
```

### Key Design Decisions

- **Handler → Service → Repository** pattern (3-layer). Handlers parse HTTP, services hold logic, repos do SQL.
- **Database migrations** via `golang-migrate` — run on startup or via CLI.
- **JWT** with access token (15 min) + refresh token (7 days). Refresh token stored in DB for revocation.
- **Password hashing** with bcrypt (cost 12).
- **Cloudinary** SDK for Go — uploads happen in the API, returning the CDN URL to store in Postgres.
- **Certificate PDF** generated server-side using `go-pdf` or `gofpdf`, then uploaded to Cloudinary. A unique cert number enables public verification via a `/verify/:certNumber` endpoint.
- **Pagination** — cursor-based for listings, offset-based for admin tables.
- **Soft delete** not used in Phase 1 — hard delete with CASCADE on course/module/lesson relationships.

### API Endpoints

```
Auth
  POST   /api/v1/auth/signup
  POST   /api/v1/auth/login
  POST   /api/v1/auth/refresh
  POST   /api/v1/auth/logout
  POST   /api/v1/auth/forgot-password
  POST   /api/v1/auth/reset-password

Users (admin)
  GET    /api/v1/admin/users              # list all users
  POST   /api/v1/admin/users              # create admin/learner
  GET    /api/v1/admin/users/:id
  PATCH  /api/v1/admin/users/:id          # update role, activate/deactivate
  DELETE /api/v1/admin/users/:id

Profile (self)
  GET    /api/v1/me
  PATCH  /api/v1/me
  PATCH  /api/v1/me/password

Courses
  GET    /api/v1/courses                  # public catalog (published only)
  GET    /api/v1/courses/:slug            # public detail
  POST   /api/v1/admin/courses            # admin create
  PATCH  /api/v1/admin/courses/:id        # admin update
  DELETE /api/v1/admin/courses/:id
  PATCH  /api/v1/admin/courses/:id/status # publish / archive

Modules
  GET    /api/v1/courses/:courseId/modules
  POST   /api/v1/admin/courses/:courseId/modules
  PATCH  /api/v1/admin/modules/:id
  DELETE /api/v1/admin/modules/:id
  PATCH  /api/v1/admin/modules/reorder    # bulk sort_order update

Lessons
  GET    /api/v1/modules/:moduleId/lessons
  GET    /api/v1/lessons/:id              # full content
  POST   /api/v1/admin/modules/:moduleId/lessons
  PATCH  /api/v1/admin/lessons/:id
  DELETE /api/v1/admin/lessons/:id
  PATCH  /api/v1/admin/lessons/reorder

Attachments
  POST   /api/v1/admin/lessons/:lessonId/attachments   # upload to Cloudinary
  DELETE /api/v1/admin/attachments/:id

Enrollment
  POST   /api/v1/courses/:courseId/enroll
  DELETE /api/v1/courses/:courseId/unenroll
  GET    /api/v1/me/enrollments           # my enrolled courses

Progress
  POST   /api/v1/lessons/:lessonId/complete
  DELETE /api/v1/lessons/:lessonId/uncomplete
  GET    /api/v1/courses/:courseId/progress   # % complete for current user

Certificates
  GET    /api/v1/me/certificates
  POST   /api/v1/courses/:courseId/certificate   # generate on 100% completion
  GET    /api/v1/certificates/verify/:certNumber # public verification

Admin Analytics
  GET    /api/v1/admin/analytics/overview        # total users, courses, enrollments
  GET    /api/v1/admin/analytics/courses/:id     # enrollment count, completion rate

Upload
  POST   /api/v1/admin/upload/image              # generic Cloudinary image upload
```

---

## UI Architecture (learna-ui)

### Project Structure

```
learna-ui/
├── src/
│   ├── app/
│   │   ├── (public)/                    # public layout group
│   │   │   ├── page.tsx                 # landing page
│   │   │   ├── courses/
│   │   │   │   ├── page.tsx             # course catalog
│   │   │   │   └── [slug]/
│   │   │   │       └── page.tsx         # course preview (public)
│   │   │   ├── verify/
│   │   │   │   └── [certNumber]/
│   │   │   │       └── page.tsx         # public cert verification
│   │   │   └── layout.tsx               # public navbar + footer
│   │   ├── (auth)/
│   │   │   ├── login/page.tsx
│   │   │   ├── signup/page.tsx
│   │   │   ├── forgot-password/page.tsx
│   │   │   └── reset-password/page.tsx
│   │   ├── (learner)/                   # authenticated learner area
│   │   │   ├── dashboard/page.tsx       # my courses, progress overview
│   │   │   ├── courses/
│   │   │   │   └── [slug]/
│   │   │   │       ├── page.tsx         # course learning view
│   │   │   │       └── lessons/
│   │   │   │           └── [lessonId]/
│   │   │   │               └── page.tsx # lesson content viewer
│   │   │   ├── certificates/page.tsx    # my certificates
│   │   │   ├── profile/page.tsx
│   │   │   └── layout.tsx               # learner sidebar + header
│   │   ├── (admin)/
│   │   │   ├── admin/
│   │   │   │   ├── dashboard/page.tsx   # analytics overview
│   │   │   │   ├── courses/
│   │   │   │   │   ├── page.tsx         # course list
│   │   │   │   │   ├── new/page.tsx     # create course
│   │   │   │   │   └── [id]/
│   │   │   │   │       ├── page.tsx     # edit course
│   │   │   │   │       ├── modules/page.tsx    # manage modules & lessons
│   │   │   │   │       └── analytics/page.tsx  # course stats
│   │   │   │   ├── users/
│   │   │   │   │   ├── page.tsx         # user list
│   │   │   │   │   └── new/page.tsx     # create user
│   │   │   │   └── settings/page.tsx    # site settings
│   │   │   └── layout.tsx               # admin sidebar + header
│   │   ├── layout.tsx                   # root layout
│   │   └── globals.css
│   ├── components/
│   │   ├── ui/                          # shadcn/ui primitives
│   │   ├── common/
│   │   │   ├── Navbar.tsx
│   │   │   ├── Footer.tsx
│   │   │   ├── SearchBar.tsx
│   │   │   └── Pagination.tsx
│   │   ├── course/
│   │   │   ├── CourseCard.tsx
│   │   │   ├── CourseGrid.tsx
│   │   │   ├── ModuleAccordion.tsx
│   │   │   ├── LessonSidebar.tsx
│   │   │   ├── LessonContent.tsx        # markdown renderer + video embed
│   │   │   ├── ProgressBar.tsx
│   │   │   └── VideoPlayer.tsx          # YouTube/Vimeo embed wrapper
│   │   ├── admin/
│   │   │   ├── CourseForm.tsx
│   │   │   ├── ModuleEditor.tsx         # drag & drop reorder
│   │   │   ├── LessonEditor.tsx         # markdown editor
│   │   │   ├── FileUploader.tsx         # Cloudinary upload via API
│   │   │   ├── UserTable.tsx
│   │   │   └── StatsCard.tsx
│   │   └── auth/
│   │       ├── LoginForm.tsx
│   │       └── SignupForm.tsx
│   ├── lib/
│   │   ├── api.ts                       # axios/fetch wrapper, base URL, interceptors
│   │   ├── auth.ts                      # token storage, refresh logic
│   │   └── utils.ts                     # formatters, helpers
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useCourses.ts
│   │   ├── useProgress.ts
│   │   └── useDebounce.ts
│   ├── types/
│   │   ├── user.ts
│   │   ├── course.ts
│   │   ├── lesson.ts
│   │   └── api.ts
│   └── providers/
│       ├── AuthProvider.tsx             # React context for auth state
│       └── ThemeProvider.tsx
├── public/
│   └── images/
├── .env.local.example
├── next.config.ts
├── tailwind.config.ts
├── package.json
├── Dockerfile
└── tsconfig.json
```

### Key Design Decisions

- **App Router** with route groups `(public)`, `(auth)`, `(learner)`, `(admin)` for clean layout separation.
- **Tailwind CSS + shadcn/ui** — utility-first styling with accessible component primitives.
- **SSR** for public pages (catalog, course preview) for SEO. **CSR** for authenticated dashboards.
- **Auth** — JWT stored in httpOnly cookie (set by API) or localStorage with refresh logic. AuthProvider wraps protected routes.
- **Markdown rendering** — `react-markdown` with `remark-gfm` for lesson content.
- **Video embeds** — `react-player` or a custom lite-youtube component for performance.
- **Drag & drop** — `@dnd-kit/core` for module/lesson reordering in admin.
- **Rich text editing** — admin uses a markdown editor (e.g., `@uiw/react-md-editor`) with live preview.
- **File uploads** — admin uploads go through the API (which proxies to Cloudinary), not direct client-to-Cloudinary.
- **State management** — React Context for auth, SWR or TanStack Query for server state / caching.

---

## Deployment Architecture

```
┌────────────────────────────────────────────────┐
│              Production Setup                  │
│                                                │
│  ┌──────────────┐    ┌──────────────────────┐  │
│  │  Nginx /     │    │   learna-ui          │  │
│  │  Reverse     │───▶│   (Next.js)          │  │
│  │  Proxy       │    │   Port 3000          │  │
│  │              │    └──────────────────────┘  │
│  │  SSL (Let's  │                              │
│  │  Encrypt)    │    ┌──────────────────────┐  │
│  │              │───▶│   learna-api          │  │
│  │  learna.com  │    │   (Go binary)        │  │
│  │    /api/*    │    │   Port 8080          │  │
│  └──────────────┘    └──────────┬───────────┘  │
│                                 │              │
│                      ┌──────────▼───────────┐  │
│                      │   PostgreSQL 16      │  │
│                      │   Port 5432          │  │
│                      └──────────────────────┘  │
└────────────────────────────────────────────────┘
                       │
                       ▼
              ┌──────────────────┐
              │   Cloudinary     │
              │   (External CDN) │
              └──────────────────┘
```

- Both services containerized with Docker, orchestrated via `docker-compose` for dev and single-server prod.
- Nginx handles SSL termination, routes `/api/*` to Go, everything else to Next.js.
- PostgreSQL runs as a managed service (or Docker container for small deployments).
- Cloudinary is external SaaS — no self-hosted file storage needed.

---

## Phase Plan

| Phase | Scope |
|-------|-------|
| **Phase 1** | Auth, course/module/lesson CRUD, Cloudinary uploads, enrollment, progress tracking, certificates, admin user management, public catalog, learner dashboard |
| **Phase 2** | Quizzes & assessments (per module or per course), quiz results, score-based certificates, learner leaderboard |
| **Phase 3** | Email notifications (welcome, course completion), discussion/comments on lessons, bookmarks, course ratings, full-text search |
