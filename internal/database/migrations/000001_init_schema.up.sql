-- Learna Phase 1 schema.
--
-- Conventions:
--   * UUID v4 primary keys, generated in Postgres via gen_random_uuid().
--   * TIMESTAMPTZ everywhere; the application never stores naive timestamps.
--   * Hard deletes with ON DELETE CASCADE down the course tree (Phase 1 has no
--     soft delete). Deleting a course removes its modules, lessons,
--     attachments, enrollments and progress.
--   * updated_at is maintained by the set_updated_at() trigger below, so no
--     query needs to remember to set it.

-- pgcrypto supplies gen_random_uuid() on Postgres < 13; on 13+ it is built in
-- but creating the extension stays harmless and keeps older servers working.
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- Case-insensitive uniqueness for emails is handled with a lower(email) index
-- rather than citext, to avoid a second extension dependency.

CREATE TYPE user_role AS ENUM ('super_admin', 'admin', 'learner');
CREATE TYPE course_status AS ENUM ('draft', 'published', 'archived');

-- Keeps updated_at honest without relying on application code.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------- users -----

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT        NOT NULL,
    password_hash TEXT        NOT NULL,
    name          TEXT        NOT NULL,
    avatar_url    TEXT,
    role          user_role   NOT NULL DEFAULT 'learner',
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));
CREATE INDEX users_role_idx ON users (role);
CREATE INDEX users_created_at_idx ON users (created_at DESC);

CREATE TRIGGER users_set_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ------------------------------------------------------- refresh_tokens -----
-- Refresh tokens are persisted so logout and password changes can revoke them.
-- Only the SHA-256 hash of the token is stored; the raw value lives solely in
-- the client.

CREATE TABLE refresh_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    user_agent TEXT,
    ip_address TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at);

-- ------------------------------------------------ password_reset_tokens -----

CREATE TABLE password_reset_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX password_reset_tokens_user_id_idx ON password_reset_tokens (user_id);

-- -------------------------------------------------------------- courses -----

CREATE TABLE courses (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title         TEXT          NOT NULL,
    slug          TEXT          NOT NULL UNIQUE,
    description   TEXT          NOT NULL DEFAULT '',
    thumbnail_url TEXT,
    category      TEXT          NOT NULL DEFAULT '',
    status        course_status NOT NULL DEFAULT 'draft',
    -- Courses outlive their author: deleting a user leaves the course with a
    -- NULL creator rather than cascading away published content.
    created_by    UUID          REFERENCES users (id) ON DELETE SET NULL,
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE INDEX courses_status_idx ON courses (status);
CREATE INDEX courses_category_idx ON courses (category);
CREATE INDEX courses_created_by_idx ON courses (created_by);
CREATE INDEX courses_created_at_idx ON courses (created_at DESC);

CREATE TRIGGER courses_set_updated_at
    BEFORE UPDATE ON courses
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- -------------------------------------------------------------- modules -----

CREATE TABLE modules (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    course_id   UUID        NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    title       TEXT        NOT NULL,
    description TEXT        NOT NULL DEFAULT '',
    sort_order  INTEGER     NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX modules_course_id_sort_order_idx ON modules (course_id, sort_order);

CREATE TRIGGER modules_set_updated_at
    BEFORE UPDATE ON modules
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- -------------------------------------------------------------- lessons -----

CREATE TABLE lessons (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    module_id    UUID        NOT NULL REFERENCES modules (id) ON DELETE CASCADE,
    title        TEXT        NOT NULL,
    content      TEXT        NOT NULL DEFAULT '',  -- raw markdown, rendered by the UI
    video_url    TEXT,
    duration_min INTEGER     NOT NULL DEFAULT 0 CHECK (duration_min >= 0),
    sort_order   INTEGER     NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX lessons_module_id_sort_order_idx ON lessons (module_id, sort_order);

CREATE TRIGGER lessons_set_updated_at
    BEFORE UPDATE ON lessons
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------- attachments -----

CREATE TABLE attachments (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    lesson_id  UUID        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    file_name  TEXT        NOT NULL,
    file_url   TEXT        NOT NULL,
    -- Cloudinary public_id, needed to delete the remote asset when the row goes.
    public_id  TEXT,
    file_type  TEXT        NOT NULL DEFAULT '',
    file_size  BIGINT      NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX attachments_lesson_id_idx ON attachments (lesson_id);

-- ---------------------------------------------------------- enrollments -----

CREATE TABLE enrollments (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id    UUID        NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    enrolled_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    UNIQUE (user_id, course_id)
);

CREATE INDEX enrollments_user_id_idx ON enrollments (user_id);
CREATE INDEX enrollments_course_id_idx ON enrollments (course_id);
CREATE INDEX enrollments_enrolled_at_idx ON enrollments (enrolled_at DESC);

-- ------------------------------------------------------ lesson_progress -----

CREATE TABLE lesson_progress (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    lesson_id    UUID        NOT NULL REFERENCES lessons (id) ON DELETE CASCADE,
    completed    BOOLEAN     NOT NULL DEFAULT TRUE,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, lesson_id)
);

CREATE INDEX lesson_progress_user_id_idx ON lesson_progress (user_id);
CREATE INDEX lesson_progress_lesson_id_idx ON lesson_progress (lesson_id);

-- --------------------------------------------------------- certificates -----

CREATE TABLE certificates (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    course_id   UUID        NOT NULL REFERENCES courses (id) ON DELETE CASCADE,
    cert_number TEXT        NOT NULL UNIQUE,  -- LEARNA-YYYY-XXXXXX
    pdf_url     TEXT,
    public_id   TEXT,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (user_id, course_id)
);

CREATE INDEX certificates_user_id_idx ON certificates (user_id);
CREATE INDEX certificates_course_id_idx ON certificates (course_id);
