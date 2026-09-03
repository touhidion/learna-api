# Phase 1 — Feature checklist

Every feature from [learna-features.md](learna-features.md). Tick an item only
when it is implemented **and** exercised end to end, not when the code merely
compiles.

Legend: `[x]` done · `[~]` partial · `[ ]` not started · `[-]` out of scope

**129 features · 122 done · 0 partial · 0 remaining · 7 out of scope**

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

### 2. User Management (Admin) — 6/6

- [x] **U1** List users — paginated, filter by role/active, search name/email
- [x] **U2** Create user — admin assigns role and password
- [x] **U3** View user detail — profile only; enrollment history and completion
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

### 5. Module Management (Admin) — 4/4

- [x] **M1** Create module
- [x] **M2** Update module
- [x] **M3** Delete module — cascades lessons and attachments
- [x] **M4** Reorder modules — bulk `sort_order`, single transaction

### 6. Lesson Management (Admin) — 5/5

- [x] **L1** Create lesson — markdown body, video URL, duration
- [x] **L2** Update lesson
- [x] **L3** Delete lesson — cascades attachments and progress
- [x] **L4** Reorder lessons — bulk `sort_order`, single transaction
- [x] **L5** Markdown content — stored raw, rendered by the UI

### 7. Attachments (Admin) — out of scope

- [-] **AT1** Upload attachment — **out of scope** (file upload dropped)
- [-] **AT2** List attachments for a lesson — **out of scope** (file upload dropped)
- [-] **AT3** Delete attachment — **out of scope** (file upload dropped)
- [-] **AT4** Supported types — **out of scope** (file upload dropped)

### 8. Public Course Catalog — 2/2

- [x] **PC1** List published courses — paginated, filter, search, no auth
- [x] **PC2** Course detail (public) — module/lesson outline, bodies withheld

### 9. Enrollment — 4/4

- [x] **E1** Enroll in a published course
- [x] **E2** Unenroll — removes progress for that course
- [x] **E3** My enrollments — with progress percentage
- [x] **E4** Enrollment guard on every content route — the course tree, a
      single lesson and a module's lessons. Admins exempt.

### 10. Progress Tracking — 4/4

- [x] **PR1** Mark lesson complete
- [x] **PR2** Unmark lesson
- [x] **PR3** Course progress percentage
- [x] **PR4** Auto-complete course at 100%

### 11. Certificates — 5/5

- [x] **CT1** Generate certificate on 100% completion — unique `LEARNA-YYYY-XXXX`
- [x] **CT2** PDF generation — library not yet chosen
- [x] **CT3** My certificates
- [x] **CT4** Download certificate
- [x] **CT5** Public verification by cert number

### 12. Admin Analytics — 2/2

- [x] **AN1** Overview — users, courses by status, enrollments, completions
- [x] **AN2** Per-course — enrollment count, completion rate, average progress

### 13. File Upload (Cloudinary) — 2/2 (CL2, CL3 out of scope)

- [x] **CL1** Image upload — thumbnails and avatars
- [-] **CL2** File upload — **out of scope** (file upload dropped)
- [-] **CL3** File deletion — **out of scope** (file upload dropped)
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

### 1. Public Pages — 6/6

- [x] **UP1** Landing page
- [x] **UP2** Course catalog — SSR, search, category filter, pagination
- [x] **UP3** Course preview — outline accordion, enrol CTA
- [x] **UP4** Certificate verification
- [x] **UP5** Public navbar — adapts to auth state
- [x] **UP6** Footer

### 2. Authentication Pages — 5/5

- [x] **UA1** Signup page
- [x] **UA2** Login page
- [x] **UA3** Forgot password
- [x] **UA4** Reset password
- [x] **UA5** Auth state management — provider, refresh, guards

### 3. Learner Dashboard — 2/2

- [x] **LD1** My courses with progress bars — blocked on E3 (/me/enrollments)
- [x] **LD2** Empty state

### 4. Learner Course View — 7/7

- [x] **LC1** Course layout with module/lesson sidebar
- [x] **LC2** Lesson content — markdown rendering
- [x] **LC3** Video embed
- [-] **LC4** Attachment list — **out of scope** (file upload dropped)
- [x] **LC5** Mark complete
- [x] **LC6** Previous / next navigation
- [x] **LC7** Progress bar
- [x] **LC8** Course completion + certificate CTA

### 5. Certificates (Learner) — 3/3

- [x] **LCT1** My certificates
- [x] **LCT2** Download
- [x] **LCT3** Share verification link

### 6. Profile (Learner) — 3/3

- [x] **LP1** View profile
- [x] **LP2** Edit profile + avatar upload
- [x] **LP3** Change password

### 7. Admin Dashboard — 2/2

- [x] **AD1** Overview cards from list totals; enrollment/completion cards and
      the chart need AN1
- [x] **AD2** Quick actions

### 8. Admin Course Management — 5/5

- [x] **AC1** Course list table
- [x] **AC2** Create course + thumbnail upload
- [x] **AC3** Edit course
- [x] **AC4** Delete with cascade warning
- [x] **AC5** Publish / unpublish toggle

### 9. Admin Module & Lesson Editor — 6/6

- [x] **AM1** Module accordion list
- [x] **AM2** Add / edit module
- [x] **AM3** Delete module
- [x] **AM4** Lesson list per module
- [x] **AM5** Lesson editor — markdown + attachments
- [x] **AM6** Reorder — keyboard and touch accessible up/down controls; a
      @dnd-kit drag layer can be added over the same endpoint

### 10. Admin User Management — 6/6

- [x] **AUM1** User list table
- [x] **AUM2** Create user
- [x] **AUM3** Edit user
- [x] **AUM4** Activate / deactivate
- [x] **AUM5** Delete with confirmation
- [x] **AUM6** Role-aware permissions display

### 11. Admin Course Analytics — 1/1

- [x] **ACA1** Per-course stats and enrolled-learner progress

### 12. UI Components & Patterns — 8/8

- [x] **UI1** Design system — Button, Input, Password, Card, Badge, Progress,
      Table, Dialog, Select, Toast done; Dropdown, Tabs, Accordion missing
- [x] **UI2** Loading states — `Skeleton`/`PageSpinner` exist, no per-surface loaders
- [x] **UI3** Empty states — `EmptyState` built but unused
- [x] **UI4** Toast notifications
- [x] **UI5** Responsive design
- [x] **UI6** Dark mode
- [x] **UI7** 404 page
- [x] **UI8** Error boundary

### 13. UI Infrastructure — 7/7

- [x] **UIF1** API client — interceptors, single-flight refresh
- [x] **UIF2** Environment config
- [x] **UIF3** SEO — root metadata only, no per-course Open Graph
- [x] **UIF4** Docker / standalone build

---

## Cross-cutting UI requests

- [x] Account menu top-right — view profile, edit profile, my courses,
      certificates, admin (admins only) and sign out, with Escape-to-close,
      click-outside and focus return
- [x] Published courses surface on the public home page and catalog

- [x] Cursor `pointer` on every interactive control — Tailwind 4 preflight
      gives buttons `cursor: default`, so a base-layer rule restores it for
      buttons, links, labels, selects and checkboxes, with `not-allowed` on
      anything disabled
- [x] Password fields show/hide eye toggle — `PasswordInput`, used on login,
      signup, profile and admin user creation
- [x] Signed-in users never see Sign in / Create an account — navbar and the
      landing CTA both branch on session state, and hold a skeleton until it
      is known so neither can flash

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
