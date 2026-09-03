# Phase 1 — Feature checklist

Every feature from [learna-features.md](learna-features.md). Tick an item only
when it is implemented **and** exercised end to end, not when the code merely
compiles.

Legend: `[x]` done · `[~]` partial · `[ ]` not started

**126 features · 50 done · 7 partial · 69 remaining**

---

## API — learna-api

### 1. Authentication & Authorization — 8/8

- [x] **A1** Email/password signup — bcrypt, format validation
- [x] **A2** Login with JWT — 15 min access + 7 day refresh, refresh stored in DB
- [x] **A3** Token refresh — with rotation
- [x] **A4** Logout — revokes refresh token, idempotent
- [x] **A5** Forgot password — single-use token, returned in body outside production
- [x] **A6** Reset password — consumes token atomically, revokes all sessions
- [x] **A7** Role-based middleware — `super_admin` / `admin` / `learner`
- [x] **A8** First-run super admin seed — idempotent, `internal/seed`

### 2. User Management (Admin) — 5/6

- [x] **U1** List users — paginated, filter by role/active, search name/email
- [x] **U2** Create user — admin assigns role and password
- [~] **U3** View user detail — profile only; enrollment history and completion
      stats need the enrollment module (E1-E4)
- [x] **U4** Update user — name, role, active; super admin only for role changes
- [x] **U5** Deactivate user — `is_active=false`, preserve data, revoke sessions
- [x] **U6** Delete user — hard delete, cascades

### 3. Profile (Self) — 3/3

- [x] **P1** View own profile
- [x] **P2** Update profile — name, avatar URL
- [x] **P3** Change password — verifies current, revokes other sessions

### 4. Course Management (Admin) — 6/6

- [x] **C1** Create course — auto-slug from title, defaults to draft
- [x] **C2** List courses (admin) — all statuses, enrollment counts, filters
- [x] **C3** Update course — slug regenerates on title change
- [x] **C4** Delete course — cascades modules, lessons, attachments, enrollments
- [x] **C5** Publish / unpublish / archive — status transitions
- [x] **C6** Course categories — free-text field

### 5. Module Management (Admin) — 0/4

- [ ] **M1** Create module
- [ ] **M2** Update module
- [ ] **M3** Delete module — cascades lessons and attachments
- [ ] **M4** Reorder modules — bulk `sort_order`, single transaction

### 6. Lesson Management (Admin) — 0/5

- [ ] **L1** Create lesson — markdown body, video URL, duration
- [ ] **L2** Update lesson
- [ ] **L3** Delete lesson — cascades attachments and progress
- [ ] **L4** Reorder lessons — bulk `sort_order`, single transaction
- [ ] **L5** Markdown content — stored raw, rendered by the UI

### 7. Attachments (Admin) — 0/4

- [ ] **AT1** Upload attachment — Cloudinary, store URL + metadata
- [ ] **AT2** List attachments for a lesson
- [ ] **AT3** Delete attachment — removes the Cloudinary asset too
- [ ] **AT4** Supported types — PDF, DOCX, PPTX, images, ZIP; 25 MB cap

### 8. Public Course Catalog — 2/2

- [x] **PC1** List published courses — paginated, filter, search, no auth
- [x] **PC2** Course detail (public) — outline only, no lesson content

### 9. Enrollment — 0/4

- [ ] **E1** Enroll in a published course
- [ ] **E2** Unenroll — removes progress for that course
- [ ] **E3** My enrollments — with progress percentage
- [ ] **E4** Enrollment check — guard before serving lesson content

### 10. Progress Tracking — 0/4

- [ ] **PR1** Mark lesson complete
- [ ] **PR2** Unmark lesson
- [ ] **PR3** Course progress percentage
- [ ] **PR4** Auto-complete course at 100%

### 11. Certificates — 0/5

- [ ] **CT1** Generate certificate on 100% completion — unique `LEARNA-YYYY-XXXX`
- [ ] **CT2** PDF generation — library not yet chosen
- [ ] **CT3** My certificates
- [ ] **CT4** Download certificate
- [ ] **CT5** Public verification by cert number

### 12. Admin Analytics — 0/2

- [ ] **AN1** Overview — users, courses by status, enrollments, completions
- [ ] **AN2** Per-course — enrollment count, completion rate, average progress

### 13. File Upload (Cloudinary) — 2/4

- [x] **CL1** Image upload — thumbnails and avatars
- [~] **CL2** File upload — `UploadService.UploadAttachment` written, no route
- [~] **CL3** File deletion — `UploadService.Delete` written, no route
- [x] **CL4** Folder organisation — `learna/{thumbnails,avatars,attachments,certificates}`

### 14. Infrastructure — 10/10

