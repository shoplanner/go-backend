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
