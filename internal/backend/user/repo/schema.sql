-- name: InitUsers :exec
CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login varchar(36) NOT NULL UNIQUE,
    hash text NOT NULL
);

-- GORM's AutoMigrate used to reach the users table through the favorite and list member
-- associations and rewrite it from its own model, which was
--
--     type User struct {
--         ID    string `gorm:"primaryKey;size:36"`
--         Login string `gorm:"size:255"`   // no unique, no uniqueIndex
--         ...
--
-- so the rebuilt table lost the NOT NULLs *and* the inline UNIQUE on login, and nothing
-- replaced them: on every deployed disk users.login has no uniqueness whatsoever. The only
-- guard was the GetByLogin check in user/service, which is racy and postdates some of the rows.
-- This index is what puts the constraint back, and CREATE TABLE IF NOT EXISTS cannot, which is
-- why it is a separate statement. It fails on a database that already holds duplicates —
-- cmd/dedup-logins is the tool that resolves them. sqlc generates nothing for CREATE INDEX, so
-- the statement is executed from sqlite.go.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login ON users(login);
