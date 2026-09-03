# Learna — Feature List

## Phase 1 Features

---

## API Service (learna-api)

### 1. Authentication & Authorization

| # | Feature | Details |
|---|---------|---------|
| A1 | Email/password signup | Learner self-registration with email validation format check, bcrypt password hashing |
| A2 | Login with JWT | Returns access token (15 min) + refresh token (7 days), refresh token stored in DB for revocation |
| A3 | Token refresh | `POST /auth/refresh` — issue new access token using valid refresh token |
| A4 | Logout | Invalidate refresh token in DB |
| A5 | Forgot password | Generate time-limited reset token, return reset link (email sending is Phase 3, for now return token in response) |
| A6 | Reset password | Validate reset token, update password hash |
| A7 | Role-based middleware | Three roles: `super_admin`, `admin`, `learner`. Middleware guards admin routes. Super admin can do everything admin can, plus manage other admins |
| A8 | First-run seed | On first startup, if no super_admin exists, create one from env vars (`SUPER_ADMIN_EMAIL`, `SUPER_ADMIN_PASSWORD`) |

### 2. User Management (Admin)

| # | Feature | Details |
|---|---------|---------|
| U1 | List users | Paginated list with filters: role, active status, search by name/email |
| U2 | Create user | Admin creates a new user with role assignment (admin or learner). Password set by admin, user can change later |
| U3 | View user detail | User profile + enrollment history + completion stats |
| U4 | Update user | Change name, role, active status. Super admin can promote/demote admins |
| U5 | Deactivate user | Set `is_active = false` — deactivated users can't log in. Preserve their data |
| U6 | Delete user | Hard delete with cascade (enrollments, progress, certificates removed) |

### 3. Profile (Self)

| # | Feature | Details |
|---|---------|---------|
| P1 | View own profile | `GET /me` — name, email, role, avatar, join date |
| P2 | Update profile | Change name, upload avatar (via Cloudinary) |
| P3 | Change password | Requires current password + new password |

### 4. Course Management (Admin)

| # | Feature | Details |
|---|---------|---------|
| C1 | Create course | Title, description, category, thumbnail (Cloudinary upload). Auto-generate slug from title. Status defaults to `draft` |
| C2 | List courses (admin) | All courses regardless of status, with enrollment counts. Filterable by status, category |
| C3 | Update course | Edit title, description, category, thumbnail. Slug regenerates if title changes (with redirect handling) |
| C4 | Delete course | Cascade deletes modules, lessons, attachments, enrollments, progress |
| C5 | Publish / Unpublish / Archive | Status transitions: `draft → published`, `published → archived`, `archived → draft`. Only published courses appear in public catalog |
| C6 | Course categories | Free-text category field in Phase 1. Predefined category table is Phase 3 |

### 5. Module Management (Admin)

| # | Feature | Details |
|---|---------|---------|
| M1 | Create module | Title, description, sort_order within a course |
| M2 | Update module | Edit title, description |
| M3 | Delete module | Cascade deletes child lessons and attachments |
| M4 | Reorder modules | Bulk update `sort_order` for all modules in a course via single endpoint |

### 6. Lesson Management (Admin)

| # | Feature | Details |
|---|---------|---------|
| L1 | Create lesson | Title, markdown content body, optional video URL (YouTube/Vimeo/any embed), optional estimated duration, sort_order within a module |
| L2 | Update lesson | Edit all fields |
| L3 | Delete lesson | Cascade deletes attachments and progress records |
| L4 | Reorder lessons | Bulk update `sort_order` for all lessons in a module |
| L5 | Markdown content | Content stored as raw markdown in DB. Rendering happens on frontend |

### 7. Attachments (Admin)

| # | Feature | Details |
|---|---------|---------|
| AT1 | Upload attachment | Upload file to Cloudinary via API, store returned URL + metadata (filename, type, size) linked to a lesson |
| AT2 | List attachments | Get all attachments for a lesson |
| AT3 | Delete attachment | Remove from Cloudinary + delete DB record |
| AT4 | Supported types | PDF, DOCX, PPTX, images (PNG, JPG), ZIP. Max 25 MB per file (Cloudinary free tier limit) |

### 8. Public Course Catalog

