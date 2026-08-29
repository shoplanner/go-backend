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

-- FindDuplicateLogins returns every row whose login is shared with another row. The GORM-era
-- schema had no uniqueness on users.login at all (see the comment on the index in schema.sql),
-- so deployed databases can hold duplicates; this is what the boot-time diagnostic and
-- cmd/dedup-logins report.
-- name: FindDuplicateLogins :many
SELECT
    *
FROM
    users
WHERE
    login IN (
        SELECT
            login
        FROM
            users
        GROUP BY
            login
        HAVING
            count(*) > 1
    )
ORDER BY
    login,
    id;

-- SetLogin renames a single account. Only cmd/dedup-logins uses it, to break up the duplicate
-- groups above; the API never renames anyone.
-- name: SetLogin :exec
UPDATE users
SET
    login = ?
WHERE
    id = ?;

-- DeleteUser removes an account. Only cmd/dedup-logins uses it, and only for accounts that
-- nothing references.
-- name: DeleteUser :exec
DELETE FROM users
WHERE
    id = ?;

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
