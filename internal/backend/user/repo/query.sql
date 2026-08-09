-- name: GetByLogin :one
SELECT
    *
FROM
    users
WHERE
    login = ?
LIMIT
    1;

-- name: CreateUser :execresult
INSERT INTO
    users(id, login, hash, role)
VALUES
    (?, ?, ?, ?);

-- name: GetAll :many
SELECT
    *
FROM
    users;

-- name: GetByID :one
SELECT
    *
FROM
    users
WHERE
    id = ?
LIMIT
    1;

-- GetLoginsByIDList backs the list repo, whose members carry the user's login. It replaces
-- GORM's Preload("Members.User").
-- name: GetLoginsByIDList :many
SELECT
    id,
    login
FROM
    users
WHERE
    id IN (sqlc.slice('ids'));