| # | Feature | Details |
|---|---------|---------|
| PC1 | List published courses | Paginated, filterable by category, searchable by title/description. No auth required |
| PC2 | Course detail (public) | Course info + module/lesson structure (titles only, no content). Shows lesson count, estimated total duration |

### 9. Enrollment

| # | Feature | Details |
|---|---------|---------|
| E1 | Enroll in course | Authenticated learner enrolls in a published course. Creates enrollment record with timestamp |
| E2 | Unenroll | Remove enrollment + all progress for that course |
| E3 | My enrollments | List courses the current user is enrolled in, with progress percentage |
| E4 | Enrollment check | Middleware/helper to verify if user is enrolled before serving lesson content |

### 10. Progress Tracking

| # | Feature | Details |
|---|---------|---------|
| PR1 | Mark lesson complete | Create progress record for user + lesson with completion timestamp |
| PR2 | Unmark lesson | Delete progress record (user changes mind) |
| PR3 | Course progress | Calculate percentage: `(completed lessons / total lessons) × 100` for a user in a course |
| PR4 | Auto-complete course | When progress reaches 100%, set `enrollments.completed_at` timestamp |

### 11. Certificates

| # | Feature | Details |
|---|---------|---------|
| CT1 | Generate certificate | On 100% course completion, user requests certificate generation. API generates a PDF with: user name, course title, completion date, unique cert number (`LEARNA-YYYY-XXXX`), site logo |
| CT2 | PDF generation | Use `gofpdf` or `go-pdf` library. Template includes border, logo, text layout. Upload generated PDF to Cloudinary |
| CT3 | My certificates | List all certificates for the current user |
| CT4 | Download certificate | Return Cloudinary PDF URL for download |
| CT5 | Public verification | `GET /certificates/verify/:certNumber` — returns cert details (name, course, date) without auth. Allows anyone to verify a certificate is genuine |

### 12. Admin Analytics

| # | Feature | Details |
|---|---------|---------|
| AN1 | Overview stats | Total users, total courses (by status), total enrollments, total completions |
| AN2 | Course stats | Per-course: enrollment count, completion count, completion rate %, average progress % |

### 13. File Upload (Cloudinary)

| # | Feature | Details |
|---|---------|---------|
| CL1 | Image upload | Generic endpoint for thumbnail/avatar uploads. Returns Cloudinary URL |
| CL2 | File upload | For attachments — accepts any allowed file type, uploads to Cloudinary, returns URL + metadata |
| CL3 | File deletion | Delete from Cloudinary by public_id when DB record is removed |
| CL4 | Folder organization | Cloudinary folders: `learna/thumbnails/`, `learna/avatars/`, `learna/attachments/`, `learna/certificates/` |

### 14. Infrastructure & Cross-Cutting

| # | Feature | Details |
|---|---------|---------|
| I1 | Database migrations | `golang-migrate` with versioned SQL files. Auto-run on startup or via `make migrate` |
| I2 | Environment config | All secrets and config via env vars (`.env` file for dev) |
| I3 | CORS middleware | Configurable allowed origins for frontend |
| I4 | Rate limiting | Basic rate limiter on auth endpoints (login, signup) to prevent brute force |
| I5 | Request logging | Structured JSON logs with request ID, method, path, status, duration |
| I6 | Input validation | Request body validation using `go-playground/validator` with proper error messages |
| I7 | Error handling | Consistent JSON error response format: `{ "error": { "code": "...", "message": "..." } }` |
| I8 | Health check | `GET /health` — returns DB connectivity status |
| I9 | Docker setup | Multi-stage Dockerfile (build + runtime). `docker-compose.yml` with API + Postgres |
| I10 | API versioning | All routes under `/api/v1/` prefix |

---

## UI Service (learna-ui)

### 1. Public Pages

| # | Feature | Details |
|---|---------|---------|
| UP1 | Landing page | Hero section with tagline, featured courses grid, "Browse All Courses" CTA, simple stats (total courses, total learners) |
| UP2 | Course catalog | Grid of published course cards (thumbnail, title, category, lesson count). Search bar, category filter. Paginated. SSR for SEO |
| UP3 | Course preview | Course description, module/lesson outline (accordion), total duration, lesson count, "Enroll" or "Continue Learning" CTA based on auth state. SSR for SEO |
| UP4 | Certificate verification | Enter or visit URL with cert number. Shows certificate details (name, course, date, cert number) or "not found" |
| UP5 | Public navbar | Logo, "Courses" link, Login/Signup buttons. Shows user avatar + dropdown when authenticated |
| UP6 | Footer | Simple footer with links: About, Contact (placeholder), social links |

