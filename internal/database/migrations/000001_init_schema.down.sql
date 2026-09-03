-- Reverse of 000001_init_schema.up.sql. Dropped in dependency order; the
-- CASCADE-linked children would go with their parents anyway, but listing them
-- explicitly keeps the intent readable.

DROP TABLE IF EXISTS certificates;
DROP TABLE IF EXISTS lesson_progress;
DROP TABLE IF EXISTS enrollments;
DROP TABLE IF EXISTS attachments;
DROP TABLE IF EXISTS lessons;
DROP TABLE IF EXISTS modules;
DROP TABLE IF EXISTS courses;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS refresh_tokens;
DROP TABLE IF EXISTS users;

DROP FUNCTION IF EXISTS set_updated_at();

DROP TYPE IF EXISTS course_status;
DROP TYPE IF EXISTS user_role;

-- pgcrypto is intentionally left installed: other schemas in the same database
-- may depend on it.
