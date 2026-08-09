-- name: InitUsers :exec
CREATE TABLE IF NOT EXISTS users (
    id varchar(36) PRIMARY KEY,
    role int NOT NULL,
    login varchar(36) NOT NULL UNIQUE,
    hash text NOT NULL
);

-- GORM's AutoMigrate used to reach the users table through the favorite and list member
-- associations and rewrite it: NOT NULL was dropped from role/login/hash and the inline UNIQUE
-- on login moved out into this index. Deployed files therefore have the loose shape while a
-- fresh one gets the strict DDL above. Creating the index here keeps both shapes enforcing the
-- same uniqueness. sqlc generates nothing for CREATE INDEX, so it is executed from sqlite.go.
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_login ON users(login);