### 2. Authentication Pages

| # | Feature | Details |
|---|---------|---------|
| UA1 | Signup page | Form: name, email, password, confirm password. Client-side validation. Success redirects to login |
| UA2 | Login page | Form: email, password. "Forgot password?" link. Success redirects to learner dashboard (or admin dashboard based on role) |
| UA3 | Forgot password | Form: email. Shows success message regardless (prevents email enumeration) |
| UA4 | Reset password | Form: new password, confirm. Token from URL params |
| UA5 | Auth state management | AuthProvider context wrapping the app. Handles token storage, refresh, and logout. Redirects unauthenticated users from protected routes |

### 3. Learner Dashboard

| # | Feature | Details |
|---|---------|---------|
| LD1 | My courses overview | Grid of enrolled courses with progress bar on each card. "Continue" button goes to last incomplete lesson |
| LD2 | Empty state | If no enrollments, show "Browse courses" CTA with illustration |

### 4. Learner Course View

| # | Feature | Details |
|---|---------|---------|
| LC1 | Course layout | Left sidebar with module/lesson navigation (collapsible modules). Completed lessons show checkmark. Main content area on right |
| LC2 | Lesson content | Render markdown content with proper typography. Support headings, code blocks, tables, images, lists, blockquotes |
| LC3 | Video embed | If lesson has video_url, render embedded YouTube/Vimeo player above or within content. Responsive sizing |
| LC4 | Attachments | List of downloadable files below lesson content. Show filename, type icon, file size. Click to download |
| LC5 | Mark complete | "Mark as Complete" button at bottom of lesson. Toggleable. Updates progress bar in sidebar |
| LC6 | Navigation | "Previous / Next Lesson" buttons at bottom. Auto-navigate across modules |
| LC7 | Progress bar | Top of sidebar or course header — shows overall course progress percentage |
| LC8 | Course completion | When 100%, show congratulations message with "Get Certificate" button |

### 5. Certificates (Learner)

| # | Feature | Details |
|---|---------|---------|
| LCT1 | My certificates | List/grid of earned certificates. Show course name, date earned, cert number |
| LCT2 | Download | Click to download PDF from Cloudinary |
| LCT3 | Share link | Copy public verification URL to clipboard |

### 6. Profile (Learner)

| # | Feature | Details |
|---|---------|---------|
| LP1 | View profile | Name, email, avatar, join date, total courses completed |
| LP2 | Edit profile | Update name, upload avatar |
| LP3 | Change password | Current password + new password form |

### 7. Admin Dashboard

| # | Feature | Details |
|---|---------|---------|
| AD1 | Analytics overview | Stats cards: total users, total courses, active enrollments, completion rate. Simple bar chart of enrollments over last 30 days |
| AD2 | Quick actions | Buttons: "New Course", "Manage Users" |

### 8. Admin Course Management

| # | Feature | Details |
|---|---------|---------|
| AC1 | Course list | Table with: title, status badge, category, enrollment count, created date. Actions: edit, delete, publish/unpublish. Filterable by status |
| AC2 | Create course | Form: title, description (textarea), category (dropdown/input), thumbnail upload (drag & drop zone with Cloudinary upload). Save as draft |
| AC3 | Edit course | Same form, pre-populated. Shows current thumbnail with replace option |
| AC4 | Delete course | Confirmation modal warning about cascade deletion |
| AC5 | Status toggle | Publish/unpublish button with confirmation |

### 9. Admin Module & Lesson Editor

| # | Feature | Details |
|---|---------|---------|
| AM1 | Module list | Accordion-style list of modules within a course. Drag handles for reordering. "Add Module" button at bottom |
| AM2 | Add/edit module | Inline form or modal: title, description |
| AM3 | Delete module | Confirmation modal |
| AM4 | Lesson list | Within each module accordion, list of lessons with drag handles. "Add Lesson" button |
| AM5 | Lesson editor page | Full page editor: title input, markdown editor with toolbar and live preview, video URL input, duration input, file attachment uploader (multi-file, shows upload progress) |
| AM6 | Drag & drop reorder | Modules and lessons reorderable via drag & drop. Saves new order on drop |