- [x] **I1** Database migrations — embedded, `golang-migrate`
- [x] **I2** Environment config — validated, `APP_ENV`-selected files
- [x] **I3** CORS middleware
- [x] **I4** Rate limiting on auth
- [x] **I5** Structured request logging with request IDs
- [x] **I6** Input validation with field-level errors
- [x] **I7** Consistent error envelope
- [x] **I8** Health check with DB connectivity
- [x] **I9** Docker — multi-stage build, compose
- [x] **I10** API versioning under `/api/v1`

---

## UI — learna-ui

### 1. Public Pages — 3/6

- [x] **UP1** Landing page
- [ ] **UP2** Course catalog — SSR, search, category filter, pagination
- [ ] **UP3** Course preview — outline accordion, enrol CTA
- [ ] **UP4** Certificate verification
- [x] **UP5** Public navbar — adapts to auth state
- [x] **UP6** Footer

### 2. Authentication Pages — 3/5

- [x] **UA1** Signup page
- [x] **UA2** Login page
- [ ] **UA3** Forgot password
- [ ] **UA4** Reset password
- [x] **UA5** Auth state management — provider, refresh, guards

### 3. Learner Dashboard — 0/2

- [ ] **LD1** My courses with progress bars and Continue
- [ ] **LD2** Empty state

### 4. Learner Course View — 0/8

- [ ] **LC1** Course layout with module/lesson sidebar
- [ ] **LC2** Lesson content — markdown rendering
- [ ] **LC3** Video embed
- [ ] **LC4** Attachment list
- [ ] **LC5** Mark complete
- [ ] **LC6** Previous / next navigation
- [ ] **LC7** Progress bar
- [ ] **LC8** Course completion + certificate CTA

### 5. Certificates (Learner) — 0/3

- [ ] **LCT1** My certificates
- [ ] **LCT2** Download
- [ ] **LCT3** Share verification link

### 6. Profile (Learner) — 0/3

- [ ] **LP1** View profile
- [ ] **LP2** Edit profile + avatar upload
- [ ] **LP3** Change password

### 7. Admin Dashboard — 0/2

- [ ] **AD1** Analytics overview cards + chart
- [ ] **AD2** Quick actions

### 8. Admin Course Management — 0/5

- [ ] **AC1** Course list table
- [ ] **AC2** Create course + thumbnail upload
- [ ] **AC3** Edit course
- [ ] **AC4** Delete with cascade warning
- [ ] **AC5** Publish / unpublish toggle

### 9. Admin Module & Lesson Editor — 0/6

- [ ] **AM1** Module accordion list
- [ ] **AM2** Add / edit module
- [ ] **AM3** Delete module
- [ ] **AM4** Lesson list per module
- [ ] **AM5** Lesson editor — markdown + attachments
- [ ] **AM6** Drag & drop reorder

### 10. Admin User Management — 0/6

- [ ] **AUM1** User list table
- [ ] **AUM2** Create user
- [ ] **AUM3** Edit user
- [ ] **AUM4** Activate / deactivate
- [ ] **AUM5** Delete with confirmation
- [ ] **AUM6** Role-aware permissions display

### 11. Admin Course Analytics — 0/1

- [ ] **ACA1** Per-course stats and enrolled-learner progress

### 12. UI Components & Patterns — 5/8

- [~] **UI1** Design system — Button, Input, Card, Badge, Progress done; Table,
      Dialog, Dropdown, Tabs, Accordion missing
- [~] **UI2** Loading states — `Skeleton`/`PageSpinner` exist, no per-surface loaders
- [~] **UI3** Empty states — `EmptyState` built but unused
- [x] **UI4** Toast notifications
- [x] **UI5** Responsive design
- [x] **UI6** Dark mode
- [x] **UI7** 404 page
- [x] **UI8** Error boundary

### 13. UI Infrastructure — 3/4

- [x] **UIF1** API client — interceptors, single-flight refresh
- [x] **UIF2** Environment config
- [~] **UIF3** SEO — root metadata only, no per-course Open Graph
- [x] **UIF4** Docker / standalone build

---

## Build order

Each layer depends on the one above it, so they are built in this sequence:

1. **Users** (U1–U6) — repository already exists; service and handler only
2. **Courses** (C1–C6, PC1–PC2) — the spine everything else hangs off
3. **Modules** (M1–M4)
4. **Lessons** (L1–L5) — plus CL2/CL3 wiring for attachments
5. **Attachments** (AT1–AT4)
6. **Enrollment** (E1–E4) — needs courses
7. **Progress** (PR1–PR4) — needs lessons and enrollment
8. **Certificates** (CT1–CT5) — needs progress at 100%
9. **Analytics** (AN1–AN2) — needs enrollment and progress
10. **UI** — each page once its endpoints exist
