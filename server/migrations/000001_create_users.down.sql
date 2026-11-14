DROP TRIGGER IF EXISTS users_set_updated_at ON users;
DROP INDEX IF EXISTS users_email_lower_idx;
DROP TABLE IF EXISTS users;
DROP FUNCTION IF EXISTS set_updated_at();