### 10. Admin User Management

| # | Feature | Details |
|---|---------|---------|
| AUM1 | User list | Table: name, email, role badge, status, join date, enrolled courses count. Search by name/email, filter by role |
| AUM2 | Create user | Form: name, email, password, role selector (admin/learner). Super admin sees admin option |
| AUM3 | Edit user | Change name, role, active status |
| AUM4 | Deactivate/activate | Toggle button with confirmation |
| AUM5 | Delete user | Confirmation modal with cascade warning |
| AUM6 | Role permissions display | Admin sees only learner management. Super admin sees admin + learner management |

### 11. Admin Course Analytics

| # | Feature | Details |
|---|---------|---------|
| ACA1 | Per-course stats | Enrollment count, completion count, completion rate %, list of enrolled users with their progress % |

### 12. UI Components & Patterns

| # | Feature | Details |
|---|---------|---------|
| UI1 | Design system | Tailwind CSS + shadcn/ui components (Button, Input, Card, Table, Dialog, Badge, Dropdown, Tabs, Accordion, Toast) |
| UI2 | Loading states | Skeleton loaders for course cards, tables, lesson content |
| UI3 | Empty states | Illustrated empty states for: no courses, no enrollments, no users, no certificates |
| UI4 | Toast notifications | Success/error toasts for all mutations (create, update, delete, enroll) |
| UI5 | Responsive design | Mobile-friendly layouts. Sidebar collapses to hamburger on mobile. Course grid adapts (3 cols → 2 → 1) |
| UI6 | Dark mode | Optional — Tailwind dark mode toggle (stored in localStorage) |
| UI7 | 404 page | Custom not-found page |
| UI8 | Error boundary | Graceful error handling with retry options |

### 13. UI Infrastructure

| # | Feature | Details |
|---|---------|---------|
| UIF1 | API client | Centralized fetch/axios wrapper with base URL config, JWT interceptor (attach token), 401 interceptor (trigger refresh or redirect to login) |
| UIF2 | Environment config | `NEXT_PUBLIC_API_URL` env var for API base URL |
| UIF3 | SEO | Meta tags, Open Graph tags for public course pages. `generateMetadata` in App Router |
| UIF4 | Docker setup | Standalone Next.js Dockerfile with output tracing |

---

## Phase 2 Features (Preview)

### Quiz & Assessment (API)

| # | Feature | Details |
|---|---------|---------|
| Q1 | Create quiz | Admin creates a quiz attached to a module or course. Multiple choice, true/false question types |
| Q2 | Question bank | Questions with options, correct answer, optional explanation |
| Q3 | Take quiz | Learner submits answers, API scores and stores result |
| Q4 | Quiz results | Score, pass/fail, which questions were wrong |
| Q5 | Retake policy | Configurable: unlimited retakes or limited attempts |
| Q6 | Score-gated certificate | Optional: require minimum quiz score for certificate |

### Quiz & Assessment (UI)

| # | Feature | Details |
|---|---------|---------|
| QU1 | Quiz builder | Admin drag-and-drop question builder with answer options |
| QU2 | Quiz taking | Clean quiz UI: one question at a time or all at once, progress indicator |
| QU3 | Results page | Score display, correct/incorrect breakdown, explanations |
| QU4 | Quiz in sidebar | Show quiz status (not taken / passed / failed) in course navigation |

---

## Phase 3 Features (Preview)

| # | Feature | Details |
|---|---------|---------|
| PH3-1 | Email notifications | Welcome email, course completion email, password reset email (SMTP integration) |
| PH3-2 | Lesson comments | Learners can ask questions on lessons, admins can reply |
| PH3-3 | Bookmarks | Learners bookmark lessons for quick access |
| PH3-4 | Course ratings | Star rating + text review after completion |
| PH3-5 | Full-text search | Search across course content, lesson titles, descriptions |
| PH3-6 | Category management | Admin-managed category table with icons/colors instead of free text |
| PH3-7 | Course prerequisites | Set course A as prerequisite for course B |
| PH3-8 | Bulk user import | CSV upload to create multiple learner accounts |
